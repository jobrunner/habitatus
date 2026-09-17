package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestParseOrigins(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "   ", want: nil},
		{in: "*", want: []string{"*"}},
		{in: "https://a.example", want: []string{"https://a.example"}},
		{in: "https://a.example, http://localhost:5173",
			want: []string{"https://a.example", "http://localhost:5173"}},
		{in: "https://*.fieldworksdiary.org", want: []string{"https://*.fieldworksdiary.org"}},
		{in: "*,https://a.example", wantErr: true},
		{in: "a.example", wantErr: true},             // no scheme
		{in: "https://", wantErr: true},              // no host
		{in: "https://:443", wantErr: true},          // a port is not a host
		{in: "https://a.example/app", wantErr: true}, // an origin has no path
		{in: "https://a.example?x=1", wantErr: true}, // nor a query
		{in: "https://a.example?", wantErr: true},    // nor a bare "?"
		{in: "https://a.example#f", wantErr: true},   // nor a fragment
		{in: "https://u:p@a.example", wantErr: true}, // nor userinfo
		{in: "https://sub*.example", wantErr: true},  // a partial-label wildcard
	} {
		got, err := ParseOrigins(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseOrigins(%q) = %v, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseOrigins(%q): %v", tc.in, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseOrigins(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i].String() != tc.want[i] {
				t.Errorf("ParseOrigins(%q)[%d] = %q, want %q", tc.in, i, got[i].String(), tc.want[i])
			}
		}
	}
}

// mustOrigins is the allowlist a test configures; a bad literal here is a bug
// in the test, not a case under test.
func mustOrigins(t *testing.T, s string) []OriginPattern {
	t.Helper()
	got, err := ParseOrigins(s)
	if err != nil {
		t.Fatalf("ParseOrigins(%q): %v", s, err)
	}
	return got
}

// An empty allowlist must leave the handler untouched, so that turning CORS
// off is indistinguishable from the service before CORS existed.
func TestWithCORSDisabledIsTransparent(t *testing.T) {
	h := okHandler()
	if got := WithCORS(h, nil); got == nil {
		t.Fatal("WithCORS returned nil")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://a.example")
	WithCORS(h, nil).ServeHTTP(rec, req)
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("allow-origin = %q, want none when CORS is off", v)
	}
	if v := rec.Header().Get("Vary"); v != "" {
		t.Errorf("Vary = %q, want none when CORS is off", v)
	}
}

// The preflight is the whole point: application/json is not a simple content
// type, so without this the browser never sends the classify request at all.
func TestWithCORSPreflight(t *testing.T) {
	h := WithCORS(okHandler(), mustOrigins(t, "https://a.example"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/classify", nil)
	req.Header.Set("Origin", "https://a.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://a.example" {
		t.Errorf("allow-origin = %q, want the caller's origin echoed", got)
	}
	for _, h := range []string{"Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("%s missing from the preflight response", h)
		}
	}
	if rec.Body.Len() != 0 {
		t.Errorf("preflight body = %q, want empty", rec.Body.String())
	}
}

// A disallowed origin gets a 204 with no access-control headers. That is what
// makes the browser refuse; the server does not need to say more.
func TestWithCORSPreflightForeignOrigin(t *testing.T) {
	h := WithCORS(okHandler(), mustOrigins(t, "https://a.example"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/classify", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want none for a foreign origin", got)
	}
}

// A bare OPTIONS is a method probe, not a preflight: it must reach the mux so
// the route can answer 405 with Allow.
func TestWithCORSBareOptionsFallsThrough(t *testing.T) {
	reached := false
	h := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusMethodNotAllowed)
	}), mustOrigins(t, "*"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/v1/classify", nil))
	if !reached {
		t.Error("a bare OPTIONS was swallowed by the CORS layer")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want the handler's 405", rec.Code)
	}
}

func TestWithCORSSimpleRequest(t *testing.T) {
	for _, tc := range []struct {
		name, origin, want string
		allow              string
	}{
		{name: "any", allow: "*", origin: "https://anything.example", want: "*"},
		{name: "listed", allow: "https://a.example, https://b.example",
			origin: "https://b.example", want: "https://b.example"},
		{name: "not listed", allow: "https://a.example",
			origin: "https://evil.example", want: ""},
		{name: "subdomain wildcard", allow: "https://*.fieldworksdiary.org",
			origin: "https://app.fieldworksdiary.org", want: "https://app.fieldworksdiary.org"},
		{name: "wildcard does not cover the bare domain", allow: "https://*.fieldworksdiary.org",
			origin: "https://fieldworksdiary.org", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			req.Header.Set("Origin", tc.origin)
			WithCORS(okHandler(), mustOrigins(t, tc.allow)).ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
				t.Errorf("allow-origin = %q, want %q", got, tc.want)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want the handler's 200 — CORS must not block on the server", rec.Code)
			}
			// Vary must be set whatever the verdict, or a cache in front can
			// hand one origin's response to another.
			if rec.Header().Get("Vary") != "Origin" {
				t.Errorf("Vary = %q, want Origin", rec.Header().Get("Vary"))
			}
		})
	}
}

// The start-up log is what makes a misconfigured allowlist readable without a
// browser: it names what the service resolved, and warns about the one entry
// shape that parses but can never match.
func TestLogCORS(t *testing.T) {
	type wantLog struct {
		level slog.Level
		have  []string
	}

	for _, tc := range []struct {
		name, allow string
		want        []wantLog
		wantNone    bool
	}{
		{name: "off says nothing", allow: "", wantNone: true},
		{name: "names the allowlist", allow: "https://a.example, https://*.b.example", want: []wantLog{{
			level: slog.LevelInfo,
			have:  []string{"CORS enabled", "https://a.example", "https://*.b.example"},
		}}},
		{name: "warns about a scheme no browser sends", allow: "htps://a.example", want: []wantLog{
			{level: slog.LevelWarn, have: []string{"check for a typo", "htps://a.example"}},
			{level: slog.LevelInfo, have: []string{"CORS enabled", "htps://a.example"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			LogCORS(slog.New(slog.NewTextHandler(&buf, nil)), mustOrigins(t, tc.allow))

			if tc.wantNone && buf.Len() != 0 {
				t.Fatalf("logged %q with CORS off, want nothing", buf.String())
			}
			if tc.wantNone {
				return
			}
			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if got, want := len(lines), len(tc.want); got != want {
				t.Fatalf("logged %d line(s), want %d: %q", got, want, buf.String())
			}
			for i, want := range tc.want {
				if !strings.Contains(lines[i], "level="+want.level.String()) {
					t.Errorf("line %d = %q, want level %s", i, lines[i], want.level)
				}
				for _, have := range want.have {
					if !strings.Contains(lines[i], have) {
						t.Errorf("line %d = %q, want it to mention %q", i, lines[i], have)
					}
				}
			}
		})
	}
}
