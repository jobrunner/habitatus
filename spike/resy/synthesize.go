//go:build ignore

// Command synthesize writes rule-driven plots to a JSONL file.
//
//	ESY_FILE=<rule file> go run spike/resy/synthesize.go <out.jsonl>
//
// The Tuexen-Archiv is purely German: Mediterranean, Black Sea and Arctic
// rules never fire there and most of the 312 rules stay unexercised. This
// generator builds plots from the rule pack itself, so the golden master
// covers rules the archive cannot reach.
//
// For every rule it enumerates a few truth assignments of its formula, builds
// the smallest plot that satisfies the expressions the assignment needs TRUE,
// and emits it. For every numeric threshold in that plot it also emits a
// variant that sits just below the threshold, so both sides of each cut are
// compared.
//
// The generator does not decide what a plot means -- upstream does. A plot
// that fails to make its target rule fire is still worth emitting, because R
// and habitatus must agree on it either way.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

// firstSyntheticID keeps synthetic plots clear of the archive's RELEVE_NRs,
// which run to five digits.
const firstSyntheticID = 1000000

// maxAssignments caps how many truth assignments of one formula are tried.
const maxAssignments = 4

// headerFields are the columns of the bundled header table, in its order. A
// synthetic plot must carry exactly these, because upstream binds header rows
// to plot.cond rows positionally and rbind demands identical columns.
var headerFields = []string{
	"Country", "Altitude..m.", "DEG_LON", "DEG_LAT", "GESELLSCH",
	"dataset", "Ecoreg", "Dunes_Bohn", "Coast_EEA",
}

type plot struct {
	ID      int               `json:"id"`
	Records []record          `json:"records"`
	Header  map[string]string `json:"header"`
	Rule    string            `json:"rule"`
}

