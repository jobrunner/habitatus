package rulepack

import (
	"sort"
	"strings"
	"testing"
)

const loadFixture = `SECTION 1: Species aggregation
Fagus sylvatica            -  0
     Fagus sylvatica agg.      0
SECTION 1: End
SECTION 2: Species groups
### Trees
     Fagus sylvatica
     Quercus robur
##D +01 SomeGroup
     Corylus avellana
SECTION 2: End
SECTION 3: Rules
4          T1H  Test habitat
<#TC Trees GR 5>
2          T2H  Missing group habitat
<##Q +02 MissingGroup>
2          T3H  Bare taxon habitat
<Fagus sylvatica GR 5>
SECTION 3: End
`

func TestLoadAssemblesPack(t *testing.T) {
	pack, err := Load(strings.NewReader(loadFixture))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := pack.Aggregation["Fagus sylvatica agg."], "Fagus sylvatica"; got != want {
		t.Errorf("Aggregation[%q] = %q, want %q", "Fagus sylvatica agg.", got, want)
	}
	if len(pack.Groups["Trees"]) != 2 {
		t.Errorf("Groups[Trees] = %v, want 2 members", pack.Groups["Trees"])
	}
	if len(pack.Rules) != 3 {
		t.Fatalf("Rules = %d, want 3", len(pack.Rules))
	}
	for _, r := range pack.Rules {
		if r.Formula == nil {
			t.Errorf("rule %s has no parsed formula", r.Label())
		}
	}
}

func TestLoadFlagsUnknownQualifiedGroup(t *testing.T) {
	pack, err := Load(strings.NewReader(loadFixture))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(pack.Issues.UnknownGroups) != 1 || pack.Issues.UnknownGroups[0] != "MissingGroup" {
		t.Errorf("UnknownGroups = %v, want [MissingGroup]", pack.Issues.UnknownGroups)
	}
}

func TestLoadKnownTaxaUnionsGroupMembersAndBareAtoms(t *testing.T) {
	pack, err := Load(strings.NewReader(loadFixture))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := make([]string, 0, len(pack.KnownTaxa))
	for n := range pack.KnownTaxa {
		got = append(got, n)
	}
	sort.Strings(got)
	want := []string{"Corylus avellana", "Fagus sylvatica", "Quercus robur"}
	if len(got) != len(want) {
		t.Fatalf("KnownTaxa = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("KnownTaxa = %v, want %v", got, want)
		}
	}
}

func TestLoadUnqualifiedUnresolvedAtomIsNotFlagged(t *testing.T) {
	// "Fagus sylvatica" in rule T3H is a bare taxon reference (no group
	// prefix, no qualifier). resolveInto (internal/esy/condition.go) always
	// falls back to treating such a name as a taxon, so it can never be an
	// "unknown group" defect — even though it is not itself a group key.
	pack, err := Load(strings.NewReader(loadFixture))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, n := range pack.Issues.UnknownGroups {
		if n == "Fagus sylvatica" {
			t.Errorf("bare taxon atom wrongly flagged as an unknown group: %v", pack.Issues.UnknownGroups)
		}
	}
}
