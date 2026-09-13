package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// preflightMaxAge is how long a browser may cache a preflight result. Ten
// minutes keeps the extra round trip off every request without pinning a
// stale policy for long after an operator changes the allowlist.
const preflightMaxAge = "600"

// ParseOrigins reads the comma-separated allowlist from -cors. An empty string
// means CORS stays off, which is the default: habitatus binds to loopback and
// expects a reverse proxy, and a permissive default would let any page a user
// visits reach an internal deployment.
//
// "*" allows any origin and may not be combined with named ones — a list that
// says both is a mistake about what it permits, so it is rejected rather than
// silently resolved one way.
func ParseOrigins(s string) ([]string, error) {
	var out []string
	for _, raw := range strings.Split(s, ",") {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		if o != "*" {
			u, err := url.Parse(o)
			if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
				return nil, fmt.Errorf("origin %q is not scheme://host[:port]", o)
			}
		}
		out = append(out, o)
	}
	for _, o := range out {
		if o == "*" && len(out) > 1 {
			return nil, fmt.Errorf("origin \"*\" cannot be combined with named origins")
		}
	}
	return out, nil
}

// WithCORS answers browser preflights and adds the access-control headers for
// an allowed origin. With an empty allowlist it returns the handler unchanged,
// so the service behaves exactly as it did before CORS existed.
//
// Why this is needed at all: POST /api/v1/classify takes application/json,
// which is not a CORS-simple content type, so every browser call is preceded
// by an OPTIONS preflight. Without this the mux answers that preflight with
// 405 and the browser never sends the request — the API is not degraded from a
// browser, it is unreachable.
//
// No Access-Control-Allow-Credentials: the service has no cookies, no sessions
// and no authentication, so there is no ambient authority for a browser to
// carry, and claiming otherwise would be the one CORS header that can actually
// grant something.
func WithCORS(next http.Handler, origins []string) http.Handler {
	if len(origins) == 0 {
		return next
	}
	allowAny := len(origins) == 1 && origins[0] == "*"
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always, even when the origin is not allowed and even when there is
		// no Origin at all: the response body differs by origin, so a cache or
		// reverse proxy in front must not serve one origin's response to
		// another.
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		ok := origin != "" && (allowAny || allowed[origin])
		if ok {
			if allowAny {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				// Echo the caller's origin rather than the list: the header
				// takes exactly one value, and the browser compares it.
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		// A preflight carries Access-Control-Request-Method. A bare OPTIONS
		// without it is not a preflight and falls through to the mux, which
		// answers 405 with Allow — the correct answer to a method probe.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			if ok {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", preflightMaxAge)
			}
			// 204 either way. A disallowed origin gets no access-control
			// headers, which is what makes the browser refuse; answering 403
			// would say the same thing less precisely and break the bare
			// OPTIONS probe above.
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
