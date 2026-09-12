package rulepack

import (
	"strings"
	"testing"
)

const miniFile = `SECTION 1: Species aggregation
Abies alba                                                -  0
     Abies pectinata                                         0
SECTION 1: End
SECTION 2: Species groups
### Trees
     Abies alba
SECTION 2: End
SECTION 3: Group definitions

4          T1H  Broadleaved deciduous plantation
<#TC Trees GR 25>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func TestSplitSections(t *testing.T) {
	got, err := SplitSections(strings.NewReader(miniFile))
	if err != nil {
		t.Fatalf("SplitSections: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 sections, got %d", len(got))
	}
	if len(got[1]) != 2 {
		t.Errorf("section 1: want 2 lines, got %d: %q", len(got[1]), got[1])
	}
	if len(got[4]) != 0 {
		t.Errorf("section 4 must be empty, got %q", got[4])
	}
	if got[3][0] != "" {
		t.Errorf("blank line inside section 3 must be preserved, got %q", got[3][0])
	}
}

func TestSplitSectionsRejectsUnterminated(t *testing.T) {
	_, err := SplitSections(strings.NewReader("SECTION 1: Species aggregation\nfoo\n"))
	if err == nil {
		t.Fatal("want error for unterminated section")
	}
}
