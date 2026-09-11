package esy

import (
	"math"
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

func testEnv() Env {
	return Env{Groups: map[string][]string{
		"Trees":  {"Fagus sylvatica", "Quercus robur", "Carpinus betulus"},
		"Shrubs": {"Corylus avellana"},
	}}
}

func testPlot() Plot {
	return Plot{
		Records: []taxa.Record{
			{Name: "Fagus sylvatica", Cover: 10},
			{Name: "Quercus robur", Cover: 9},
			{Name: "Carpinus betulus", Cover: 8},
			{Name: "Corylus avellana", Cover: 30},
			{Name: "Anemone nemorosa", Cover: 40},
		},
		Header: map[string]string{
			"Country":      "Germany",
			"Coast_EEA":    "N_COAST",
			"Altitude (m)": "250",
			"DEG_LAT":      "49.79",
		},
	}
}

func evalRaw(t *testing.T, raw string) (Tri, float64, float64) {
	t.Helper()
	x, err := rulepack.ParseExpr(raw)
	if err != nil {
		t.Fatalf("ParseExpr(%q): %v", raw, err)
	}
	return testEnv().EvalExpr(x, testPlot())
}

func TestEvalTotalCoverOfGroup(t *testing.T) {
	// 10, 9, 8 -> 24.652 by Jennings-Fischer, NOT 27 by addition.
	got, left, _ := evalRaw(t, "#TC Trees GR 25")
	if math.Abs(left-24.652) > 1e-9 {
		t.Errorf("left value = %v, want 24.652", left)
	}
	if got != False {
		t.Errorf("24.652 GR 25 = %v, want FALSE — addition would wrongly pass", got)
	}
}

func TestEvalSpeciesCount(t *testing.T) {
	if got, left, _ := evalRaw(t, "### Trees GR 2"); got != True || left != 3 {
		t.Errorf("### Trees = %v (left %v), want TRUE with 3", got, left)
	}
}

func TestEvalAtLeastNSpecies(t *testing.T) {
	if got, _, _ := evalRaw(t, "#03 Trees"); got != True {
		t.Errorf("#03 Trees = %v, want TRUE (3 species present)", got)
	}
	if got, _, _ := evalRaw(t, "#04 Trees"); got != False {
		t.Errorf("#04 Trees = %v, want FALSE", got)
	}
}

func TestEvalSumOfCovers(t *testing.T) {
	if _, left, _ := evalRaw(t, "##C Trees GR 0"); math.Abs(left-27) > 1e-9 {
		t.Errorf("##C = %v, want 27 (plain sum)", left)
	}
}

func TestEvalSumOfSquareRoots(t *testing.T) {
	want := math.Sqrt(10) + math.Sqrt(9) + math.Sqrt(8)
	if _, left, _ := evalRaw(t, "##Q Trees GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("##Q = %v, want %v", left, want)
	}
}

func TestEvalGroupUnion(t *testing.T) {
	// Trees plus Shrubs: 10, 9, 8, 30.
	want := TotalCover([]float64{10, 9, 8, 30})
	if _, left, _ := evalRaw(t, "#TC Trees|#TC Shrubs GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("union = %v, want %v", left, want)
	}
}

func TestEvalExcept(t *testing.T) {
	// Trees EXCEPT Shrubs is just Trees here.
	want := TotalCover([]float64{10, 9, 8})
	if _, left, _ := evalRaw(t, "#TC Trees|#TC Shrubs EXCEPT #TC Shrubs GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("EXCEPT = %v, want %v", left, want)
	}
}

func TestEvalTotalCoverExcludingGroup(t *testing.T) {
	// #T$ for Trees: everything except the Trees = Corylus 30 and Anemone 40.
	want := TotalCover([]float64{30, 40})
	_, left, right := evalRaw(t, "#TC Trees GR #T$ Trees")
	if math.Abs(right-want) > 1e-9 {
		t.Errorf("#T$ = %v, want %v", right, want)
	}
	if left >= right {
		t.Errorf("trees (%v) should not dominate the rest (%v)", left, right)
	}
}

func TestEvalHighestCoverOutsideGroup(t *testing.T) {
	// #$$ EXCEPT Trees: highest single cover outside Trees = Anemone 40.
	_, _, right := evalRaw(t, "#SC Trees GE #$$ EXCEPT Trees")
	if math.Abs(right-40) > 1e-9 {
		t.Errorf("#$$ = %v, want 40", right)
	}
}

func TestEvalSingleSpeciesCoverInGroup(t *testing.T) {
	// #SC Trees: the highest single cover inside Trees = Fagus 10.
	_, left, _ := evalRaw(t, "#SC Trees GE 0")
	if math.Abs(left-10) > 1e-9 {
		t.Errorf("#SC = %v, want 10", left)
	}
}

func TestEvalPercentOfTotalCover(t *testing.T) {
	total := TotalCover([]float64{10, 9, 8, 30, 40})
	_, _, right := evalRaw(t, "#TC Trees GE $50")
	if math.Abs(right-total*0.5) > 1e-9 {
		t.Errorf("$50 = %v, want %v (half of total cover %v)", right, total*0.5, total)
	}
}

func TestEvalCategoricalHeader(t *testing.T) {
	if got, _, _ := evalRaw(t, "$$C Country EQ Germany"); got != True {
		t.Errorf("Country EQ Germany = %v, want TRUE", got)
	}
	if got, _, _ := evalRaw(t, "$$C Country EQ France"); got != False {
		t.Errorf("Country EQ France = %v, want FALSE", got)
	}
}

func TestEvalNumericHeader(t *testing.T) {
	if got, _, _ := evalRaw(t, "$$N Altitude (m) GR 100"); got != True {
		t.Errorf("Altitude GR 100 = %v, want TRUE", got)
	}
	if got, _, _ := evalRaw(t, "$$N Altitude (m) GR 1000"); got != False {
		t.Errorf("Altitude GR 1000 = %v, want FALSE", got)
	}
}

func TestEvalMissingHeaderIsUnknown(t *testing.T) {
	x, _ := rulepack.ParseExpr("$$N Ecoreg EQ 664")
	got, _, _ := testEnv().EvalExpr(x, testPlot())
	if got != Unknown {
		t.Errorf("missing header = %v, want Unknown", got)
	}
}

// TestEvalUnknownQualifiedGroupIsUnknown covers correction 2, step 2: an atom
// that carries a qualifier but whose key resolves to no group in Env.Groups
// means a group was meant and is missing — the condition is Unknown, not a
// count of zero.
func TestEvalUnknownQualifiedGroupIsUnknown(t *testing.T) {
	x, err := rulepack.ParseExpr("#TC +99 Nonexistent-group GR 5")
	if err != nil {
		t.Fatalf("ParseExpr: %v", err)
	}
	got, left, _ := testEnv().EvalExpr(x, testPlot())
	if got != Unknown {
		t.Errorf("unknown qualified group = %v, want Unknown", got)
	}
	if !math.IsNaN(left) {
		t.Errorf("left = %v, want NaN", left)
	}
}

// TestEvalUnresolvedBareNameIsTreatedAsAbsentTaxon covers correction 2, step
// 3: a name with no qualifier that is not a known group must NOT be guessed
// as an undefined group (the brief's ContainsAny("-") heuristic did that,
// which is wrong — see TestEvalHyphenatedTaxonName below). It is a taxon,
// simply absent from the plot, so its measure is 0 and the condition is
// FALSE, never Unknown.
func TestEvalUnresolvedBareNameIsTreatedAsAbsentTaxon(t *testing.T) {
	got, left, _ := evalRaw(t, "Nonexistent-taxon GR 5")
	if got != False {
		t.Errorf("absent taxon = %v, want FALSE", got)
	}
	if left != 0 {
		t.Errorf("left = %v, want 0 (species absent)", left)
	}
}

// TestEvalHyphenatedTaxonName is the concrete case correction 2 exists for:
// "Abies borisii-regis" contains a hyphen but is a genuine bare taxon name,
// not a group reference. It must resolve as a taxon (absent here, cover 0),
// not be rejected as an unresolvable group.
func TestEvalHyphenatedTaxonName(t *testing.T) {
	got, left, _ := evalRaw(t, "Abies borisii-regis GR 5")
	if got != False {
		t.Errorf("Abies borisii-regis GR 5 = %v, want FALSE (absent, cover 0)", got)
	}
	if left != 0 {
		t.Errorf("left = %v, want 0", left)
	}
}

func TestEvalBareTaxonName(t *testing.T) {
	if got, _, _ := evalRaw(t, "Corylus avellana GR 25"); got != True {
		t.Errorf("bare taxon name = %v, want TRUE (cover 30)", got)
	}
}

// TestEvalNonInnerMeasure covers correction 3: NON is a composite atom whose
// Inner field names the measure to compute over the species OUTSIDE the
// group. Outside Trees: Corylus (30) and Anemone (40). ##C is the plain sum,
// 70 — NOT TotalCover's Jennings-Fischer union, which would give 58. Using
// the wrong measure here would silently produce 58 instead of 70, so this
// assertion is what actually pins Inner down.
func TestEvalNonInnerMeasure(t *testing.T) {
	x, err := rulepack.ParseExpr("NON ##C Trees GR 0")
	if err != nil {
		t.Fatalf("ParseExpr: %v", err)
	}
	got, left, _ := testEnv().EvalExpr(x, testPlot())
	if math.Abs(left-70) > 1e-9 {
		t.Errorf("NON ##C Trees = %v, want 70 (30+40 plain sum)", left)
	}
	if got != True {
		t.Errorf("70 GR 0 = %v, want TRUE", got)
	}
	if wrong := TotalCover([]float64{30, 40}); math.Abs(left-wrong) < 1e-9 {
		t.Fatalf("test is not distinguishing: TotalCover would also give %v", wrong)
	}
}

// TestEvalWithinQualifierComparisonSet: an expression with no right-hand
// side compares the group's measure against every OTHER group in the SAME
// +NN comparison set only (spec 5.1) — not every group in the rule pack. "+01
// A" (cover 10) must be compared against "+01 B" (cover 5), not against "+02
// C" (cover 90), which would wrongly flip the result to FALSE.
func TestEvalWithinQualifierComparisonSet(t *testing.T) {
	env := Env{Groups: map[string][]string{
		"+01 A": {"Sp1"},
		"+01 B": {"Sp2"},
		"+02 C": {"Sp3"},
	}}
	plot := Plot{Records: []taxa.Record{
		{Name: "Sp1", Cover: 10},
		{Name: "Sp2", Cover: 5},
		{Name: "Sp3", Cover: 90},
	}}
	x, err := rulepack.ParseExpr("#TC +01 A")
	if err != nil {
		t.Fatalf("ParseExpr: %v", err)
	}
	got, left, right := env.EvalExpr(x, plot)
	if math.Abs(left-10) > 1e-9 {
		t.Errorf("left = %v, want 10", left)
	}
	if math.Abs(right-5) > 1e-9 {
		t.Errorf("right (best of comparison set) = %v, want 5 (Sp3's 90 is a different qualifier and must be excluded)", right)
	}
	if got != True {
		t.Errorf("10 GR 5 = %v, want TRUE", got)
	}
}
