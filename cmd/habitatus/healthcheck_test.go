package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeAddr(t *testing.T) {
	for in, want := range map[string]string{
		":8080":           "127.0.0.1:8080",
		"0.0.0.0:8080":    "127.0.0.1:8080",
		"[::]:8080":       "127.0.0.1:8080",
		"127.0.0.1:8080":  "127.0.0.1:8080",
		"192.168.1.5:900": "192.168.1.5:900",
	} {
		if got := probeAddr(in); got != want {
			t.Errorf("probeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// The probe must succeed only on 200. Everything else — including a server
// that answers, but not with readiness — has to fail, or the health check
// reports healthy for a container that cannot serve.
func TestHealthcheckStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "ready", status: http.StatusOK},
		{name: "service unavailable", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Through a channel, not a shared variable: the handler runs in
			// the server's goroutine and CI runs the suite under -race.
			paths := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths <- r.URL.Path
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			err := healthcheck(strings.TrimPrefix(srv.URL, "http://"), false)
			if tc.wantErr && err == nil {
				t.Fatalf("status %d accepted, want an error", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("status %d: %v", tc.status, err)
			}
			select {
			case gotPath := <-paths:
				if gotPath != "/health/ready" {
					t.Errorf("probed %q, want /health/ready", gotPath)
				}
			default:
				t.Error("the probe never reached the server")
			}
		})
	}
}

// Nothing listening is the case that matters most: it is what a crashed or
// still-starting container looks like.
func TestHealthcheckNoListener(t *testing.T) {
	// Port 1 is privileged and unused; dialling it fails fast.
	if err := healthcheck("127.0.0.1:1", false); err == nil {
		t.Fatal("healthcheck succeeded with nothing listening")
	}
}

// In MCP mode there is no HTTP server to probe. Reporting the container
// unhealthy there would be wrong about a process that is working, so the probe
// says so and succeeds — and must not touch the network at all.
func TestHealthcheckMCPModeDoesNotProbe(t *testing.T) {
	if err := healthcheck("127.0.0.1:1", true); err != nil {
		t.Fatalf("MCP mode should not probe, got: %v", err)
	}
}
