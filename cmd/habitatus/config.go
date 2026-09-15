// Configuration resolution lives beside main rather than inside it: it is a
// distinct concern with its own tests, and the composition root is long
// enough without it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/jobrunner/habitatus/internal/adapters/httpapi"
	"github.com/jobrunner/habitatus/internal/esy"
)

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
