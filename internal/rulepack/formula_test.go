package rulepack

import "testing"

// shape renders the tree so precedence is visible in one string.
func shape(n Node) string {
	switch v := n.(type) {
	case Leaf:
		return v.Raw
	case And:
		return "(" + shape(v.L) + " & " + shape(v.R) + ")"
	case Or:
		return "(" + shape(v.L) + " | " + shape(v.R) + ")"
	case Not:
		return "(" + shape(v.L) + " &! " + shape(v.R) + ")"
	}
	return "?"
}

func TestParseFormulaPrecedence(t *testing.T) {
	cases := []struct{ in, want string }{
		// AND binds tighter than OR — R semantics.
		{"<a> OR <b> AND <c>", "(a | (b & c))"},
		{"<a> AND <b> OR <c>", "((a & b) | c)"},
		// NOT is binary "and not" and binds tighter than OR.
		{"<a> OR <b> NOT <c>", "(a | (b &! c))"},
		// Parentheses win.
		{"(<a> OR <b>) AND <c>", "((a | b) & c)"},
		// Left associativity within one precedence level.
		{"<a> AND <b> AND <c>", "((a & b) & c)"},
	}
	for _, c := range cases {
		n, err := ParseFormula(c.in)
		if err != nil {
			t.Errorf("ParseFormula(%q): %v", c.in, err)
			continue
		}
		if got := shape(n); got != c.want {
			t.Errorf("ParseFormula(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestParseFormulaRealRule(t *testing.T) {
	in := "(<#TC Shrubs GR 25> NOT (<#TC Trees GR 25> OR <#TC Native-light-canopy-trees GR 15>)) " +
		"AND (<$$C Dunes_Bohn EQ Y_DUNES> AND (<$$C Coast_EEA EQ ATL_COAST> OR <$$C Coast_EEA EQ BAL_COAST>))"
	n, err := ParseFormula(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := n.(And); !ok {
		t.Fatalf("top level should be AND, got %T", n)
	}
}

func TestParseFormulaRejectsUnbalanced(t *testing.T) {
	if _, err := ParseFormula("(<a> AND <b>"); err == nil {
		t.Fatal("want error for unbalanced parentheses")
	}
}
