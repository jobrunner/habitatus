package rulepack

import "testing"

func TestParseAggregationBasics(t *testing.T) {
	lines := []string{
		"Abies alba                                                -  0",
		"     Abies pectinata                                         0",
		"     Abies alba subsp. alba                                   0",
		"Empetrum nigrum aggr.                                     -  0",
		"     Empetrum nigrum                                         0",
	}
	got, issues := ParseAggregation(lines)
	if got["Abies pectinata"] != "Abies alba" {
		t.Errorf("Abies pectinata -> %q, want %q", got["Abies pectinata"], "Abies alba")
	}
	if got["Empetrum nigrum"] != "Empetrum nigrum aggr." {
		t.Errorf("Empetrum nigrum -> %q", got["Empetrum nigrum"])
	}
	if _, ok := got["Abies alba"]; ok {
		t.Error("a target must not map to itself")
	}
	if len(issues.DuplicateSources) != 0 {
		t.Errorf("unexpected duplicates: %v", issues.DuplicateSources)
	}
}

// The real file lists five source names under two different targets. The first
// occurrence wins, which is what upstream's match() does.
func TestParseAggregationFirstEntryWins(t *testing.T) {
	lines := []string{
		"Bupleurum commutatum                                      -  0",
		"     Bupleurum commutatum subsp. glaucocarpus                0",
		"Bupleurum pachnospermum                                   -  0",
		"     Bupleurum commutatum subsp. glaucocarpus                0",
	}
	got, issues := ParseAggregation(lines)
	if got["Bupleurum commutatum subsp. glaucocarpus"] != "Bupleurum commutatum" {
		t.Errorf("first entry must win, got %q", got["Bupleurum commutatum subsp. glaucocarpus"])
	}
	if len(issues.DuplicateSources) != 1 {
		t.Fatalf("want 1 duplicate reported, got %v", issues.DuplicateSources)
	}
}

// Chains are reported but never followed: resolution is single-pass.
func TestParseAggregationReportsChains(t *testing.T) {
	lines := []string{
		"Elymus species                                            -  0",
		"     Elytrigia species                                       0",
		"Elytrigia species                                         -  0",
		"     Elymus pycnanthus                                       0",
	}
	got, issues := ParseAggregation(lines)
	if got["Elymus pycnanthus"] != "Elytrigia species" {
		t.Errorf("want single-pass mapping to Elytrigia species, got %q", got["Elymus pycnanthus"])
	}
	if len(issues.Chains) != 1 || issues.Chains[0] != "Elytrigia species" {
		t.Errorf("want chain on Elytrigia species, got %v", issues.Chains)
	}
}

// A block at end of input must keep its last member. Upstream fixed exactly
// this bug in commit fb86835 for section 2.
func TestParseAggregationKeepsLastMember(t *testing.T) {
	lines := []string{
		"Salix euxina                                              -  0",
		"     Salix fragilis                                          0",
	}
	got, _ := ParseAggregation(lines)
	if got["Salix fragilis"] != "Salix euxina" {
		t.Errorf("last member of last block lost: %q", got["Salix fragilis"])
	}
}

// trimEntry must reproduce upstream's trim.trailing literally
// (spike/ESy-upstream/code/prep.R:53):
//
//	sub("\\s+$|\\s+\\d$|\\s+\\-\\s+\\d$", "", x)
//
// One branch per case, including the one that motivated the fix: where the
// name is long enough that the layer digit is glued onto its last character
// with no separating whitespace, R keeps the digit and so must we.
func TestTrimEntryMatchesRTrimTrailing(t *testing.T) {
	cases := []struct {
		in, want, why string
	}{
		{"Abies alba   ", "Abies alba", `\s+$`},
		{"Abies pectinata   0", "Abies pectinata", `\s+\d$`},
		{"Abies alba          -  0", "Abies alba", `\s+\-\s+\d$`},
		{"Abies alba", "Abies alba", "no trailer at all"},
		{
			"Alyssum repens subsp. trichostachyum var. trichostachyum0",
			"Alyssum repens subsp. trichostachyum var. trichostachyum0",
			"digit glued to the name: R keeps it",
		},
		{
			"Aconitum ochroleucum Willd. n Gard. Dict., ed. 8.: n.° 10",
			"Aconitum ochroleucum Willd. n Gard. Dict., ed. 8.: n.° 10",
			`two digits: \s+\d$ needs exactly one`,
		},
		{"Carex nigra 2 0", "Carex nigra 2", "only the last digit goes"},
		{"Poa annua 0   ", "Poa annua 0", `leftmost match is \s+$, the digit stays`},
		{"     Abies pectinata   0", "Abies pectinata", "leading whitespace (trim.leading)"},
	}
	for _, c := range cases {
		if got := trimEntry(c.in); got != c.want {
			t.Errorf("trimEntry(%q) = %q, want %q (%s)", c.in, got, c.want, c.why)
		}
	}
}
