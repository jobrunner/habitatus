package httpapi

import "testing"

func TestParseOriginPatternRejects(t *testing.T) {
	for _, in := range []string{
		"a.example",             // no scheme
		"://a.example",          // nor an empty one
		"https://",              // no host
		"https://:443",          // a port is not a host
		"https://a.example/app", // an origin has no path
		"https://a.example?x=1", // nor a query
		"https://a.example?",    // nor a bare "?"
		"https://a.example#f",   // nor a fragment
		"https://u:p@a.example", // nor userinfo
		"https://a.example:",    // a colon with no port
		"https://a.example:0",   // ports outside 1-65535 can never be sent
		"https://a.example:99999",
		"https://a.example:+443",      // a sign is not part of a port
		"https://a.example: 443",      // nor whitespace
		"1https://a.example",          // a scheme starts with a letter
		"https/evil://a.example",      // and carries no slash
		"https://[::1%eth0]",          // a zone is not part of an origin
		"https://[::1%/path]",         // least of all one smuggling a path
		"https://127.1",               // a browser sends this as 127.0.0.1
		"https://0177.0.0.1",          // and this too
		"https://*.127.0.0.1",         // a wildcard only applies to DNS labels
		"https://0x7f000001",          // a browser reads this as 127.0.0.1 too
		"https://example.0x1",         // and a hex last label the same way
		"https://2130706433",          // as it does a bare number
		"chrome-extension://abcd:443", // an extension origin carries no port
		"chrome-extension://*.abcd",   // and has no subdomains
		"https://a.example:http",      // nor a service name
		opaqueOrigin,                  // the opaque origin is not allow-listable
		"https://*.",                  // a wildcard needs a base host
		"https://*",                   // "*" alone is not a host
		"https://sub*.example.com",    // the "*" must be a whole label
		"https://*.*.example.com",     // and there is only one of it
		"https://a_b.example/x",       // still a path
		"https://[::1",                // unbalanced IPv6 literal
		"https://[::1]x",              // junk after the literal
	} {
		if got, err := ParseOriginPattern(in); err == nil {
			t.Errorf("ParseOriginPattern(%q) = %v, want an error", in, got)
		}
	}
}

func TestParseOriginPatternAccepts(t *testing.T) {
	for _, in := range []string{
		"https://a.example",
		"http://localhost:5173",
		"https://a.example:8443",
		"http://[::1]:8080",
		"https://192.0.2.10:8080",
		"https://*.fieldworksdiary.org",
		"https://a_b.example",
		"chrome-extension://abcdefghijklmnop",
		"https://app.example.", // an absolute DNS name is one a browser sends
		"https://*.fieldworksdiary.org.",
		"https://127.0.0.1:8080",
	} {
		p, err := ParseOriginPattern(in)
		if err != nil {
			t.Errorf("ParseOriginPattern(%q): %v", in, err)
			continue
		}
		if p.String() != in {
			t.Errorf("ParseOriginPattern(%q).String() = %q, want the entry back", in, p.String())
		}
	}
}

