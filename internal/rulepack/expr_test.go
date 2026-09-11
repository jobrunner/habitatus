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
			in: "#TC Cliff-ferns GR05",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#TC", Name: "Cliff-ferns"}}},
				Op:    "GR",
				Right: Operand{Literal: "05"},
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
