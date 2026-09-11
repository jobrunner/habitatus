package esy

import (
	"sort"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

// Match is one rule that evaluated TRUE.
type Match struct {
	Code     string
	Variant  string
	Priority int
}

// Result is the outcome for one plot.
type Result struct {
	// Winner is a EUNIS code, "?" (no match) or "+" (ambiguous at every level).
	Winner string
	// Matches are all rules that fired, in file order.
	Matches []Match
	// Conditions maps each evaluated expression to its left and right numeric
	// value. This is the seam the golden master compares against upstream.
	Conditions map[string][2]float64
}

// Evaluate runs every rule against the plot.
func (e Env) Evaluate(rules []rulepack.Rule, p Plot) Result {
	res := Result{Conditions: map[string][2]float64{}}
	for _, r := range rules {
		if e.node(r.Formula, p, res.Conditions).IsTrue() {
			res.Matches = append(res.Matches, Match{
				Code: r.Code, Variant: r.Variant, Priority: r.Priority,
			})
		}
	}
	res.Winner = winner(res.Matches)
	return res
}

func (e Env) node(n rulepack.Node, p Plot, cond map[string][2]float64) Tri {
	switch v := n.(type) {
	case rulepack.Leaf:
		t, l, r := e.EvalExpr(v.Expr, p)
		cond[v.Raw] = [2]float64{l, r}
		return t
	case rulepack.And:
		return e.node(v.L, p, cond).And(e.node(v.R, p, cond))
	case rulepack.Or:
		return e.node(v.L, p, cond).Or(e.node(v.R, p, cond))
	case rulepack.Not:
		return e.node(v.L, p, cond).AndNot(e.node(v.R, p, cond))
	}
	return Unknown
}

// winner reproduces upstream v1.2's classify():
//
//	length 0 -> "?"; length 1 -> that one; otherwise walk priority levels from
//	highest down and return the first level holding exactly one match; "+" if
//	no level does.
func winner(ms []Match) string {
	switch len(ms) {
	case 0:
		return "?"
	case 1:
		return ms[0].Code
	}
	levels := map[int][]Match{}
	for _, m := range ms {
		levels[m.Priority] = append(levels[m.Priority], m)
	}
	keys := make([]int, 0, len(levels))
	for k := range levels {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	for _, k := range keys {
		if len(levels[k]) == 1 {
			return levels[k][0].Code
		}
	}
	return "+"
}
