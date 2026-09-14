package rulepack

import (
	"strings"
	"testing"
)

// The rule file is third-party input: a new upstream release, a backbone table
// or an operator's own file all reach these parsers unchecked. The contract
// they must hold under any input is the one the service depends on — return a
// value or an error, never panic — because a panic in the parser takes the
// start-up down with no diagnosis.

func FuzzParseExpr(f *testing.F) {
	for _, s := range []string{
		"<#TC Quercus GR 25>",
		"##Q Grassland GE 3",
		"#SC Wet-meadows GR #SC Dry-meadows",
		"$$C GR 50",
		"NON ##Q +10 Grp",
		"#T$ EXCEPT",
		"#$$",
		"#5 Grassland",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseExpr(s)
	})
}

func FuzzParseFormula(f *testing.F) {
	for _, s := range []string{
		"<A> AND <B>",
		"<A> OR NOT <B>",
		"(<A> AND <B>) OR <C>",
		"NOT (<A>)",
		"<A",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseFormula(s)
	})
}

func FuzzSplitSections(f *testing.F) {
	f.Add("SECTION 1: Species aggregation\n Quercus robur\n\tQuercus petraea\nSECTION 1: End\n")
	f.Add("SECTION 3: Rules\nT1A <A>\nSECTION 3: End\n")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = SplitSections(strings.NewReader(s))
	})
}

func FuzzLoad(f *testing.F) {
	f.Add(loadFixture)
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Load(strings.NewReader(s))
	})
}
