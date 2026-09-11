package rulepack

import "testing"

func TestParseRuleHeaderFixedWidth(t *testing.T) {
	lines := []string{
		"7          MA211 Arctic coastal saltmarsh",
		"<#TC Trees GR 15>",
		"",
		"4          N15! Atlantic and Baltic coastal dune grassland",
		"<#02 N15-specialists>",
		"",
		"2          N15!!Atlantic and Baltic coastal dune grassland",
		"<##Q +04 R1Q>",
		"",
		"2          MAa  Angiosperm vegetation in the marine littoral zone",
		"<##Q +04 MAa>",
	}
	got, err := ParseRuleHeaders(lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rules, got %d", len(got))
	}
	want := []struct{ prio int; code, variant, name string }{
		{7, "MA211", "", "Arctic coastal saltmarsh"},
		{4, "N15", "!", "Atlantic and Baltic coastal dune grassland"},
		{2, "N15", "!!", "Atlantic and Baltic coastal dune grassland"},
		{2, "MAa", "", "Angiosperm vegetation in the marine littoral zone"},
	}
	for i, w := range want {
		g := got[i]
		if g.Priority != w.prio || g.Code != w.code || g.Variant != w.variant || g.Name != w.name {
			t.Errorf("rule %d = %+v, want %v", i, g, w)
		}
	}
}

// Rule S63 spans three lines, breaking after OR and before NOT.
func TestParseRuleHeadersJoinsContinuationLines(t *testing.T) {
	lines := []string{
		"4          S63  Eastern garrigue",
		"((<#TC E-garrigue-shrubs GR 25> AND (<#TC E-garrigue-shrubs GE $50>)) OR",
		"(<#03 E-garrigue-herbs>))",
		"NOT <#TC Trees GR 10>",
	}
	got, err := ParseRuleHeaders(lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 rule, got %d", len(got))
	}
	want := "((<#TC E-garrigue-shrubs GR 25> AND (<#TC E-garrigue-shrubs GE $50>)) OR " +
		"(<#03 E-garrigue-herbs>)) NOT <#TC Trees GR 10>"
	if got[0].Raw != want {
		t.Errorf("joined formula:\n got %q\nwant %q", got[0].Raw, want)
	}
}

func TestParseRuleHeadersRejectsFormulaWithoutHeader(t *testing.T) {
	_, err := ParseRuleHeaders([]string{"<#TC Trees GR 15>"})
	if err == nil {
		t.Fatal("want error for formula without a preceding header")
	}
}