func TestOriginPatternMatches(t *testing.T) {
	for _, tc := range []struct {
		pattern, origin string
		want            bool
	}{
		{pattern: "https://a.example", origin: "https://a.example", want: true},
		{pattern: "https://a.example", origin: "http://a.example"},       // scheme differs
		{pattern: "https://a.example", origin: "https://a.example:8443"}, // port differs
		// A browser leaves the scheme's default port out of the Origin header,
		// so an entry that spells it out has to mean the same origin.
		{pattern: "https://a.example:443", origin: "https://a.example", want: true},
		{pattern: "https://a.example", origin: "https://a.example:443", want: true},
		{pattern: "http://a.example:80", origin: "http://a.example", want: true},
		{pattern: "http://a.example:443", origin: "http://a.example"}, // 443 is not http's default
		{pattern: "https://*.b.example:443", origin: "https://a.b.example", want: true},
		// A port is a number, and an IPv6 literal an address: both are compared
		// as a browser writes them, not as the operator spelled them.
		{pattern: "https://a.example:0443", origin: "https://a.example", want: true},
		{pattern: "https://a.example:08443", origin: "https://a.example:8443", want: true},
		{pattern: "https://[0:0:0:0:0:0:0:1]", origin: "https://[::1]", want: true},
		{pattern: "https://[::1]:8080", origin: "https://[0:0:0:0:0:0:0:1]:8080", want: true},
		{pattern: "https://[::1]", origin: "https://[::2]"},
		{pattern: "https://127.0.0.1:8080", origin: "https://127.0.0.1:8080", want: true},
		{pattern: "https://127.0.0.1", origin: "https://localhost"}, // an address is not the name for it
		// The trailing dot is part of the host a browser serialises, so the two
		// spellings stay two origins rather than being quietly merged.
		{pattern: "https://app.example.", origin: "https://app.example.", want: true},
		{pattern: "https://app.example.", origin: "https://app.example"},
		{pattern: "https://app.example", origin: "https://app.example."},
		{pattern: "http://localhost:5173", origin: "http://localhost:5173", want: true},
		{pattern: "http://localhost:5173", origin: "http://localhost"},
		{pattern: "https://*.fieldworksdiary.org", origin: "https://app.fieldworksdiary.org", want: true},
		{pattern: "https://*.fieldworksdiary.org", origin: "https://a.b.fieldworksdiary.org", want: true},
		{pattern: "https://*.fieldworksdiary.org.", origin: "https://app.fieldworksdiary.org.", want: true},
		{pattern: "https://*.fieldworksdiary.org.", origin: "https://app.fieldworksdiary.org"},
		{pattern: "https://*.fieldworksdiary.org", origin: "https://fieldworksdiary.org"},          // the bare domain is not a subdomain
		{pattern: "https://*.fieldworksdiary.org", origin: "https://notfieldworksdiary.org"},       // nor a lookalike
		{pattern: "https://*.fieldworksdiary.org", origin: "http://app.fieldworksdiary.org"},       // scheme still exact
		{pattern: "https://*.fieldworksdiary.org", origin: "https://app.fieldworksdiary.org:8443"}, // port too
		{pattern: "https://*.fieldworksdiary.org", origin: "https://evil.example"},
		{pattern: "https://a.example", origin: opaqueOrigin}, // never matches
		{pattern: "https://a.example", origin: "garbage"},
		// Scheme and host are case-insensitive; a browser lowercases them, and
		// an operator writing the allowlist by hand may not.
		{pattern: "HTTPS://A.Example", origin: "https://a.example", want: true},
		{pattern: "https://*.Fieldworksdiary.ORG", origin: "https://App.Fieldworksdiary.org", want: true},
	} {
		p, err := ParseOriginPattern(tc.pattern)
		if err != nil {
			t.Errorf("ParseOriginPattern(%q): %v", tc.pattern, err)
			continue
		}
		if got := p.Matches(tc.origin); got != tc.want {
			t.Errorf("ParseOriginPattern(%q).Matches(%q) = %v, want %v", tc.pattern, tc.origin, got, tc.want)
		}
	}
}

// "*" is the one entry that is not an origin at all: it allows any origin and
// is answered with a literal "*" rather than an echo.
func TestParseOriginPatternAny(t *testing.T) {
	p, err := ParseOriginPattern("*")
	if err != nil {
		t.Fatalf("ParseOriginPattern(\"*\"): %v", err)
	}
	if !p.IsAny() {
		t.Error("\"*\" did not parse as the any-origin entry")
	}
	if !p.Matches("https://anything.example") {
		t.Error("\"*\" must match any origin")
	}
	// "any origin" means any origin — not any string. A malformed Origin
	// header is answered with no access-control header at all, the same as
	// under a named allowlist.
	if p.Matches("garbage") || p.Matches("https://bad host") {
		t.Error("\"*\" matched a malformed Origin header")
	}
	// The exception is the opaque origin: a browser sends it as a literal, and
	// "*" is the one allowlist that legitimately covers it.
	if !p.Matches(opaqueOrigin) {
		t.Error("\"*\" must cover the opaque origin a sandboxed document sends")
	}
	if named, err := ParseOriginPattern("https://a.example"); err != nil || named.IsAny() {
		t.Errorf("a named origin reported IsAny (err=%v)", err)
	}
}

// A misspelled scheme is the one typo that survives every structural check:
// "htps://a.example" is a well-formed origin, so it parses and the service
// starts — and then matches nothing, because no browser sends that scheme.
func TestOriginPatternHasBrowserScheme(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{in: "https://a.example", want: true},
		{in: "http://localhost:5173", want: true},
		{in: "https://*.a.example", want: true},
		{in: "chrome-extension://abcdefghijklmnop", want: true},
		{in: "moz-extension://abcdefghijklmnop", want: true},
		{in: "*", want: true},
		{in: "htps://a.example"},
		{in: "ftp://a.example"},
	} {
		p, err := ParseOriginPattern(tc.in)
		if err != nil {
			t.Errorf("ParseOriginPattern(%q): %v", tc.in, err)
			continue
		}
		if got := p.HasBrowserScheme(); got != tc.want {
			t.Errorf("ParseOriginPattern(%q).HasBrowserScheme() = %v, want %v", tc.in, got, tc.want)
		}
	}
}
