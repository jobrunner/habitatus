package httpapi

import (
	"fmt"
	"strings"
)

// opaqueOrigin is what a browser sends for a document with no origin of its
// own: a sandboxed iframe, a file:// page, some cross-site redirects.
const opaqueOrigin = "null"

// OriginPattern is one entry of the CORS allowlist: the literal "*", an exact
// origin, or a wildcard covering the subdomains of a host
// ("https://*.example.com").
//
// The wildcard applies to the leading host label only. Scheme and port still
// have to match exactly, because widening them would hand responses to servers
// the operator never listed — a plaintext "http://" sibling, or a different
// service on another port of the same host.
type OriginPattern struct {
	raw      string // the entry as configured, for String and for messages
	any      bool   // the entry was "*"
	origin   origin
	wildcard bool   // the host was written as "*.<suffix>"
	suffix   string // ".example.com" — the part after the "*", wildcard only
}

// ParseOriginPattern parses one allowlist entry.
//
// A wildcard entry needs a scheme like any other origin, and the "*" must be a
// whole leading label: "https://*.example.com" is valid, "https://sub*.example.com"
// is not. Anything that is not a bare origin is refused rather than repaired:
// an entry a browser can never send would sit in the allowlist looking
// configured while matching nothing, which is far harder to diagnose than a
// start-up refusal. That failure is not hypothetical — "https://*.example.com"
// was accepted by the earlier exact-match allowlist and silently matched no
// subdomain at all.
func ParseOriginPattern(s string) (OriginPattern, error) {
	if s == "*" {
		return OriginPattern{raw: s, any: true}, nil
	}

	o, err := parseOrigin(s, true)
	if err != nil {
		return OriginPattern{}, err
	}
	// An extension origin is a scheme and an opaque id: a browser serialises it
	// without a port, and it has no DNS labels for a wildcard to stand in for.
	// Either spelling would be an entry that matches nothing.
	if extensionSchemes[o.scheme] && (o.port != "" || strings.HasPrefix(o.host, "*.")) {
		return OriginPattern{}, fmt.Errorf(
			"origin %q: a %s origin carries neither a port nor subdomains", s, o.scheme)
	}
	if !strings.HasPrefix(o.host, "*.") {
		return OriginPattern{raw: s, origin: o}, nil
	}

	return OriginPattern{
		raw:      s,
		origin:   o,
		wildcard: true,
		suffix:   o.host[1:], // "*.example.com" -> ".example.com"
	}, nil
}

// browserSchemes are the schemes a browser can actually put in an Origin
// header: http and https for ordinary pages, plus the extension schemes.
// Anything else in an allowlist is far more often a typo than an intention.
var browserSchemes = map[string]bool{"http": true, "https": true}

// extensionSchemes are the browser-internal ones. They are origins a browser
// can send, but not ones with a host structure: no port, no subdomains.
var extensionSchemes = map[string]bool{
	"chrome-extension":     true,
	"moz-extension":        true,
	"safari-web-extension": true,
}

// HasBrowserScheme reports whether the entry's scheme is one a browser can
// send. "*" carries no scheme and is always true.
//
// A misspelled scheme is the one typo that survives every structural check:
// "htps://a.example" is a well-formed origin, so it parses, the service starts,
// and the entry then matches nothing — the same silent failure a wildcard used
// to cause. Callers warn rather than refuse, because the list above cannot be
// proven exhaustive for every browser.
func (p OriginPattern) HasBrowserScheme() bool {
	return p.any || browserSchemes[p.origin.scheme] || extensionSchemes[p.origin.scheme]
}

// String returns the entry as it was configured, so a log line or an error
// names what the operator wrote rather than a normalised rewrite of it.
func (p OriginPattern) String() string { return p.raw }

// IsAny reports whether the entry is the literal "*". Such a request is
// answered with "*" rather than an echo of the caller's origin.
func (p OriginPattern) IsAny() bool { return p.any }

// Matches reports whether a raw Origin header value is covered by the pattern.
// A malformed origin never matches — not even under "*", which is a rule about
// origins and not about arbitrary header values.
func (p OriginPattern) Matches(s string) bool {
	o, err := parseOrigin(s, false)
	if p.any {
		// The opaque origin cannot be parsed and cannot be allow-listed by
		// name, but "*" is the one allowlist that legitimately covers it: a
		// sandboxed document sends it as this literal.
		return err == nil || s == opaqueOrigin
	}
	if err != nil {
		return false
	}
	if o.scheme != p.origin.scheme || o.port != p.origin.port {
		return false
	}
	if !p.wildcard {
		return o.host == p.origin.host
	}
	// Requiring more than the suffix keeps the bare domain out; keeping the
	// leading dot keeps "notexample.com" out.
	return strings.HasSuffix(o.host, p.suffix) && len(o.host) > len(p.suffix)
}
