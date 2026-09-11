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
	"syscall"
	"time"

	"github.com/jobrunner/habitatus/internal/adapters/httpapi"
	"github.com/jobrunner/habitatus/internal/adapters/mcpapi"
	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/rulepack"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	rulesPath := flag.String("rules", "", "path to the ESy rule file")
	backbonesPath := flag.String("backbones", "", "path to the directory of nomenclature translation tables (optional)")
	mcp := flag.Bool("mcp", false, "serve MCP over stdio instead of HTTP")
	flag.Parse()

	// Logging always goes to stderr, never stdout. In -mcp mode stdout is
	// the JSON-RPC transport; even one stray log line on stdout before the
	// first protocol message would corrupt the stream for the client
	// reading it.
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if *rulesPath == "" {
		log.Error("missing -rules")
		os.Exit(2)
	}

	if err := run(log, *addr, *rulesPath, *backbonesPath, *mcp); err != nil {
		log.Error("habitatus exited with an error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, addr, rulesPath, backbonesPath string, mcp bool) error {
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
		backbones, err = rulepack.LoadBackbones(backbonesPath)
		if err != nil {
			return errors.New("cannot load backbone tables: " + err.Error())
		}
		log.Info("backbone tables loaded", "path", backbonesPath, "tables", len(backbones))
	}

	versions := map[string]string{
		"rulepack":        rulesPath,
		"rulepack_sha256": digest,
	}
	svc := classify.NewService(pack, backbones, versions)

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
