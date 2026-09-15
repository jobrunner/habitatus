package httpapi

import (
	"fmt"
	"strconv"
	"strings"
)

// origin is a web origin: the scheme/host/port triple a browser sends in the
// Origin header. All three parts identify it — two URLs that differ in scheme
// or port are different origins, even on the same host.
type origin struct {
	scheme string // "https", lowercased
	host   string // "example.com", or a bracketed IPv6 literal "[::1]", lowercased
	port   string // "" when the scheme's default port is used
}

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
	if !strings.Contains(o.host, "*") {
		return OriginPattern{raw: s, origin: o}, nil
	}

	// The base host must be present: "https://*." would leave the suffix ".",
	// which matches any host ending in a dot.
	base, ok := strings.CutPrefix(o.host, "*.")
	if !ok || base == "" || strings.Contains(base, "*") {
		return OriginPattern{}, fmt.Errorf(
			"origin %q: \"*\" must be the whole leading host label (e.g. https://*.example.com)", s)
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
var browserSchemes = map[string]bool{
	"http":                 true,
	"https":                true,
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
func (p OriginPattern) HasBrowserScheme() bool { return p.any || browserSchemes[p.origin.scheme] }

// String returns the entry as it was configured, so a log line or an error
// names what the operator wrote rather than a normalised rewrite of it.
func (p OriginPattern) String() string { return p.raw }

// IsAny reports whether the entry is the literal "*". Such a request is
// answered with "*" rather than an echo of the caller's origin.
func (p OriginPattern) IsAny() bool { return p.any }

// Matches reports whether a raw Origin header value is covered by the pattern.
// A malformed origin never matches.
func (p OriginPattern) Matches(s string) bool {
	if p.any {
		return true
	}
	o, err := parseOrigin(s, false)
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

// parseOrigin parses "scheme://host[:port]" and nothing else. allowWildcard
// permits a leading "*." label, which only an allowlist entry may carry — an
// Origin header that contained one would be a host named "*", not a pattern.
//
// Scheme and host are lowercased: a browser sends them lowercased, while an
// operator writing the allowlist by hand may not, and a case difference that
// silently disables an entry is the same trap as the one above.
func parseOrigin(s string, allowWildcard bool) (origin, error) {
	// Browsers send "null" for opaque origins: sandboxed iframes, file://
	// documents, some cross-site redirects. Allow-listing it would open the API
	// to any sandboxed document anywhere, and it cannot be narrowed — so it is
	// refused with its own reason; the generic "needs a scheme" advice would
	// suggest "https://null".
	if s == "null" {
		return origin{}, fmt.Errorf(
			"the opaque origin \"null\" cannot be allow-listed — it would admit any " +
				"sandboxed document; list the real origin instead")
	}

	scheme, rest, ok := strings.Cut(s, "://")
	if !ok || scheme == "" {
		return origin{}, fmt.Errorf("origin %q needs a scheme — write it as https://%s",
			s, strings.TrimPrefix(s, "://"))
	}

	host, port, hasPort, err := splitHostPort(rest)
	if err != nil {
		return origin{}, fmt.Errorf("origin %q: %w", s, err)
	}
	if host == "" {
		return origin{}, fmt.Errorf("origin %q needs a host", s)
	}
	if !isHost(host, allowWildcard) {
		return origin{}, fmt.Errorf(
			"origin %q: %q is not a host — an origin is scheme://host[:port] with no "+
				"path, query, fragment or userinfo", s, host)
	}
	// A port outside 1-65535 is one no browser can ever send, so the entry
	// would be a rule that never matches.
	if hasPort {
		if n, convErr := strconv.Atoi(port); convErr != nil || n < 1 || n > 65535 {
			return origin{}, fmt.Errorf("origin %q has an invalid port %q (expected 1-65535)", s, port)
		}
	}

	return origin{
		scheme: strings.ToLower(scheme),
		host:   strings.ToLower(host),
		port:   port,
	}, nil
}

// splitHostPort separates an optional ":port" from a host, leaving a bracketed
// IPv6 literal intact — its colons belong to the address, not to a port.
// hasPort distinguishes "no port given" from a port that is present but empty
// ("example.com:"), which is malformed rather than a default.
func splitHostPort(hostPort string) (host, port string, hasPort bool, err error) {
	if after, found := strings.CutPrefix(hostPort, "["); found {
		literal, rest, closed := strings.Cut(after, "]")
		if !closed {
			return "", "", false, fmt.Errorf("unbalanced IPv6 literal %q", hostPort)
		}
		// Only ":port" may follow the literal. Anything else is neither host
		// nor port; letting it through would leave an entry no browser origin
		// can match.
		if rest != "" && !strings.HasPrefix(rest, ":") {
			return "", "", false, fmt.Errorf(
				"unexpected %q after the IPv6 literal (expected \":port\" or nothing)", rest)
		}
		p, hasP := strings.CutPrefix(rest, ":")
		return "[" + literal + "]", p, hasP, nil
	}

	if h, p, found := strings.Cut(hostPort, ":"); found {
		return h, p, true, nil
	}
	return hostPort, "", false, nil
}

// isHost reports whether s is a host an Origin header can carry: a bracketed
// IPv6 literal, or dot-separated labels of letters, digits, "-" and "_".
//
// Checking the character set is what keeps a path, query, fragment or userinfo
// out — each of them shows up here as a character no host may contain.
func isHost(s string, allowWildcard bool) bool {
	if strings.HasPrefix(s, "[") {
		return isIPv6Literal(s)
	}
	for i, label := range strings.Split(s, ".") {
		// The wildcard is a whole label and only the leading one; a "*"
		// anywhere else is rejected by ParseOriginPattern.
		if label == "*" && allowWildcard && i == 0 {
			continue
		}
		if !isHostLabel(label) {
			return false
		}
	}
	return true
}

// isHostLabel reports whether one dot-separated label is well formed.
// Underscores are allowed: they are not legal in a hostname, but browsers do
// send them, so refusing one would reject an origin that actually arrives.
func isHostLabel(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// isIPv6Literal reports whether s is a bracketed IPv6 address, the one host
// form whose own syntax contains colons. The zone separator "%" is part of it.
func isIPv6Literal(s string) bool {
	inner, ok := strings.CutSuffix(strings.TrimPrefix(s, "["), "]")
	if !ok || inner == "" {
		return false
	}
	for _, r := range inner {
		if !isHexDigit(r) && r != ':' && r != '.' && r != '%' {
			return false
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}
