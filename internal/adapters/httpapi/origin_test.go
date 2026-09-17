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
		"https://a.example:http",   // nor a service name
		opaqueOrigin,               // the opaque origin is not allow-listable
		"https://*.",               // a wildcard needs a base host
		"https://*",                // "*" alone is not a host
		"https://sub*.example.com", // the "*" must be a whole label
		"https://*.*.example.com",  // and there is only one of it
		"https://a_b.example/x",    // still a path
		"https://[::1",             // unbalanced IPv6 literal
		"https://[::1]x",           // junk after the literal
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
		{pattern: "http://localhost:5173", origin: "http://localhost:5173", want: true},
		{pattern: "http://localhost:5173", origin: "http://localhost"},
		{pattern: "https://*.fieldworksdiary.org", origin: "https://app.fieldworksdiary.org", want: true},
		{pattern: "https://*.fieldworksdiary.org", origin: "https://a.b.fieldworksdiary.org", want: true},
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
