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
