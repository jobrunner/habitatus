package rulepack

import (
	"reflect"
	"testing"
)

func TestParseExpr(t *testing.T) {
	cases := []struct {
		in   string
		want Expr
	}{
		{
			in: "#TC Trees GR 25",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#TC", Name: "Trees"}}},
				Op:    "GR",
				Right: Operand{Literal: "25"},
			},
		},
		{
			in: "$$N Altitude (m) GR 1000",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "$$N", Name: "Altitude (m)"}}},
				Op:    "GR",
				Right: Operand{Literal: "1000"},
			},
		},
		{
			in: "$$C Country EQ United Kingdom",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "$$C", Name: "Country"}}},
				Op:    "EQ",
				Right: Operand{Literal: "United Kingdom"},
			},
		},
		{
			in: "Empetrum nigrum aggr. GR 05",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Name: "Empetrum nigrum aggr."}}},
				Op:    "GR",
				Right: Operand{Literal: "05"},
			},
		},
		{
			in: "##Q +12 Coastal-saltmarsh-species",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "##Q", Qualifier: "+12", Name: "Coastal-saltmarsh-species"}}},
			},
		},
		{
			in: "#TC Trees|#TC Shrubs GR 15",
			want: Expr{
				Left: Operand{Atoms: []Atom{
					{Kind: "#TC", Name: "Trees"},
					{Kind: "#TC", Name: "Shrubs"},
				}},
				Op:    "GR",
				Right: Operand{Literal: "15"},
			},
		},
		{
			in: "#TC Bog-Pinus GR #TC Trees|#TC Shrubs EXCEPT #TC Bog-Pinus",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "#TC", Name: "Bog-Pinus"}}},
				Op:   "GR",
				Right: Operand{
					Atoms:  []Atom{{Kind: "#TC", Name: "Trees"}, {Kind: "#TC", Name: "Shrubs"}},
					Except: []Atom{{Kind: "#TC", Name: "Bog-Pinus"}},
				},
			},
		},
		{
			in: "#SC W-acidic-garrigue-shrubs GE #$$",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#SC", Name: "W-acidic-garrigue-shrubs"}}},
				Op:    "GE",
				Right: Operand{Atoms: []Atom{{Kind: "#$$"}}},
			},
		},
		{
			in: "#TC Dry-heath-shrubs GE $50",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#TC", Name: "Dry-heath-shrubs"}}},
				Op:    "GE",
				Right: Operand{Literal: "$50"},
			},
		},
		{
			in: "#01 +04 R52-Forest-fringe",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "#01", Qualifier: "+04", Name: "R52-Forest-fringe"}}},
			},
		},
		{
			// The glued operator is NOT acted on: upstream never splits this
			// expression, so the whole text is one condition and the group
			// name it looks up is "Cliff-ferns GR05", which matches nothing.
			// See ParseExpr.
			in: "#TC Cliff-ferns GR05",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "#TC", Name: "Cliff-ferns GR05"}}},
			},
		},
	}
	for _, c := range cases {
		got, err := ParseExpr(c.in)
		if err != nil {
			t.Errorf("ParseExpr(%q): %v", c.in, err)
			continue
		}
		if !exprEqual(got, c.want) {
			t.Errorf("ParseExpr(%q):\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
}

func TestParseExprNonIsComposite(t *testing.T) {
	got, err := ParseExpr("##Q +10 D-Mires GR NON ##Q +10 D-Mires")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Right.Atoms) != 1 {
		t.Fatalf("right side: %+v", got.Right)
	}
	a := got.Right.Atoms[0]
	if a.Kind != "NON" || a.Inner != "##Q" || a.Qualifier != "+10" || a.Name != "D-Mires" {
		t.Errorf("NON atom = %+v, want Kind NON, Inner ##Q, Qualifier +10, Name D-Mires", a)
	}
}

func TestExtractExpressions(t *testing.T) {
	got := extractExpressions("(<a GR 1> OR <b>) NOT <c>")
	want := []string{"a GR 1", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractExpressions: got %v, want %v", got, want)
	}
}

// TestParseExprSCExcept covers upstream's step 3B: an expression whose
// right-hand side names a "#SC" group gets "EXCEPT <left-hand side>" appended,
// so the group maximum is taken over every species except the one being
// compared (ParsingExpertFile.R:404-430).
func TestParseExprSCExcept(t *testing.T) {
	cases := []struct {
		in   string
		want Expr
	}{
		{
			in: "Fagus sylvatica GR #SC Trees",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Name: "Fagus sylvatica"}}},
				Op:   "GR",
				Right: Operand{
					Atoms:  []Atom{{Kind: "#SC", Name: "Trees"}},
					Except: []Atom{{Name: "Fagus sylvatica"}},
				},
			},
		},
		{
			// "#SC" on the left-hand side alone triggers nothing.
			in: "#SC Charoids GE #$$",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#SC", Name: "Charoids"}}},
				Op:    "GE",
				Right: Operand{Atoms: []Atom{{Kind: "#$$"}}},
			},
		},
	}
	for _, c := range cases {
		got, err := ParseExpr(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if !exprEqual(got, c.want) {
			t.Errorf("%q:\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
}

// TestParseExprAlwaysFalse covers upstream's degenerate expressions. An
// expression that does not split into a left and a right condition evaluates
// to a number rather than a logical, and step 8 overwrites every numeric
// result with FALSE (step3and5...R:493). Upstream splits conditions on the
// SPACED operators " GR ", " GE ", " EQ " only, so an expression whose only
// operator is glued to its operand never splits.
func TestParseExprAlwaysFalse(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Glued operator: "GR05" leaves the expression unsplit.
		{"#TC Cliff-ferns GR05", true},
		// "#NN Group" carries no operator at all and is excluded from the
		// "GR NON" rewrite, so it too stays unsplit.
		{"#03 Coastal-saltmarsh-specialists", true},
		{"#02 +04 R22-Low-and-medium-altitude-hay-meadow", true},
		// No operator, but not a "#NN" form: upstream appends " GR NON <self>".
		{"##Q +12 Coastal-saltmarsh-species", false},
		{"#TC Trees GR 25", false},
	}
	for _, c := range cases {
		got, err := ParseExpr(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got.AlwaysFalse != c.want {
			t.Errorf("%q: AlwaysFalse = %v, want %v", c.in, got.AlwaysFalse, c.want)
		}
	}
}

func exprEqual(a, b Expr) bool {
	return a.Op == b.Op && operandEqual(a.Left, b.Left) && operandEqual(a.Right, b.Right)
}

func operandEqual(a, b Operand) bool {
	if a.Literal != b.Literal || len(a.Atoms) != len(b.Atoms) || len(a.Except) != len(b.Except) {
		return false
	}
	for i := range a.Atoms {
		if a.Atoms[i] != b.Atoms[i] {
			return false
		}
	}
	for i := range a.Except {
		if a.Except[i] != b.Except[i] {
			return false
		}
	}
	return true
}

// The parser's risk model is that a silent misparse is worse than a crash:
// the rule file is the specification, and a shape we do not understand must
// stop the load rather than evaluate into a plausible wrong answer. These
// three shapes do not occur in the 2025-10-03 file, and each of them used to
// parse into something quietly different from what it says.
func TestParseExprRejectsShapesItCannotRepresent(t *testing.T) {
	cases := []struct {
		in, why string
	}{
		{
			"#TC Trees EXCEPT Fagus sylvatica EXCEPT Picea abies",
			"only the first EXCEPT was honoured; the second was absorbed into the atom name",
		},
		{"#TC Trees GR ", "an operator with no right-hand side yielded an empty operand"},
		{"#TC Trees GR", "same, with no trailing space"},
	}
	for _, c := range cases {
		if _, err := ParseExpr(c.in); err == nil {
			t.Errorf("ParseExpr(%q) must fail: %s", c.in, c.why)
		}
	}
}

// Nonea is a real Euro+Med genus, and upstream matches the NON prefix with
// startsWith/grep as well (step3and5…R:289, 328) — neither side checks for a
// word boundary. No name in today's file begins with the three capitals, so
// nothing misparses; a name that did would be read as NON plus the rest,
// silently and with a truth value of its own. Refuse to guess instead.
func TestParseExprNONNeedsABoundary(t *testing.T) {
	if _, err := ParseExpr("#TC Trees GR NONEA PULLA"); err == nil {
		t.Error("NONEA PULLA must not be read as NON + EA PULLA without a word boundary")
	}

	// The genus as actually spelled is unaffected: the prefix match is
	// case-sensitive, so "Nonea pulla" never looked like a NON atom.
	if _, err := ParseExpr("#TC Trees GR Nonea pulla"); err != nil {
		t.Errorf("Nonea pulla: %v", err)
	}

	// The real NON prefix, with its space, must keep working.
	got, err := ParseExpr("##Q +10 D-Mires GR NON ##Q +10 D-Mires")
	if err != nil {
		t.Fatal(err)
	}
	if a := got.Right.Atoms[0]; a.Kind != "NON" {
		t.Errorf("NON atom lost: %+v", a)
	}
}
