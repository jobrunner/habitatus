package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// probeClient talks to this container and nothing else. http.DefaultClient
// honours HTTP_PROXY/HTTPS_PROXY, and for a non-loopback -addr — say
// 172.17.0.2:8080 — a proxy set in the environment would receive the probe
// instead of the server one process away. Worse than useless: the proxy's own
// 200 would report the container healthy while the server is down.
var probeClient = &http.Client{
	Transport: &http.Transport{Proxy: nil},
}

// healthcheckTimeout bounds the probe. A readiness check that hangs is a
// readiness check that fails, and an orchestrator waiting on it is worse off
// than one told "not ready" quickly.
const healthcheckTimeout = 3 * time.Second

// probeAddr turns a listen address into one a client can dial. ":8080" and
// "0.0.0.0:8080" mean "every interface" to a listener and nothing to a dialler,
// so the probe talks to loopback — which is where it runs anyway, inside the
// same container.
func probeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// healthcheck probes GET /health/ready and returns an error unless it answers
// 200. It exists because the image is distroless: it has no shell and no curl,
// and Docker can only run a health probe from inside the container. The binary
// is the only executable in there, so the binary has to be able to ask.
//
// It deliberately does NOT load the rule file. This is a liveness-and-
// readiness probe for an already-running server, not a second start-up.
func healthcheck(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	url := "http://" + probeAddr(addr) + "/health/ready"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: %s: status %d, want 200", url, resp.StatusCode)
	}
	return nil
}
