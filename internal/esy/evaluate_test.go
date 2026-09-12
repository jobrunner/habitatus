package esy

import (
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

func rule(prio int, code, variant, formula string) rulepack.Rule {
	n, err := rulepack.ParseFormula(formula)
	if err != nil {
		panic(err)
	}
	return rulepack.Rule{Priority: prio, Code: code, Variant: variant, Formula: n}
}

func TestEvaluateNoMatchIsQuestionMark(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 99>")}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "?" {
		t.Errorf("winner = %q, want %q", got.Winner, "?")
	}
}

func TestEvaluateSingleMatchWins(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 5>")}
	got := testEnv().Evaluate(rs, testPlot())
	if got.Winner != "T1H" {
		t.Errorf("winner = %q, want T1H", got.Winner)
	}
	if len(got.Matches) != 1 {
		t.Errorf("matches = %+v, want exactly one", got.Matches)
	}
}

func TestEvaluateHighestPriorityWins(t *testing.T) {
	rs := []rulepack.Rule{
		rule(2, "LOW", "", "<#TC Trees GR 5>"),
		rule(7, "HIGH", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "HIGH" {
		t.Errorf("winner = %q, want HIGH", got.Winner)
	}
}

// v1.2 change: an ambiguous top level falls through to the next lower level
// instead of returning "+" immediately.
func TestEvaluateAmbiguousTopLevelDescends(t *testing.T) {
	rs := []rulepack.Rule{
		rule(7, "A", "", "<#TC Trees GR 5>"),
		rule(7, "B", "", "<#TC Trees GR 5>"),
		rule(4, "C", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "C" {
		t.Errorf("winner = %q, want C (descend to the unambiguous level)", got.Winner)
	}
}

func TestEvaluateAmbiguousEverywhereIsPlus(t *testing.T) {
	rs := []rulepack.Rule{
		rule(7, "A", "", "<#TC Trees GR 5>"),
		rule(7, "B", "", "<#TC Trees GR 5>"),
		rule(4, "C", "", "<#TC Trees GR 5>"),
		rule(4, "D", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "+" {
		t.Errorf("winner = %q, want +", got.Winner)
	}
}

// A missing header field evaluates to 0 (see condition.go's EvalExpr doc
// comment), so this is an ordinary false comparison (0 EQ 664), not an
// Unknown value being excluded from the match set.
func TestEvaluateMissingHeaderValueDoesNotMatch(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "X", "", "<$$N Ecoreg EQ 664>")}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "?" {
		t.Errorf("winner = %q, want ? — a missing field zeroes to a false comparison", got.Winner)
	}
}

func TestEvaluateReportsVariant(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "N15", "!!", "<#TC Trees GR 5>")}
	got := testEnv().Evaluate(rs, testPlot())
	if got.Matches[0].Variant != "!!" {
		t.Errorf("variant = %q, want !!", got.Matches[0].Variant)
	}
	if got.Winner != "N15" {
		t.Errorf("winner = %q, want N15 (code without the marker)", got.Winner)
	}
}

func TestEvaluateCollectsConditionValues(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 25>")}
	got := testEnv().Evaluate(rs, testPlot())
	v, ok := got.Conditions["#TC Trees GR 25"]
	if !ok {
		t.Fatalf("condition values not collected: %v", got.Conditions)
	}
	if v[0] == 0 {
		t.Errorf("left value not recorded: %v", v)
	}
}
