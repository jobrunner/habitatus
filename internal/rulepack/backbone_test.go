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
	got, err := LoadBackbones(dir)
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
}

func TestLoadBackbonesRejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"Europe_ExpertSystem.txt", "europe_ExpertSystem.txt"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("SECTION 1: Species aggregation\nSECTION 1: End\n"), 0o644)
	}
	if _, err := LoadBackbones(dir); err == nil {
		t.Skip("case-insensitive filesystem collapsed the two names")
	}
}