type record struct {
	Name  string  `json:"name"`
	Cover float64 `json:"cover"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: synthesize <out.jsonl>")
		os.Exit(2)
	}
	f, err := os.Open(os.Getenv("ESY_FILE"))
	if err != nil {
		fatal(err)
	}
	pack, err := rulepack.Load(f)
	f.Close()
	if err != nil {
		fatal(err)
	}

	out, err := os.Create(os.Args[1])
	if err != nil {
		fatal(err)
	}
	w := bufio.NewWriter(out)
	enc := json.NewEncoder(w)

	id := firstSyntheticID
	emit := func(p plot) {
		p.ID = id
		id++
		if err := enc.Encode(p); err != nil {
			fatal(err)
		}
	}

	for _, r := range pack.Rules {
		leaves := leavesOf(r.Formula)
		for _, a := range assignments(r.Formula, true) {
			b := newBuilder(pack.Groups)
			for i, leaf := range leaves {
				if a[i] {
					b.satisfy(leaf.Expr)
				}
			}
			// A plot with no species cannot exist upstream: plot.cond's rows
			// are unique(obs$RELEVE_NR), so a header row without
			// observations would put header and matrix out of step.
			if len(b.records) == 0 {
				continue
			}
			emit(b.plot(r.Label()))
			for _, v := range b.belowVariants() {
				if len(v.records) == 0 {
					continue
				}
				emit(v.plot(r.Label() + "<"))
			}
		}
	}
	if err := w.Flush(); err != nil {
		fatal(err)
	}
	if err := out.Close(); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "synthesize: %d plots for %d rules\n",
		id-firstSyntheticID, len(pack.Rules))
}

// ---------------------------------------------------------------- formulas

func leavesOf(n rulepack.Node) []rulepack.Leaf {
	var out []rulepack.Leaf
	var walk func(rulepack.Node)
	walk = func(n rulepack.Node) {
		switch v := n.(type) {
		case rulepack.Leaf:
			out = append(out, v)
		case rulepack.And:
			walk(v.L)
			walk(v.R)
		case rulepack.Or:
			walk(v.L)
			walk(v.R)
		case rulepack.Not:
			walk(v.L)
			walk(v.R)
		}
	}
	walk(n)
	return out
}

// assignment maps a leaf's position in leavesOf order to the truth value the
// formula needs from it. Leaves the assignment does not mention are free.
type assignment map[int]bool

// assignments enumerates, at most maxAssignments of them, the ways the formula
// can take the given truth value. It is a plain expansion of the tree, not a
// solver: conflicting requirements on the same leaf drop the candidate.
func assignments(n rulepack.Node, want bool) []assignment {
	next := 0
	var walk func(rulepack.Node, bool) []assignment
	walk = func(n rulepack.Node, want bool) []assignment {
		switch v := n.(type) {
		case rulepack.Leaf:
			i := next
			next++
			return []assignment{{i: want}}
		case rulepack.And:
			return combine(v.L, v.R, want, true, walk)
		case rulepack.Or:
			return combine(v.L, v.R, want, false, walk)
		case rulepack.Not:
			// "L NOT R" is L AND NOT R.
			l := walk(v.L, want)
			r := walk(v.R, !want)
			if want {
				return merge(l, r)
			}
			return cap4(append(l, r...))
		}
		return nil
	}
	return walk(n, want)
}

// combine expands a binary node. conj says whether the node behaves as AND;
// an AND needs both sides when want is true and either side when it is false,
// and an OR is the mirror image.
func combine(l, r rulepack.Node, want, conj bool, walk func(rulepack.Node, bool) []assignment) []assignment {
	// The left subtree must be walked first either way: walk assigns leaf
	// indices in order and both branches have to see the same numbering.
	ls := walk(l, want)
	rs := walk(r, want)
	if conj == want {
		return merge(ls, rs)
	}
	return cap4(append(ls, rs...))
}

// merge builds the cross product of two assignment sets, dropping pairs that
// disagree about a leaf.
func merge(a, b []assignment) []assignment {
	var out []assignment
	for _, x := range a {
		for _, y := range b {
			m := assignment{}
			ok := true
			for k, v := range x {
				m[k] = v
			}
			for k, v := range y {
				if w, seen := m[k]; seen && w != v {
					ok = false
					break
				}
				m[k] = v
			}
			if ok {
				out = append(out, m)
			}
			if len(out) >= maxAssignments {
				return out
			}
		}
	}
	return out
}

func cap4(a []assignment) []assignment {
	if len(a) > maxAssignments {
		return a[:maxAssignments]
	}
	return a
}

// ------------------------------------------------------------------ builder

type builder struct {
	groups  map[string][]string
	records map[string]float64
	header  map[string]string
	// thresholds remembers, per group key, the numeric cut that made its
	// cover what it is, so belowVariants can step back under it.
	thresholds []threshold
}

type threshold struct {
	expr rulepack.Expr
	n    float64
}

func newBuilder(groups map[string][]string) *builder {
	h := map[string]string{}
	for _, f := range headerFields {
		h[f] = ""
	}
	h["Altitude..m."] = "0"
	h["DEG_LON"] = "0"
	h["DEG_LAT"] = "0"
	h["Ecoreg"] = "0"
	return &builder{groups: groups, records: map[string]float64{}, header: h}
}

func (b *builder) clone() *builder {
	c := newBuilder(b.groups)
	for k, v := range b.records {
		c.records[k] = v
	}
	for k, v := range b.header {
		c.header[k] = v
	}
	return c
}

func (b *builder) plot(rule string) plot {
	names := make([]string, 0, len(b.records))
	for n := range b.records {
		names = append(names, n)
	}
	sort.Strings(names)
	recs := make([]record, 0, len(names))
	for _, n := range names {
		recs = append(recs, record{Name: n, Cover: b.records[n]})
	}
	h := map[string]string{}
	for k, v := range b.header {
		h[k] = v
	}
	return plot{Records: recs, Header: h, Rule: rule}
}

// add raises a taxon's cover to at least c, never lowering an earlier demand.
func (b *builder) add(name string, c float64) {
	if name == "" {
		return
	}
	c = math.Min(math.Max(c, 1), 100)
	if b.records[name] < c {
		b.records[name] = c
	}
}

// members returns the taxa an atom resolves to: a group's members, or the
// atom's own name when it is a bare taxon.
func (b *builder) members(a rulepack.Atom) []string {
	key := a.Name
	if a.Qualifier != "" {
		key = a.Qualifier + " " + a.Name
	}
	if m, ok := b.groups[key]; ok {
		return m
	}
	if a.Qualifier != "" {
		return nil
	}
	return []string{a.Name}
}

// satisfy adds whatever the plot needs for one expression to hold. It is a
// best effort: upstream, not this function, decides whether the plot really
// satisfies the rule.
func (b *builder) satisfy(x rulepack.Expr) {
	if x.AlwaysFalse || len(x.Left.Atoms) == 0 {
		return
	}
	lead := x.Left.Atoms[0]

	if lead.Kind == "$$C" {
		b.header[lead.Name] = x.Right.Literal
		return
	}
	if lead.Kind == "$$N" {
		b.setNumericHeader(lead.Name, x.Op, x.Right.Literal)
		return
	}

	// A numeric right-hand side gives the measure a concrete target; anything
	// else (another group, "#T$", "#$$", or no right-hand side at all) is met
	// by pushing the left side to its maximum instead.
	target := 100.0
	if n, err := strconv.ParseFloat(strings.TrimPrefix(x.Right.Literal, "$"), 64); err == nil {
		target = n
		if x.Op == "GR" {
			target = n + 1
		}
		b.thresholds = append(b.thresholds, threshold{expr: x, n: n})
	}
	b.reach(lead, x.Left.Atoms, target)
}

// reach gives the atoms' members enough cover for the measure named by lead to
// reach target.
func (b *builder) reach(lead rulepack.Atom, atoms []rulepack.Atom, target float64) {
	var pool []string
	for _, a := range atoms {
		pool = append(pool, b.members(a)...)
	}
	if len(pool) == 0 {
		return
	}
	switch lead.Kind {
	case "###", "##D":
		// Species count: target members, each present at a low cover so the
		// count is what carries the condition.
		for i := 0; i < int(target)+1 && i < len(pool); i++ {
			b.add(pool[i], 5)
		}
	case "##Q":
		// Sum of square roots: a member at 100% contributes 10.
		for i := 0; i <= int(target/10) && i < len(pool); i++ {
			b.add(pool[i], 100)
		}
	case "#T$", "#$$":
		// Plot-wide totals: anything present raises them.
		b.add(pool[0], math.Min(target, 100))
	default:
		// #TC, ##C, #SC and a bare taxon all rise with a single member.
		b.add(pool[0], target)
	}
}

func (b *builder) setNumericHeader(field, op, literal string) {
	n, err := strconv.ParseFloat(literal, 64)
	if err != nil {
		return
	}
	if op == "GR" {
		n++
	}
	b.header[field] = strconv.FormatFloat(n, 'f', -1, 64)
}

// belowVariants returns one plot per numeric threshold the assignment used,
// each with that threshold's group pushed just under its cut. Upstream and
// habitatus must agree on those too, and they are where an off-by-one in a
// comparison shows up.
func (b *builder) belowVariants() []*builder {
	var out []*builder
	for _, th := range b.thresholds {
		c := b.clone()
		lead := th.expr.Left.Atoms[0]
		// Clear the group, then put it back just below the cut.
		for _, at := range th.expr.Left.Atoms {
			for _, m := range c.members(at) {
				delete(c.records, m)
			}
		}
		if th.n > 1 {
			c.reach(lead, th.expr.Left.Atoms, th.n-1)
		}
		out = append(out, c)
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "synthesize:", err)
	os.Exit(1)
}
