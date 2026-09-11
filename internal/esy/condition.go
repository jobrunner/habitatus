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

// nan marks a value that could not be computed — a missing header or an
// unresolved qualified group reference. It propagates to Unknown, mirroring
// R's NA.
var nan = math.NaN()

// EvalExpr evaluates one membership expression and returns its truth value
// plus the numeric values of both sides. The numbers are the intermediate
// results the golden master compares against upstream's plot.cond matrix.
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
		return e.compareCategorical(x, p), nan, nan
	}
	right := e.operandValue(x.Right, p, x)
	if math.IsNaN(left) || math.IsNaN(right) {
		return Unknown, left, right
	}
	switch x.Op {
	case "GR":
		return FromBool(left > right), left, right
	case "GE":
		return FromBool(left >= right), left, right
	case "EQ":
		return FromBool(left == right), left, right
	}
	return Unknown, left, right
}

func isCategorical(o rulepack.Operand) bool {
	return len(o.Atoms) == 1 && o.Atoms[0].Kind == "$$C"
}

func (e Env) compareCategorical(x rulepack.Expr, p Plot) Tri {
	field := x.Left.Atoms[0].Name
	have, ok := p.Header[field]
	if !ok || have == "" {
		return Unknown
	}
	return FromBool(have == x.Right.Literal)
}

// compareWithinSet handles expressions with no right-hand side: the measure
// of the named group must exceed that of every other group in the same +NN
// comparison set (or, for an atom with no qualifier, every other group at
// all — the qualifier is what defines "the same set").
func (e Env) compareWithinSet(x rulepack.Expr, p Plot, left float64) (Tri, float64, float64) {
	if math.IsNaN(left) {
		return Unknown, left, nan
	}
	a := x.Left.Atoms[0]
	if isCountPrefixKind(a.Kind) {
		// "#03 Group" is a standalone predicate: at least N species present.
		return FromBool(left > 0), left, nan
	}
	own := groupKey(a)
	best := 0.0
	for key, members := range e.Groups {
		if key == own {
			continue
		}
		if a.Qualifier != "" && !strings.HasPrefix(key, a.Qualifier+" ") {
			continue // not in the same comparison set
		}
		v := computeMeasure(a.Kind, coversOf(p, toSet(members)))
		if !math.IsNaN(v) && v > best {
			best = v
		}
	}
	return FromBool(left > best), left, best
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
		return nan
	}
	a := o.Atoms[0]

	// Header atoms.
	if a.Kind == "$$N" {
		v, ok := p.Header[a.Name]
		if !ok || v == "" {
			return nan
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nan
		}
		return f
	}

	// "#T$" and "#$$" with no group name of their own take the group named
	// on the other side of the expression.
	atoms := o.Atoms
	if (a.Kind == "#T$" || a.Kind == "#$$") && allNamesEmpty(atoms) {
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
			return nan
		}
		return e.plotTotal(p, nil) * pct / 100
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return nan
	}
	return f
}

// measure computes one of the ten condition kinds. lead is the atom that
// names the kind (o.Atoms[0], possibly with Inner set for "NON"); atoms and
// except are the (possibly borrowed) group/taxon references to resolve.
func (e Env) measure(lead rulepack.Atom, atoms, except []rulepack.Atom, p Plot) float64 {
	switch lead.Kind {
	case "#T$":
		// Total plot cover excluding the compared group. #T$ never combines
		// with its own EXCEPT — it already IS an exclusion.
		in, ok := e.membersChecked(atoms, nil)
		if !ok {
			return nan
		}
		return e.plotTotal(p, in)
	case "#$$":
		// Highest single cover outside the compared group.
		in, ok := e.membersChecked(atoms, nil)
		if !ok {
			return nan
		}
		return e.highestOutside(p, in)
	}

	in, ok := e.membersChecked(atoms, except)
	if !ok {
		return nan
	}
	if lead.Kind == "NON" {
		// NON is a composite atom: Inner names the measure, computed over
		// the species OUTSIDE the group, not TotalCover of it.
		return computeMeasure(lead.Inner, coversOutside(p, in))
	}
	return computeMeasure(lead.Kind, coversOf(p, in))
}

// computeMeasure evaluates one condition kind (or a NON atom's Inner
// measure) over an already-resolved set of covers.
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
	return nan
}

func isCountPrefixKind(k string) bool {
	return len(k) == 3 && k[0] == '#' && k[1] >= '0' && k[1] <= '9' && k[2] >= '0' && k[2] <= '9'
}

// membersChecked resolves a union of atoms (minus an except union) to the set
// of member taxon names. Each atom is looked up as a group first; only when
// that fails does it fall back to being a bare taxon name — never guessed
// from its spelling. See resolveInto.
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
//     missing: report failure so the whole condition becomes Unknown.
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
