package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/adapters/httpapi"
)

func envMap(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

func TestResolveConfigPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want config
	}{
		{
			name: "built-in defaults, nothing set",
			env:  nil,
			args: nil,
			want: config{addr: "127.0.0.1:8080", rulesPath: "", backbonesPath: "", modeName: "repaired"},
		},
		{
			name: "environment overrides built-in defaults",
			env: map[string]string{
				"HABITATUS_ADDR":      ":9090",
				"HABITATUS_RULES":     "/env/rules.txt",
				"HABITATUS_BACKBONES": "/env/backbones",
				"HABITATUS_MODE":      "faithful",
			},
			args: nil,
			want: config{
				addr:          ":9090",
				rulesPath:     "/env/rules.txt",
				backbonesPath: "/env/backbones",
				modeName:      "faithful",
			},
		},
		{
			name: "flag overrides environment",
			env: map[string]string{
				"HABITATUS_ADDR":  ":9090",
				"HABITATUS_RULES": "/env/rules.txt",
				"HABITATUS_MODE":  "faithful",
			},
			args: []string{"-mode", "repaired", "-rules", "/flag/rules.txt"},
			want: config{
				addr:      ":9090", // untouched by a flag, environment still wins here
				rulesPath: "/flag/rules.txt",
				modeName:  "repaired",
			},
		},
		{
			name: "mcp flag has no environment equivalent, defaults false",
			env:  nil,
			args: []string{"-mcp"},
			want: config{addr: "127.0.0.1:8080", modeName: "repaired", mcp: true},
		},
		{
			name: "HABITATUS_MCP selects MCP mode",
			env:  map[string]string{"HABITATUS_MCP": "1"},
			args: nil,
			want: config{addr: "127.0.0.1:8080", modeName: "repaired", mcp: true},
		},
		{
			name: "CORS is off unless asked for",
			env:  nil,
			args: nil,
			want: config{addr: "127.0.0.1:8080", modeName: "repaired"},
		},
		{
			name: "CORS origins come from the environment",
			env:  map[string]string{"HABITATUS_CORS": "https://a.example, https://b.example"},
			args: nil,
			want: config{addr: "127.0.0.1:8080", modeName: "repaired",
				corsOrigins: mustOrigins(t, "https://a.example, https://b.example")},
		},
		{
			name: "a wildcard covers the subdomains of a host",
			env:  map[string]string{"HABITATUS_CORS": "https://*.fieldworksdiary.org"},
			args: nil,
			want: config{addr: "127.0.0.1:8080", modeName: "repaired",
				corsOrigins: mustOrigins(t, "https://*.fieldworksdiary.org")},
		},
		{
			name: "a -cors flag beats the environment",
			env:  map[string]string{"HABITATUS_CORS": "https://a.example"},
			args: []string{"-cors", "*"},
			want: config{addr: "127.0.0.1:8080", modeName: "repaired", corsOrigins: mustOrigins(t, "*")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveConfig(envMap(tc.env), tc.args)
			if err != nil {
				t.Fatalf("resolveConfig: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolveConfig(%v, %v) = %+v, want %+v", tc.env, tc.args, got, tc.want)
			}
		})
	}
}

// A malformed allowlist must stop the start rather than silently leave CORS
// half-configured: an operator who wrote the origin wrong would otherwise get
// a service that looks up and rejects every browser call.
func TestResolveConfigRejectsBadCORS(t *testing.T) {
	for _, in := range []string{"a.example", "*,https://a.example", "https://",
		"https://sub*.example", "https://*.", "null"} {
		if _, err := resolveConfig(envMap(nil), []string{"-cors", in}); err == nil {
			t.Errorf("resolveConfig(-cors %q) = nil error, want a rejection", in)
		}
	}
}

// The rejection must be recognisable as ours, so main can report it. An
// operator who gets exit 2 with no message has nothing to act on.
func TestBadCORSIsReportableConfigError(t *testing.T) {
	_, err := resolveConfig(envMap(nil), []string{"-cors", "a.example"})
	if err == nil {
		t.Fatal("resolveConfig accepted a bad origin")
	}
	if !errors.Is(err, errConfig) {
		t.Errorf("error %v does not wrap errConfig, so main would exit silently", err)
	}
	if !strings.Contains(err.Error(), "a.example") {
		t.Errorf("error %v does not name the offending origin", err)
	}
}

// mustOrigins builds the parsed allowlist a table case expects. A bad literal
// here is a bug in the test, not a case under test.
func mustOrigins(t *testing.T, s string) []httpapi.OriginPattern {
	t.Helper()
	got, err := httpapi.ParseOrigins(s)
	if err != nil {
		t.Fatalf("ParseOrigins(%q): %v", s, err)
	}
	return got
}
