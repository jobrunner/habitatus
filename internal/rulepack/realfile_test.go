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
