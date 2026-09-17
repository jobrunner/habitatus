package httpapi

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// opaqueOrigin is what a browser sends for a document with no origin of its
// own: a sandboxed iframe, a file:// page, some cross-site redirects.
const opaqueOrigin = "null"

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

// parseOrigin parses "scheme://host[:port]" and nothing else. allowWildcard
// permits a leading "*." label, which only an allowlist entry may carry — an
// Origin header that contained one would be a host named "*", not a pattern.
//
// Scheme, host and port come back canonicalised — lowercased, an IPv6 literal
// in its shortest form, a port without leading zeroes — because the comparison
// is a string comparison against what a browser sends. An entry that differs
// from the browser's spelling only in case, in ":0443" or in
// "[0:0:0:0:0:0:0:1]" would parse, start the service, and match nothing: the
// same trap as the one above.
func parseOrigin(s string, allowWildcard bool) (origin, error) {
	// Browsers send "null" for opaque origins: sandboxed iframes, file://
	// documents, some cross-site redirects. Allow-listing it would open the API
	// to any sandboxed document anywhere, and it cannot be narrowed — so it is
	// refused with its own reason; the generic "needs a scheme" advice would
	// suggest "https://null".
	if s == opaqueOrigin {
		return origin{}, fmt.Errorf(
			"the opaque origin \"null\" cannot be allow-listed — it would admit any " +
				"sandboxed document; list the real origin instead")
	}

	scheme, rest, ok := strings.Cut(s, "://")
	if !ok || scheme == "" {
		return origin{}, fmt.Errorf("origin %q needs a scheme — write it as https://%s",
			s, strings.TrimPrefix(s, "://"))
	}
	if !schemePattern.MatchString(scheme) {
		return origin{}, fmt.Errorf(
			"origin %q has an invalid scheme %q — a scheme starts with a letter and "+
				"continues with letters, digits, \"+\", \"-\" or \".\"", s, scheme)
	}

	host, port, hasPort, err := splitHostPort(rest)
	if err != nil {
		return origin{}, fmt.Errorf("origin %q: %w", s, err)
	}
	if host == "" {
		return origin{}, fmt.Errorf("origin %q needs a host", s)
	}
	canonicalHost, hostOK := canonicalHost(host, allowWildcard)
	if !hostOK {
		return origin{}, fmt.Errorf(
			"origin %q: %q is not a host — an origin is scheme://host[:port] with no "+
				"path, query, fragment or userinfo", s, host)
	}
	// A port that is not decimal digits in 1-65535 is one no browser can ever
	// send, so the entry would be a rule that never matches. strconv.Atoi alone
	// is not that check: it accepts a sign, and "+443" is not a port.
	if hasPort {
		canonical, portOK := canonicalPort(port)
		if !portOK {
			return origin{}, fmt.Errorf("origin %q has an invalid port %q (expected 1-65535)", s, port)
		}
		port = canonical
	}

	scheme = strings.ToLower(scheme)
	return origin{
		scheme: scheme,
		host:   canonicalHost,
		port:   withoutDefaultPort(scheme, port),
	}, nil
}

// canonicalPort returns a decimal port in 1-65535 as a browser writes it.
// Leading zeroes are a spelling, not a different port: ":0443" and ":443" are
// the same, and comparing them as text would leave the first matching nothing.
func canonicalPort(s string) (string, bool) {
	if strings.TrimLeft(s, "0123456789") != "" {
		return "", false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return "", false
	}
	return strconv.Itoa(n), true
}

// defaultPorts are the ports a browser leaves out of the Origin header because
// the scheme implies them.
var defaultPorts = map[string]string{"http": "80", "https": "443"}

