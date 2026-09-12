//go:build ignore

// Command diagnose compares habitatus against the golden-master fixtures and
// reports where they diverge, at three levels of detail:
//
//	go run spike/resy/diagnose.go tally            rules over- and under-fired
//	go run spike/resy/diagnose.go rule <label>     plots where that rule differs
//	go run spike/resy/diagnose.go plot <id> <rule> per-expression truth values,
//	                                               ours next to R's
//
// It reads $ESY_FILE and, by default, the faithful fixture set in
// testdata/golden/. Set HABITATUS_MODE=repaired to diagnose the other golden
// master instead: that switches both the fixture directory and the evaluation
// semantics, which have to move together. This is a debugging aid for the
// golden master, not part of the build.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// dir and evalMode are chosen together: a fixture set is only comparable
// against the semantics its oracle implemented.
var dir, evalMode = fixtures()

func fixtures() (string, esy.Mode) {
	name := os.Getenv("HABITATUS_MODE")
	if name == "" {
		name = esy.Faithful.String()
	}
	mode, err := esy.ParseMode(name)
	must(err)
	if mode == esy.Repaired {
		return "testdata/golden-repaired", mode
	}
	return "testdata/golden", mode
}

type kase struct {
	ID      int `json:"id"`
	Records struct {
		Name  []string  `json:"name"`
		Cover []float64 `json:"cover"`
	} `json:"records"`
	Header map[string]string `json:"header"`
}

type expect struct {
	ID      int      `json:"id"`
	Winner  string   `json:"winner"`
	Matches []string `json:"matches"`
}

type ruleExprs struct {
	Label    string `json:"label"`
	Short    string `json:"short"`
	Priority string `json:"priority"`
	Exprs    []int  `json:"exprs"`
}

