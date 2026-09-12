// Command habitatus serves EUNIS habitat classification over HTTP.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jobrunner/habitatus/internal/adapters/httpapi"
	"github.com/jobrunner/habitatus/internal/adapters/mcpapi"
	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
)

// version and commit identify the build. They stay at their zero values for
// `go build`/`go run`; the release Dockerfile stamps real values in via
// `-ldflags "-X main.version=... -X main.commit=..."` so a running binary
// can be traced back to the source it was built from.
var (
	version = "dev"
	commit  = "unknown"
)

// config holds everything main needs after flags and environment are
// resolved. Kept separate from flag.FlagSet so resolveConfig is callable
// from a test without starting a server or touching the real environment.
type config struct {
	addr          string
	rulesPath     string
	backbonesPath string
	modeName      string
	mcp           bool
}

// resolveConfig applies -addr/-rules/-backbones/-mode/-mcp with the
// precedence a container operator expects: an explicit flag beats the
// matching environment variable, which beats the built-in default. This is
// what lets `docker run habitatus -mode faithful` work at all — with the
// old plain CMD-array defaults, passing any argument replaced the whole
// CMD, including -rules and -addr, and the container exited on "missing
// -rules". Environment variables live outside CMD and survive that.
func resolveConfig(env func(string) string, args []string) (config, error) {
	envOrDefault := func(key, def string) string {
		if v := env(key); v != "" {
			return v
		}
		return def
	}

	fs := flag.NewFlagSet("habitatus", flag.ContinueOnError)
	addr := fs.String("addr", envOrDefault("HABITATUS_ADDR", ":8080"), "listen address")
	rulesPath := fs.String("rules", envOrDefault("HABITATUS_RULES", ""), "path to the ESy rule file")
	backbonesPath := fs.String("backbones", envOrDefault("HABITATUS_BACKBONES", ""),
		"path to the directory of nomenclature translation tables (optional)")
	mcp := fs.Bool("mcp", false, "serve MCP over stdio instead of HTTP")
	modeName := fs.String("mode", envOrDefault("HABITATUS_MODE", esy.Repaired.String()),
		"evaluation semantics: "+strings.Join(esy.ModeNames(), "|")+
			" (repaired evaluates the expressions ESy v1.2 forces to FALSE by "+
			"coercing their numeric value, as R did up to v1.1; faithful "+
			"reproduces v1.2 as shipped)")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	return config{
		addr:          *addr,
		rulesPath:     *rulesPath,
		backbonesPath: *backbonesPath,
		modeName:      *modeName,
		mcp:           *mcp,
	}, nil
}

func main() {
	cfg, err := resolveConfig(os.Getenv, os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}

	// Logging always goes to stderr, never stdout. In -mcp mode stdout is
	// the JSON-RPC transport; even one stray log line on stdout before the
	// first protocol message would corrupt the stream for the client
	// reading it.
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	log.Info("habitatus starting", "version", version, "commit", commit)

	if cfg.rulesPath == "" {
		log.Error("missing -rules (or HABITATUS_RULES)")
		os.Exit(2)
	}

	mode, err := esy.ParseMode(cfg.modeName)
	if err != nil {
		log.Error("invalid -mode (or HABITATUS_MODE)", "err", err)
		os.Exit(2)
	}

	// Logged so an operator reading the startup log after the fact can see
	// exactly which rule file and mode a run resolved to, regardless of
	// whether that came from a flag, an environment variable, or the
	// built-in default.
	log.Info("configuration resolved",
		"addr", cfg.addr, "rules", cfg.rulesPath, "backbones", cfg.backbonesPath, "mode", cfg.modeName)

	if err := run(log, cfg.addr, cfg.rulesPath, cfg.backbonesPath, cfg.mcp, mode); err != nil {
		log.Error("habitatus exited with an error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, addr, rulesPath, backbonesPath string, mcp bool, mode esy.Mode) error {
	// Logged before anything else the service does: which semantics a run
	// used decides what its answers mean, and an operator reading the log
	// after the fact must not have to infer it.
	log.Info("evaluation mode", "mode", mode.String())

	f, err := os.Open(rulesPath)
	if err != nil {
		return errors.New("cannot open rule file: " + err.Error())
	}
	defer f.Close()

	digest, err := fileDigest(f)
	if err != nil {
		return errors.New("cannot digest rule file: " + err.Error())
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errors.New("cannot rewind rule file: " + err.Error())
	}

	// A parse error aborts the start: a service running on a half-parsed
	// rule pack would answer with silent nonsense. Data defects (duplicate
	// sources, chains) do not abort — the official rule file has both, and
	// Load already draws the line between a broken file and a merely
	// imperfect one. We only log the counters so an operator can see them.
	pack, err := rulepack.Load(f)
	if err != nil {
		return errors.New("cannot load rule pack: " + err.Error())
	}
	log.Info("rule pack loaded",
		"path", rulesPath,
		"sha256", digest,
		"rules", len(pack.Rules),
		"groups", len(pack.Groups),
		"aggregations", len(pack.Aggregation),
		"duplicate_sources", len(pack.Issues.DuplicateSources),
		"chains", len(pack.Issues.Chains),
		"unknown_groups", len(pack.Issues.UnknownGroups),
	)

	var backbones map[string]map[string]string
	if backbonesPath != "" {
		var backboneIssues map[string]rulepack.Issues
		backbones, backboneIssues, err = rulepack.LoadBackbones(backbonesPath)
		if err != nil {
			return errors.New("cannot load backbone tables: " + err.Error())
		}
		// These translation tables are more defective than the main rule
		// file, not less: GermanSL 1.4 alone carries 26 duplicate source
		// names and 51 chains. Sum the counters across all tables so an
		// operator sees the same class of defect Load already logs for
		// the main pack, without a line per table.
		var duplicateSources, chains int
		for _, iss := range backboneIssues {
			duplicateSources += len(iss.DuplicateSources)
			chains += len(iss.Chains)
		}
		log.Info("backbone tables loaded",
			"path", backbonesPath,
			"tables", len(backbones),
			"duplicate_sources", duplicateSources,
			"chains", chains,
		)
	}

	// The file name, not the path: where the rule file sits on this server is
	// an operational detail about our filesystem, and it went out to every
	// caller of every HTTP and MCP response. The digest is what actually
	// identifies a rule pack, and it stays. The server log above keeps the
	// full path for the operator.
	versions := map[string]string{
		"rulepack":        filepath.Base(rulesPath),
		"rulepack_sha256": digest,
	}
	svc := classify.NewService(pack, backbones, versions, mode)

	if mcp {
		if err := mcpapi.NewServer(svc).Serve(os.Stdin, os.Stdout); err != nil {
			return errors.New("mcp server stopped: " + err.Error())
		}
		return nil
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewServer(svc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.New("graceful shutdown failed: " + err.Error())
		}
		log.Info("shut down cleanly")
		return nil
	}
}

func fileDigest(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
