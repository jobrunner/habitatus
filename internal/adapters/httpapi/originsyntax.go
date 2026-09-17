package httpapi

// The syntax layer: what an origin string is, as opposed to what an allowlist
// entry means (origin.go). Everything here answers one question — would a
// browser ever send this, and in exactly which spelling?

import (
	"fmt"
	"net"
	"regexp"
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
// labels of letters, digits, "-" and "_".
//
// Matching the whole host against it is what keeps a path, query, fragment or
// userinfo out — each of them shows up here as a character no host may
// contain, which is the check url.Parse does not offer. Underscores are
// allowed: they are not legal in a hostname, but browsers do send them, so
// refusing one would reject an origin that actually arrives.
// The optional final dot is the absolute form of a DNS name: a page served
// from "app.example." sends exactly that in the Origin header, so refusing it
// would reject an origin that arrives.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*\.?$`)

// ipv4LastLabel is the WHATWG URL parser's "ends in a number" test: a host
// whose last label is all decimal digits, or a "0x"-prefixed hex number, is
// read as an IPv4 address rather than as a name. "127.1", "2130706433",
// "0x7f000001" and "example.0x1" are all addresses by this rule, and a browser
// serialises every one of them as a dotted quad — see canonicalHost.
var ipv4LastLabel = regexp.MustCompile(`^(\d+|0[xX][0-9a-fA-F]*)$`)

// endsInNumber reports whether a browser would read this host as an IPv4
// address. The trailing dot of an absolute name is not a label.
func endsInNumber(host string) bool {
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	return ipv4LastLabel.MatchString(labels[len(labels)-1])
}

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
	base := s
	wildcard := false
	if rest, hasWildcard := strings.CutPrefix(s, "*."); hasWildcard && allowWildcard {
		base = rest
		wildcard = true
	}
	if !hostPattern.MatchString(base) {
		return "", false
	}
	// An address is not a name, so a wildcard over it is not a rule about
	// subdomains at all, and only the one dotted-quad form a browser sends is
	// accepted. The shortened, octal and hex spellings ("127.1", "0177.0.0.1",
	// "0x7f000001") are refused rather than converted: they are all 127.0.0.1
	// to a browser, and converting them correctly would mean reproducing the
	// URL host parser — including the octal reading of a leading zero, where
	// getting it wrong means quietly allow-listing a different address.
	if endsInNumber(base) {
		ip := net.ParseIP(base)
		if wildcard || ip == nil || ip.To4() == nil {
			return "", false
		}
		return ip.String(), true
	}
	return strings.ToLower(s), true
}