// withoutDefaultPort drops a port the scheme already implies, so that
// "https://app.example:443" and "https://app.example" are the one origin a
// browser calls both of them. Spelling the default out is not a mistake worth
// refusing — but keeping it would make the entry match nothing, which is the
// failure this whole file exists to prevent.
func withoutDefaultPort(scheme, port string) string {
	if port != "" && port == defaultPorts[scheme] {
		return ""
	}
	return port
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

// hostPattern is a host as an Origin header can carry it: dot-separated
// labels of letters, digits, "-" and "_". The optional final dot of an
// absolute DNS name is handled outside the regexp.
//
// Matching the whole host against it is what keeps a path, query, fragment or
// userinfo out — each of them shows up here as a character no host may
// contain, which is the check url.Parse does not offer. Underscores are
// allowed: they are not legal in a hostname, but browsers do send them, so
// refusing one would reject an origin that actually arrives.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// schemePattern is the URI scheme grammar of RFC 3986 §3.1. Without it
// "1https://app.example" and "https/evil://app.example" parse as schemes, and
// the entry is then one no browser can ever send.
var schemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)

// canonicalHost returns the host as a browser writes it in the Origin header,
// and reports whether it is a host at all: dot-separated labels, a bracketed
// IPv6 literal, or — for an allowlist entry — a wildcard over the leading
// label. Only an entry may carry the "*"; an Origin header containing one
// would be a host named "*".
//
// Canonicalising rather than merely accepting is what makes the comparison
// hold: a browser sends "https://[::1]", so an allowlist entry written
// "https://[0:0:0:0:0:0:0:1]" has to arrive at the same string.
func canonicalHost(s string, allowWildcard bool) (string, bool) {
	if after, bracketed := strings.CutPrefix(s, "["); bracketed {
		inner, closed := strings.CutSuffix(after, "]")
		ip := net.ParseIP(inner)
		// The colon requirement keeps a bracketed IPv4 address out, which no
		// browser sends. A zone ("%eth0") is refused by ParseIP, and with it
		// the delimiters a zone could otherwise smuggle past this check.
		if !closed || ip == nil || !strings.Contains(inner, ":") {
			return "", false
		}
		return "[" + ip.String() + "]", true
	}
	// Only as a whole leading label: "*.example.com" is a rule about the
	// subdomains of example.com, while "sub*.example.com" is a shape no
	// browser origin can be compared against label by label.
	prefix, base := "", s
	if rest, wildcard := strings.CutPrefix(s, "*."); wildcard && allowWildcard {
		prefix, base = "*.", rest
	}
	trailingDot := strings.HasSuffix(base, ".")
	base = strings.TrimSuffix(base, ".")
	if host, ok, numeric := canonicalNumericHost(base); numeric {
		if !ok {
			return "", false
		}
		return prefix + host, true
	}
	if !hostPattern.MatchString(base) {
		return "", false
	}
	if trailingDot {
		base += "."
	}
	return prefix + strings.ToLower(base), true
}

// canonicalNumericHost canonicalises the all-decimal IPv4 spellings browsers
// collapse in URL origins: "127.1", "127.000.000.001" and "2130706433" all
// come back as "127.0.0.1". A host containing only digits and dots is treated
// as one of these IPv4 forms rather than as a DNS name; otherwise an allowlist
// entry could look valid, start the service, and still never match the Origin
// header a browser serialises from the same URL.
func canonicalNumericHost(s string) (canonical string, ok, numeric bool) {
	if s == "" || strings.Trim(s, "0123456789.") != "" {
		return "", false, false
	}
	parts := strings.Split(s, ".")
	if len(parts) > 4 || parts[len(parts)-1] == "" {
		return "", false, true
	}
	var ip uint64
	for i, part := range parts {
		if part == "" {
			return "", false, true
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return "", false, true
		}
		if i < len(parts)-1 {
			if n > 255 {
				return "", false, true
			}
			ip |= n << (24 - 8*i)
			continue
		}
		lastBytes := 5 - len(parts)
		if n >= uint64(1)<<(8*lastBytes) {
			return "", false, true
		}
		ip |= n
	}
	return fmt.Sprintf("%d.%d.%d.%d", byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip)), true, true
}
