package rulepack

import (
	"os"
	"testing"
)

func TestSplitSectionsRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	for n, want := range map[int]int{1: 141032, 2: 23278, 3: 940, 4: 1} {
		if len(got[n]) != want {
			t.Errorf("section %d: got %d lines, want %d", n, len(got[n]), want)
		}
	}
}

func TestAggregationRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	agg, issues := ParseAggregation(secs[1])
	if len(agg) != 100704 {
		t.Errorf("got %d mappings, want 100704", len(agg))
	}
	if len(issues.DuplicateSources) != 7 {
		t.Errorf("got %d duplicate sources, want 7: %v", len(issues.DuplicateSources), issues.DuplicateSources)
	}
	if len(issues.Chains) != 2 {
		t.Errorf("got %d chains, want 2: %v", len(issues.Chains), issues.Chains)
	}
	if agg["Populus x canadensis + P. nigra"] != "Polypogon monspeliensis x viridis" {
		t.Errorf("first-entry-wins broken: %q", agg["Populus x canadensis + P. nigra"])
	}
}

func TestGroupsRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	groups := ParseGroups(secs[2])
	if len(groups) != 326 {
		t.Errorf("got %d groups, want 326", len(groups))
	}
	distinct := map[string]bool{}
	for name, ms := range groups {
		if len(ms) == 0 {
			t.Errorf("group %q is empty", name)
		}
		for _, m := range ms {
			distinct[m] = true
		}
	}
	// 8492, not 8477: ParseGroups keeps members' trailing whitespace on
	// purpose (0e23c97), matching upstream's trim.leading rather than a
	// full trim (ParsingExpertFile.R:37-38). 27 members of this file carry
	// a trailing blank and therefore never match a plot's taxon name,
	// exactly as upstream never matches them; trimming here would repair
	// the rule file instead of porting it. Both-side trimming would
	// collapse those 27 onto members already present without the blank,
	// giving 8477 distinct names instead — the number this assertion held
	// until 0e23c97 changed the trimming and it went stale, unnoticed for
	// four tasks because this test is ESY_FILE-gated and `go test ./...`
	// skips it. If this number moves again, that reason must move with it.
	if len(distinct) != 8492 {
		t.Errorf("got %d distinct members, want 8492", len(distinct))
	}
	if len(groups["Trees"]) != 323 {
		t.Errorf("got %d members in Trees, want 323", len(groups["Trees"]))
	}
	if len(groups["+01 MA211-Arctic-coastal-saltmarsh"]) != 19 {
		t.Errorf("got %d members in +01 MA211-Arctic-coastal-saltmarsh, want 19", len(groups["+01 MA211-Arctic-coastal-saltmarsh"]))
	}
}

func TestRulesRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 312 {
		t.Fatalf("got %d rules, want 312", len(rules))
	}
	var plain, bang, bangbang int
	for _, r := range rules {
		switch r.Variant {
		case "":
			plain++
		case "!":
			bang++
		case "!!":
			bangbang++
		}
		if r.Priority < 1 || r.Priority > 8 {
			t.Errorf("rule %s: priority %d out of range", r.Label(), r.Priority)
		}
		if r.Raw == "" {
			t.Errorf("rule %s has an empty formula", r.Label())
		}
	}
	if plain != 273 || bang != 34 || bangbang != 5 {
		t.Errorf("variants: plain=%d bang=%d bangbang=%d, want 273/34/5", plain, bang, bangbang)
	}
}

func TestParseEveryRealExpression(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	kinds := map[string]int{}
	for _, r := range rules {
		for _, raw := range extractExpressions(r.Raw) {
			if seen[raw] {
				continue
			}
			seen[raw] = true
			e, err := ParseExpr(raw)
			if err != nil {
				t.Errorf("rule %s: ParseExpr(%q): %v", r.Label(), raw, err)
				continue
			}
			for _, a := range append(append([]Atom{}, e.Left.Atoms...), e.Right.Atoms...) {
				kinds[a.Kind]++
			}
		}
	}
	if len(seen) != 931 {
		t.Errorf("got %d distinct expressions, want 931", len(seen))
	}
	for _, k := range []string{"#TC", "##Q", "$$N", "$$C", "#SC", "#T$", "#$$", "#01"} {
		if kinds[k] == 0 {
			t.Errorf("no atom of kind %s was parsed", k)
		}
	}
}

func TestParseEveryRealFormula(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		if _, err := ParseFormula(r.Raw); err != nil {
			t.Errorf("rule %s: %v\n  %s", r.Label(), err, r.Raw)
		}
	}
}

func TestLoadRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	pack, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(pack.Rules) != 312 {
		t.Errorf("rules = %d, want 312", len(pack.Rules))
	}
	for _, r := range pack.Rules {
		if r.Formula == nil {
			t.Errorf("rule %s has no parsed formula", r.Label())
		}
	}
	if len(pack.Issues.UnknownGroups) != 0 {
		t.Errorf("rules reference undefined groups: %v", pack.Issues.UnknownGroups)
	}
	if len(pack.KnownTaxa) == 0 {
		t.Errorf("KnownTaxa is empty")
	}
	t.Logf("issues: %d duplicate sources, %d chains",
		len(pack.Issues.DuplicateSources), len(pack.Issues.Chains))
}
