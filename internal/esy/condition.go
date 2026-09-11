package esy

import (
	"math"
	"strconv"
	"strings"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// Plot is one resolved vegetation plot.
type Plot struct {
	Records []taxa.Record
	Header  map[string]string
}

// Env holds everything the evaluator needs besides the plot. Groups is keyed
// exactly as ParseGroups produces it: the qualifier, when the atom carries
// one, is part of the key ("+01 MA211-...").
type Env struct {
	Groups map[string][]string
}

// EvalExpr evaluates one membership expression and returns its truth value
// plus the numeric values of both sides. The numbers are the intermediate
// results the golden master compares against upstream's plot.cond matrix.
//
// Tri and its Kleene operators (And/Or/AndNot, used by the formula layer
// above this one) stay available, but EvalExpr itself never returns Unknown.
// v1.2 zeroes every unresolved condition value before any logical evaluation
// happens (step3and5_extract-and-solve-membership-conditions.R:450-452, and
// again in prep.R:62-63):
//
//	if(any(is.na(plot.cond))) warning('NA in plot.cond')
//	plot.cond[is.na(plot.cond)] <- 0
//	plot.cond[plot.cond == -Inf] <- 0
//
// So there is no NA/Unknown propagation in the implementation being ported: a
// missing header value, an unresolvable qualified group reference, or an
// empty max() all become the number 0, and the comparison then resolves to
// an ordinary True or False. This overturns the three-valued design in spec
// §5.3, which describes the 2019 standalone script rather than v1.2. Tri
// stays exactly as it is for the formula layer, and for a future rule pack
// or upstream version that reintroduces NA.
func (e Env) EvalExpr(x rulepack.Expr, p Plot) (Tri, float64, float64) {
	left := e.operandValue(x.Left, p, x)
	if x.Op == "" {
		// Left-hand-only expressions carry an implicit "GR NON <same
		// group>": the +NN qualifier names a comparison set of groups, and
		// the condition means "greater than every other group of that set"
		// (spec 5.1).
		return e.compareWithinSet(x, p, left)
	}
	if isCategorical(x.Left) {
		return e.compareCategorical(x, p), 0, 0
	}
	right := e.operandValue(x.Right, p, x)
	switch x.Op {
	case "GR":
		return FromBool(left > right), left, right
	case "GE":
		return FromBool(left >= right), left, right
	case "EQ":
		return FromBool(left == right), left, right
	}
	return False, left, right
}

func isCategorical(o rulepack.Operand) bool {
	return len(o.Atoms) == 1 && o.Atoms[0].Kind == "$$C"
}

// compareCategorical compares a categorical header field. R turns the header
// column into factor levels and compares level numbers; a missing value
// becomes factor level 0, which matches no real level — i.e. false, not
// Unknown (see EvalExpr's doc comment).
func (e Env) compareCategorical(x rulepack.Expr, p Plot) Tri {
	field := x.Left.Atoms[0].Name
	have, ok := p.Header[field]
	if !ok || have == "" {
		return False
	}
	return FromBool(have == x.Right.Literal)
}

// compareWithinSet handles expressions with no right-hand side: the measure
// of the named group must exceed that of every other group in the same +NN
// comparison set (or, for an atom with no qualifier, every other group at
// all — the qualifier is what defines "the same set").
func (e Env) compareWithinSet(x rulepack.Expr, p Plot, left float64) (Tri, float64, float64) {
	a := x.Left.Atoms[0]
	if isCountPrefixKind(a.Kind) {
		// "#03 Group" is a standalone predicate: at least N species present.
		return FromBool(left > 0), left, 0
	}
	best := e.bestOfComparisonSet(a.Kind, a.Qualifier, groupKey(a), p)
	return FromBool(left > best), left, best
}

// bestOfComparisonSet returns the highest value of the named measure kind
// over every OTHER group sharing the given qualifier — the "+NN comparison
// set" of spec 5.1 — excluding the group itself (ownKey). An atom with no
// qualifier is compared against every other group. Both compareWithinSet
// (operator-less expressions) and NON atoms (see measure) resolve to this;
// R implements the same restriction in both places, step3and5…R:361-380:
//
//	# only groups of the same set are compared with each other
//	# at the same time the group itself is excluded
//	group.set <- substr(conditions.wn[j],5,7)
//	index9 <- which(substr(pgna,5,7)==group.set & pgna!=conditions.wn[j])
func (e Env) bestOfComparisonSet(kind, qualifier, ownKey string, p Plot) float64 {
	best := 0.0
	for key, members := range e.Groups {
		if key == ownKey {
			continue
		}
		if qualifier != "" && !strings.HasPrefix(key, qualifier+" ") {
			continue
		}
		if v := computeMeasure(kind, coversOf(p, toSet(members))); v > best {
			best = v
		}
	}
	return best
}

// groupKey is the lookup key ParseGroups produces for a group header: the
// qualifier plus the name when the atom carries a qualifier, the bare name
// otherwise. This is what makes qualified references resolvable — dropping
// the qualifier would fail every one of them.
func groupKey(a rulepack.Atom) string {
	if a.Qualifier != "" {
		return a.Qualifier + " " + a.Name
	}
	return a.Name
}

func (e Env) operandValue(o rulepack.Operand, p Plot, x rulepack.Expr) float64 {
	if o.Literal != "" {
		return e.literalValue(o.Literal, p)
	}
	if len(o.Atoms) == 0 {
		return 0
	}
	a := o.Atoms[0]

	// Header atoms. A missing or unparsable value becomes 0 (see EvalExpr).
	if a.Kind == "$$N" {
		v, ok := p.Header[a.Name]
		if !ok || v == "" {
			return 0
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0
		}
		return f
	}

	// A bare "#T$" with no group name of its own takes the group named on
	// the other side of the expression (spec 5.1) — but only when it has no
	// EXCEPT of its own; see measure() for why EXCEPT changes everything.
	// "#$$" never borrows this way: every real occurrence is either bare
	// (plot-wide maximum) or carries its own EXCEPT group directly.
	atoms := o.Atoms
	if a.Kind == "#T$" && len(o.Except) == 0 && allNamesEmpty(atoms) {
		atoms = x.Left.Atoms
	}
	return e.measure(a, atoms, o.Except, p)
}

func allNamesEmpty(atoms []rulepack.Atom) bool {
	for _, a := range atoms {
		if a.Name != "" {
			return false
		}
	}
	return true
}

func (e Env) literalValue(lit string, p Plot) float64 {
	if strings.HasPrefix(lit, "$") {
		pct, err := strconv.ParseFloat(lit[1:], 64)
		if err != nil {
			return 0
		}
		return e.plotTotal(p, nil) * pct / 100
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return 0
	}
	return f
}

// measure computes one of the ten condition kinds. lead is the atom that
// names the kind (o.Atoms[0], possibly with Inner set for "NON"); atoms and
// except are the (possibly borrowed) group/taxon references to resolve.
func (e Env) measure(lead rulepack.Atom, atoms, except []rulepack.Atom, p Plot) float64 {
	switch lead.Kind {
	case "#T$":
		if len(except) > 0 {
			// "#T$ EXCEPT <group>" occurs twice in the real file. R's
			// "fourth: deal with EXCEPT only" block builds the base set to
			// exclude from substr(a[1], 5, nchar(a[1])) where a[1] is the
			// text "#T$" itself (3 characters) — substr with a start past
			// the string's end yields "", so the base set is always empty,
			// no relevé rows are ever written for this condition, and the
			// value stays at R's zero-filled default. The named EXCEPT
			// group is never actually used. Reproduced verbatim, defect and
			// all: step3and5…R:205-225.
			return 0
		}
		in, ok := e.membersChecked(atoms, nil)
		if !ok {
			return 0
		}
		return e.plotTotal(p, in)
	case "#$$":
		if len(except) == 0 {
			// Bare "#$$" (all 74 real occurrences) is the plot-wide maximum
			// cover, INCLUDING the group being compared — not "always with
			// EXCEPT" as originally assumed. step3and5…R:296-301.
			return maxOf(coversOutside(p, nil))
		}
		// "#$$ EXCEPT <group>": the maximum over species NOT in that named
		// group. Zero real occurrences, but R implements it,
		// step3and5…R:305-317.
		in, ok := e.membersChecked(except, nil)
		if !ok {
			return 0
		}
		return e.highestOutside(p, in)
	case "NON":
		// NON compares against the other groups of the SAME +NN comparison
		// set, exactly like an operator-less expression — it is not a
		// measure over the complement species set. See
		// bestOfComparisonSet's doc comment for the R citation.
		return e.bestOfComparisonSet(lead.Inner, lead.Qualifier, groupKey(lead), p)
	}

	in, ok := e.membersChecked(atoms, except)
	if !ok {
		return 0
	}
	return computeMeasure(lead.Kind, coversOf(p, in))
}

// computeMeasure evaluates one condition kind over an already-resolved set
// of covers (also used, with a NON atom's Inner kind, inside
// bestOfComparisonSet).
func computeMeasure(kind string, covers []float64) float64 {
	switch {
	case kind == "#TC":
		return TotalCover(covers)
	case kind == "##C":
		return sum(covers)
	case kind == "##Q":
		return sumSqrt(covers)
	case kind == "###" || kind == "##D":
		return float64(len(covers))
	case kind == "#SC":
		return maxOf(covers)
	case isCountPrefixKind(kind):
		n, _ := strconv.Atoi(kind[1:])
		return boolToFloat(len(covers) >= n)
	case kind == "":
		// A bare taxon name: its own cover.
		return maxOf(covers)
	}
	return 0
}

func isCountPrefixKind(k string) bool {
	return len(k) == 3 && k[0] == '#' && k[1] >= '0' && k[1] <= '9' && k[2] >= '0' && k[2] <= '9'
}

// membersChecked resolves a union of atoms (minus an except union) to the set
// of member taxon names. Each atom is looked up as a group first; only when
// that fails does it fall back to being a bare taxon name — never guessed
// from its spelling. See resolveInto. ok is false only when a qualified
// group reference cannot be resolved; callers turn that into the value 0
// (see EvalExpr's doc comment), not Unknown.
func (e Env) membersChecked(atoms, except []rulepack.Atom) (map[string]bool, bool) {
	in := map[string]bool{}
	for _, a := range atoms {
		if a.Name == "" {
			continue
		}
		if !e.resolveInto(a, in, true) {
			return nil, false
		}
	}
	for _, a := range except {
		if a.Name == "" {
			continue
		}
		if !e.resolveInto(a, in, false) {
			return nil, false
		}
	}
	return in, true
}

// resolveInto adds (add=true) or removes (add=false) one atom's resolved
// members from the set.
//
// Resolution order:
//  1. Its key (groupKey: qualifier + name, or just name) names a known
//     group — use its members.
//  2. Otherwise, if the atom carries a qualifier, a group was meant and is
//     missing: report failure so the caller falls back to the value 0.
//  3. Otherwise, treat the name as a single taxon. This is deliberately NOT
//     guessed from the spelling (e.g. a hyphen) — "Abies borisii-regis" is a
//     bare taxon name that happens to contain a hyphen, and guessing by
//     spelling would misclassify it as an unresolved group forever.
func (e Env) resolveInto(a rulepack.Atom, in map[string]bool, add bool) bool {
	key := groupKey(a)
	if members, ok := e.Groups[key]; ok {
		for _, m := range members {
			if add {
				in[m] = true
			} else {
				delete(in, m)
			}
		}
		return true
	}
	if a.Qualifier != "" {
		return false
	}
	if add {
		in[a.Name] = true
	} else {
		delete(in, a.Name)
	}
	return true
}

func toSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func coversOf(p Plot, in map[string]bool) []float64 {
	var out []float64
	for _, r := range p.Records {
		if in[r.Name] {
			out = append(out, r.Cover)
		}
	}
	return out
}

// coversOutside returns the covers of every record NOT in in. A nil map
// matches nothing, so this also serves as "every cover in the plot".
func coversOutside(p Plot, in map[string]bool) []float64 {
	var out []float64
	for _, r := range p.Records {
		if !in[r.Name] {
			out = append(out, r.Cover)
		}
	}
	return out
}

// plotTotal is the total cover of the plot, excluding the given taxa.
func (e Env) plotTotal(p Plot, exclude map[string]bool) float64 {
	var cs []float64
	for _, r := range p.Records {
		if exclude != nil && exclude[r.Name] {
			continue
		}
		cs = append(cs, r.Cover)
	}
	return TotalCover(cs)
}

func (e Env) highestOutside(p Plot, in map[string]bool) float64 {
	return maxOf(coversOutside(p, in))
}

func sum(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

func sumSqrt(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += math.Sqrt(x)
	}
	return t
}

func maxOf(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