type intermediate struct {
	ID        int       `json:"id"`
	CondIndex []int     `json:"cond_index"`
	CondValue []float64 `json:"cond_value"`
	ExprTrue  []int     `json:"expr_true"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: diagnose tally | rule <label> | plot <id> <label>")
		os.Exit(2)
	}
	pack := load()
	env := esy.Env{Groups: pack.Groups, Mode: evalMode}
	fmt.Fprintf(os.Stderr, "diagnosing %s against %s\n", evalMode, dir)
	exp := map[int]expect{}
	each("expected.jsonl", func(b []byte) {
		var e expect
		must(json.Unmarshal(b, &e))
		exp[e.ID] = e
	})

	switch os.Args[1] {
	case "tally":
		tally(env, pack, exp)
	case "rule":
		listRule(env, pack, exp, os.Args[2])
	case "plot":
		id, _ := strconv.Atoi(os.Args[2])
		plotDetail(env, pack, id, os.Args[3])
	default:
		fmt.Println("unknown mode")
		os.Exit(2)
	}
}

func tally(env esy.Env, pack *rulepack.Pack, exp map[int]expect) {
	extra := map[string]int{}
	missing := map[string]int{}
	ex := map[string][]int{}
	limit := 0
	if len(os.Args) > 2 {
		limit, _ = strconv.Atoi(os.Args[2])
	}
	n, bad := 0, 0
	each("cases.jsonl", func(b []byte) {
		if limit > 0 && n >= limit {
			return
		}
		var c kase
		must(json.Unmarshal(b, &c))
		n++
		got := set(labels(env.Evaluate(pack.Rules, plotOf(c, pack)).Matches))
		want := set(exp[c.ID].Matches)
		diff := false
		note := func(k string) {
			if len(ex[k]) < 3 {
				ex[k] = append(ex[k], c.ID)
			}
			diff = true
		}
		for k := range got {
			if !want[k] {
				extra[k]++
				note(k)
			}
		}
		for k := range want {
			if !got[k] {
				missing[k]++
				note(k)
			}
		}
		if diff {
			bad++
		}
	})
	fmt.Printf("%d plots, %d with a differing match set\n", n, bad)
	fmt.Println("\nfired by habitatus but not by R:")
	printCounts(extra, ex)
	fmt.Println("\nfired by R but not by habitatus:")
	printCounts(missing, ex)
}

func listRule(env esy.Env, pack *rulepack.Pack, exp map[int]expect, label string) {
	shown := 0
	each("cases.jsonl", func(b []byte) {
		if shown >= 10 {
			return
		}
		var c kase
		must(json.Unmarshal(b, &c))
		got := set(labels(env.Evaluate(pack.Rules, plotOf(c, pack)).Matches))
		want := set(exp[c.ID].Matches)
		if got[label] != want[label] {
			fmt.Printf("plot %d: ours=%v R=%v\n", c.ID, got[label], want[label])
			shown++
		}
	})
}

// plotDetail prints, for one plot and one rule, every membership expression of
// that rule with habitatus's truth value and both operand values, next to the
// truth value R recorded in logi1. Requires the plot to be in
// intermediates.jsonl (see HABITATUS_INTERMEDIATE_PLOTS).
func plotDetail(env esy.Env, pack *rulepack.Pack, id int, label string) {
	var c kase
	each("cases.jsonl", func(b []byte) {
		var k kase
		must(json.Unmarshal(b, &k))
		if k.ID == id {
			c = k
		}
	})
	if c.ID == 0 {
		fmt.Println("plot not found")
		return
	}
	var im intermediate
	each("intermediates.jsonl", func(b []byte) {
		var v intermediate
		must(json.Unmarshal(b, &v))
		if v.ID == id {
			im = v
		}
	})
	if im.ID == 0 {
		fmt.Printf("plot %d not in intermediates.jsonl; regenerate with HABITATUS_INTERMEDIATE_PLOTS=%d\n", id, id)
		return
	}
	rtrue := map[int]bool{}
	for _, i := range im.ExprTrue {
		rtrue[i] = true
	}
	var exprs []string
	readJSON("expressions.json", &exprs)
	var res []ruleExprs
	readJSON("rule-exprs.json", &res)

	var idx []int
	for _, r := range res {
		if r.Label == label {
			idx = r.Exprs
		}
	}
	var rule *rulepack.Rule
	for i := range pack.Rules {
		if pack.Rules[i].Label() == label {
			rule = &pack.Rules[i]
		}
	}
	if rule == nil || idx == nil {
		fmt.Println("rule not found on one of the two sides")
		return
	}
	p := plotOf(c, pack)
	leaves := leavesOf(rule.Formula)
	fmt.Printf("plot %d, rule %s: %d leaves here, %d expressions in R\n", id, label, len(leaves), len(idx))
	for i, lf := range leaves {
		t, l, r := env.EvalExpr(lf.Expr, p)
		rv, rs := "?", ""
		if i < len(idx) {
			rv = fmt.Sprint(rtrue[idx[i]])
			rs = exprs[idx[i]-1]
		}
		mark := " "
		if rv != strings.ToLower(t.String()) && !(rv == "true" && t == esy.True) && !(rv == "false" && t == esy.False) {
			mark = "*"
		}
		fmt.Printf("%s [%2d] ours=%-5s (%g vs %g)  R=%-5s\n      ours: %s\n      R:    %s\n",
			mark, i, t, l, r, rv, lf.Raw, rs)
	}
}

func leavesOf(n rulepack.Node) []rulepack.Leaf {
	switch v := n.(type) {
	case rulepack.Leaf:
		return []rulepack.Leaf{v}
	case rulepack.And:
		return append(leavesOf(v.L), leavesOf(v.R)...)
	case rulepack.Or:
		return append(leavesOf(v.L), leavesOf(v.R)...)
	case rulepack.Not:
		return append(leavesOf(v.L), leavesOf(v.R)...)
	}
	return nil
}

func plotOf(c kase, pack *rulepack.Pack) esy.Plot {
	recs := make([]taxa.Record, len(c.Records.Name))
	for i := range c.Records.Name {
		recs[i] = taxa.Record{Name: c.Records.Name[i], Cover: c.Records.Cover[i]}
	}
	resolved, _ := taxa.Resolve(recs, nil, pack.Aggregation, nil)
	return esy.Plot{Records: resolved, Header: c.Header}
}

func labels(ms []esy.Match) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Code+m.Variant)
	}
	return out
}

func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func printCounts(m map[string]int, ex map[string][]int) {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	for _, e := range s {
		fmt.Printf("  %-8s %5d   e.g. %v\n", e.k, e.v, ex[e.k])
	}
}

func load() *rulepack.Pack {
	f, err := os.Open(os.Getenv("ESY_FILE"))
	must(err)
	defer f.Close()
	p, err := rulepack.Load(f)
	must(err)
	return p
}

func each(name string, fn func([]byte)) {
	f, err := os.Open(dir + "/" + name)
	must(err)
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for s.Scan() {
		fn(s.Bytes())
	}
	must(s.Err())
}

func readJSON(name string, v any) {
	b, err := os.ReadFile(dir + "/" + name)
	must(err)
	must(json.Unmarshal(b, v))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
