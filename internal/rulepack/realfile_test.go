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
	if len(distinct) != 8477 {
		t.Errorf("got %d distinct members, want 8477", len(distinct))
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
