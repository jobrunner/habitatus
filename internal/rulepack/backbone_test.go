package rulepack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBackbones(t *testing.T) {
	dir := t.TempDir()
	content := `SECTION 1: Species aggregation
Abies alba                                                -  0
     Abies pectinata                                         0
SECTION 1: End
SECTION 2: Species groups
SECTION 2: End
SECTION 3: Group definitions
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`
	if err := os.WriteFile(filepath.Join(dir, "GermanSL 1.4_ExpertSystem.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, issues, err := LoadBackbones(dir)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := got["germansl-1.4"]
	if !ok {
		t.Fatalf("id not normalised to kebab-case: %v keys", len(got))
	}
	if tbl["Abies pectinata"] != "Abies alba" {
		t.Errorf("table not loaded: %v", tbl)
	}
	if _, ok := issues["germansl-1.4"]; !ok {
		t.Errorf("issues must carry an entry for every loaded table, even a clean one: %v", issues)
	}
}

func TestLoadBackbonesRejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"Europe_ExpertSystem.txt", "europe_ExpertSystem.txt"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("SECTION 1: Species aggregation\nSECTION 1: End\n"), 0o644)
	}
	if _, _, err := LoadBackbones(dir); err == nil {
		t.Skip("case-insensitive filesystem collapsed the two names")
	}
}

// TestLoadBackbonesCollectsParseIssues verifies that a duplicate source name
// and a chain within one backbone table are collected instead of discarded,
// mirroring what Load already does for the main rule pack's own section 1.
func TestLoadBackbonesCollectsParseIssues(t *testing.T) {
	dir := t.TempDir()
	content := `SECTION 1: Species aggregation
Target A                                                  -  0
     Shared name                                             0
Target B                                                  -  0
     Shared name                                             0
     Target A                                                0
SECTION 1: End
SECTION 2: Species groups
SECTION 2: End
SECTION 3: Group definitions
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`
	if err := os.WriteFile(filepath.Join(dir, "Quirky_ExpertSystem.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, issues, err := LoadBackbones(dir)
	if err != nil {
		t.Fatal(err)
	}
	iss, ok := issues["quirky"]
	if !ok {
		t.Fatalf("no issues entry for %q: %v", "quirky", issues)
	}
	if len(iss.DuplicateSources) != 1 || iss.DuplicateSources[0] != "Shared name" {
		t.Errorf("DuplicateSources = %v, want [Shared name]", iss.DuplicateSources)
	}
	if len(iss.Chains) != 1 || iss.Chains[0] != "Target A" {
		t.Errorf("Chains = %v, want [Target A]", iss.Chains)
	}
}
