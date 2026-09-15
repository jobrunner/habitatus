// Command habitatus serves EUNIS habitat classification over HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
// errConfig marks a configuration error this program raised itself, as
// opposed to one the flag package already reported. main prints only the
// former: the flag package writes its own message and usage to stderr, and a
// second copy would be noise — but an unreported error is worse than noise.
// Exiting 2 in silence is how an operator ends up staring at a container that
// stops with no reason given.
var errConfig = errors.New("invalid configuration")

type config struct {
	addr          string
	rulesPath     string
	backbonesPath string
	modeName      string
	corsOrigins   []string
	mcp           bool
	healthcheck   bool
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
	// 127.0.0.1, not ":8080": the built-in default is what a bare `habitatus
	// -rules …` gets, and this API is unauthenticated — including /metrics.
	// Listening on every interface by default contradicts what the README and
	// the compose files argue, and "the operator will bind it properly" is not
	// a property of a default. The container sets HABITATUS_ADDR=:8080
	// explicitly, because there the port is reachable only through an explicit
	// -p mapping.
	addr := fs.String("addr", envOrDefault("HABITATUS_ADDR", "127.0.0.1:8080"),
		"listen address; the default is loopback only")
	rulesPath := fs.String("rules", envOrDefault("HABITATUS_RULES", ""), "path to the ESy rule file")
	backbonesPath := fs.String("backbones", envOrDefault("HABITATUS_BACKBONES", ""),
		"path to the directory of nomenclature translation tables (optional)")
	mcp := fs.Bool("mcp", false, "serve MCP over stdio instead of HTTP")
	health := fs.Bool("healthcheck", false,
		"probe GET /health/ready on -addr and exit 0 or 1, instead of serving; "+
			"for a container HEALTHCHECK, where the distroless image has no curl")
	cors := fs.String("cors", envOrDefault("HABITATUS_CORS", ""),
		"comma-separated origins allowed to call the API from a browser "+
			"(scheme://host[:port]), or * for any; empty keeps CORS off")
	modeName := fs.String("mode", envOrDefault("HABITATUS_MODE", esy.Repaired.String()),
		"evaluation semantics: "+strings.Join(esy.ModeNames(), "|")+
			" (repaired evaluates the expressions ESy v1.2 forces to FALSE by "+
			"coercing their numeric value, as R did up to v1.1; faithful "+
			"reproduces v1.2 as shipped)")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	origins, err := httpapi.ParseOrigins(*cors)
	if err != nil {
		return config{}, fmt.Errorf("%w: %w", errConfig, err)
	}

	return config{
		addr:          *addr,
		rulesPath:     *rulesPath,
		backbonesPath: *backbonesPath,
		modeName:      *modeName,
		corsOrigins:   origins,
		mcp:           *mcp,
		healthcheck:   *health,
	}, nil
}

func main() {
	cfg, err := resolveConfig(os.Getenv, os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		if errors.Is(err, errConfig) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(2)
	}

	// Before anything else, and before the rule file is even looked at: the
	// probe talks to a server that is already running, and loading an 8 MB
	// rule pack to ask it whether it is ready would make every health check
	// as expensive as a start-up.
	if cfg.healthcheck {
		if err := healthcheck(cfg.addr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
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

	if err := run(log, cfg, mode); err != nil {
		log.Error("habitatus exited with an error", "err", err)
		os.Exit(1)
	}
}

// run takes the whole config rather than a growing parameter list: every
// option here is one an operator sets, and threading them individually made
// adding the CORS allowlist a change to the signature rather than to the
// struct that already describes the configuration.
func run(log *slog.Logger, cfg config, mode esy.Mode) error {
	addr, rulesPath, backbonesPath, mcp := cfg.addr, cfg.rulesPath, cfg.backbonesPath, cfg.mcp
	// Logged before anything else the service does: which semantics a run
	// used decides what its answers mean, and an operator reading the log
	// after the fact must not have to infer it.
	log.Info("evaluation mode", "mode", mode.String())

	//nolint:gosec // the rule file path is an operator-supplied flag; reading it is the program's purpose
	f, err := os.Open(rulesPath)
	if err != nil {
		return errors.New("cannot open rule file: " + err.Error())
	}
	defer func() { _ = f.Close() }()

	digest, err := rulepack.Digest(f)
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
		if err := mcpapi.NewServer(svc, version).Serve(os.Stdin, os.Stdout); err != nil {
			return errors.New("mcp server stopped: " + err.Error())
		}
		return nil
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.WithCORS(httpapi.NewServer(svc), cfg.corsOrigins),
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
