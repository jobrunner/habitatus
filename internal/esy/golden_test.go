package esy_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// The golden master compares habitatus against the upstream R implementation
// ESy v1.2 run over the bundled Tuexen-Archiv data. The fixtures are produced
// by `make fixtures`, which drives the upstream code without modifying it; see
// spike/resy/README.md.

const goldenDir = "../../testdata/golden"

type goldenCase struct {
	ID      int `json:"id"`
	Records struct {
		Name  []string  `json:"name"`
		Cover []float64 `json:"cover"`
	} `json:"records"`
	Header map[string]string `json:"header"`
}

type goldenExpect struct {
	ID      int      `json:"id"`
	Winner  string   `json:"winner"`
	Matches []string `json:"matches"`
}

// intermediate is the part of one plot's intermediates.jsonl entry this test
// needs: the indices of the membership expressions upstream's logi1 holds
// TRUE for it, 1-based as R writes them. The same line also carries the
// non-zero entries of that plot's plot.cond row, which spike/resy/diagnose.go
// reads when a divergence has to be traced to a single condition.
type intermediate struct {
	ID       int   `json:"id"`
	ExprTrue []int `json:"expr_true"`
}

// loadPack parses the rule file the fixtures were generated from, after
// checking that it IS that file.
func loadPack(t *testing.T) *rulepack.Pack {
	t.Helper()
	esyFile := os.Getenv("ESY_FILE")
	if esyFile == "" {
		t.Skip("ESY_FILE not set")
	}
	requireFreshFixtures(t, esyFile)
	f, err := os.Open(esyFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pack, err := rulepack.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

// requireFreshFixtures fails early when the fixtures were generated from a
// different rule file or a different upstream commit than the one in play.
// Without this a stale fixture set produces thousands of mismatches that read
// like a port regression, which is the opposite of what rulepack.sha256 and
// upstream.commit are recorded for.
func requireFreshFixtures(t *testing.T, esyFile string) {
	t.Helper()
	const regen = "the fixtures are stale; regenerate them with `make fixtures`"

	var meta struct {
		UpstreamCommit string `json:"upstream_commit"`
		ESYFile        string `json:"esy_file"`
	}
	readJSON(t, "meta.json", &meta)

	if want, got := fixtureLine(t, "rulepack.sha256"), fileSHA256(t, esyFile); want != got {
		t.Fatalf("%s\nESY_FILE %s\n  has SHA-256 %s\n  fixtures were built from %s (%s)",
			regen, esyFile, got, want, meta.ESYFile)
	}
	if want, got := fixtureLine(t, "upstream.commit"), meta.UpstreamCommit; want != got {
		t.Fatalf("%s\nupstream.commit is %s but meta.json records %s", regen, want, got)
	}
}

// fixtureLine reads a fixture file holding a single value.
func fixtureLine(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(goldenDir, name))
	if err != nil {
		t.Fatalf("%s: %v (run `make fixtures`)", name, err)
	}
	return strings.TrimSpace(string(b))
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func requireFixtures(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(goldenDir, "cases.jsonl")); err != nil {
		t.Skip("no fixtures; run `make fixtures`")
	}
}

// scanJSONL reads a JSONL file line by line into T.
func scanJSONL[T any](t *testing.T, name string, fn func(T)) {
	t.Helper()
	f, err := os.Open(filepath.Join(goldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for s.Scan() {
		var v T
		if err := json.Unmarshal(s.Bytes(), &v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fn(v)
	}
	if err := s.Err(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// readJSON reads one whole JSON file from the fixture directory.
func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(goldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// plotOf turns one fixture case into an evaluable plot, applying the same
// two-stage name resolution the service does.
func plotOf(c goldenCase, pack *rulepack.Pack) esy.Plot {
	recs := make([]taxa.Record, len(c.Records.Name))
	for i := range c.Records.Name {
		recs[i] = taxa.Record{Name: c.Records.Name[i], Cover: c.Records.Cover[i]}
	}
	resolved, _ := taxa.Resolve(recs, nil, pack.Aggregation, nil)
	return esy.Plot{Records: resolved, Header: c.Header}
}

func TestGoldenMaster(t *testing.T) {
	requireFixtures(t)
	pack := loadPack(t)
	env := esy.Env{Groups: pack.Groups}

	expected := map[int]goldenExpect{}
	scanJSONL(t, "expected.jsonl", func(e goldenExpect) { expected[e.ID] = e })
	if len(expected) == 0 {
		t.Fatal("expected.jsonl is empty")
	}

	var total, winnerBad, matchBad int
	scanJSONL(t, "cases.jsonl", func(c goldenCase) {
		want, ok := expected[c.ID]
		if !ok {
			t.Fatalf("plot %d has no expectation", c.ID)
		}
		got := env.Evaluate(pack.Rules, plotOf(c, pack))
		total++

		if wantWinner := bareCode(want.Winner); got.Winner != wantWinner {
			winnerBad++
			if winnerBad <= 20 {
				t.Errorf("plot %d: winner %q, want %q", c.ID, got.Winner, wantWinner)
			}
		}
		gotLabels := labelsOf(got.Matches)
		if !sameSet(gotLabels, want.Matches) {
			matchBad++
			if matchBad <= 20 {
				t.Errorf("plot %d: matches %v, want %v\n  only ours: %v\n  only R:    %v",
					c.ID, gotLabels, want.Matches,
					missing(gotLabels, want.Matches), missing(want.Matches, gotLabels))
			}
		}
	})

	t.Logf("%d plots compared, %d winner mismatches, %d match-set mismatches",
		total, winnerBad, matchBad)
	if winnerBad > 0 || matchBad > 0 {
		t.Errorf("golden master: %d of %d plots differ in the winner, %d in the match set",
			winnerBad, total, matchBad)
	}
}

// TestGoldenExpressions compares the layer beneath the classification: every
// membership expression of every rule, plot by plot, against the truth values
// upstream recorded in logi1. Agreeing on the winner while disagreeing on an
// expression is possible -- two errors can cancel inside one formula -- and
// this is where that would show. It runs over the plots in
// intermediates.jsonl -- by default an even stride of 300 across the archive
// and the synthetic plots; see HABITATUS_INTERMEDIATE_PLOTS.
//
// The two sides are matched positionally: upstream extracts the "<...>"
// bodies of a formula left to right and so does the Go parser, and none of
// upstream's rewrites changes how many there are.
func TestGoldenExpressions(t *testing.T) {
	requireFixtures(t)
	pack := loadPack(t)
	env := esy.Env{Groups: pack.Groups}

	var ruleExprs []struct {
		Label string `json:"label"`
		Exprs []int  `json:"exprs"`
	}
	readJSON(t, "rule-exprs.json", &ruleExprs)
	// Matched by position, not by label: twelve rules share the label "T3M".
	if len(ruleExprs) != len(pack.Rules) {
		t.Fatalf("%d rules in R, %d here", len(ruleExprs), len(pack.Rules))
	}
	for i, r := range pack.Rules {
		if ruleExprs[i].Label != r.Label() {
			t.Fatalf("rule %d is %q in R and %q here", i, ruleExprs[i].Label, r.Label())
		}
	}

	cases := map[int]goldenCase{}
	scanJSONL(t, "cases.jsonl", func(c goldenCase) { cases[c.ID] = c })

	var plots, compared, bad int
	scanJSONL(t, "intermediates.jsonl", func(im intermediate) {
		c, ok := cases[im.ID]
		if !ok {
			t.Fatalf("intermediate for unknown plot %d", im.ID)
		}
		plots++
		rTrue := map[int]bool{}
		for _, i := range im.ExprTrue {
			rTrue[i] = true
		}
		p := plotOf(c, pack)
		for ri, rule := range pack.Rules {
			idx := ruleExprs[ri].Exprs
			leaves := leavesOf(rule.Formula)
			if len(leaves) != len(idx) {
				t.Fatalf("rule %s: %d expressions here, %d in R", rule.Label(), len(leaves), len(idx))
			}
			for i, lf := range leaves {
				got, left, right := env.EvalExpr(lf.Expr, p)
				compared++
				if got.IsTrue() != rTrue[idx[i]] {
					bad++
					if bad <= 20 {
						t.Errorf("plot %d rule %s expression %d %q: %v (%g, %g), R says %v",
							im.ID, rule.Label(), i, lf.Raw, got, left, right, rTrue[idx[i]])
					}
				}
			}
		}
	})
	t.Logf("%d plots, %d expression evaluations compared, %d differ", plots, compared, bad)
	if bad > 0 {
		t.Errorf("%d of %d expression evaluations differ from upstream", bad, compared)
	}
}

// leavesOf returns a formula's membership expressions in file order.
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

// TestGoldenRuleCoverage reports which rules never fired over the whole fixture
// set. A rule that never fires is either unreachable with this data or a hint
// that the synthetic generator missed it; it is reported, not failed, because
// the upstream run is the authority on what is reachable.
func TestGoldenRuleCoverage(t *testing.T) {
	requireFixtures(t)
	pack := loadPack(t)

	fired := map[string]bool{}
	scanJSONL(t, "expected.jsonl", func(e goldenExpect) {
		for _, m := range e.Matches {
			fired[m] = true
		}
	})

	var unreachable, silent []string
	for _, r := range pack.Rules {
		if fired[r.Label()] {
			continue
		}
		if needsAlwaysFalse(r.Formula) {
			unreachable = append(unreachable, r.Label())
		} else {
			silent = append(silent, r.Label())
		}
	}
	sort.Strings(unreachable)
	sort.Strings(silent)
	t.Logf("%d of %d rules fired at least once", len(pack.Rules)-len(unreachable)-len(silent), len(pack.Rules))
	t.Logf("%d can never fire: every way of satisfying them needs an expression upstream forces to FALSE: %s",
		len(unreachable), strings.Join(unreachable, " "))
	t.Logf("%d are reachable in principle but no fixture triggers them: %s",
		len(silent), strings.Join(silent, " "))
}

// needsAlwaysFalse reports whether every way of satisfying the formula needs
// an expression upstream forces to FALSE (see rulepack.ParseExpr). Such a
// rule cannot fire for any plot, so no fixture can cover it.
func needsAlwaysFalse(n rulepack.Node) bool {
	switch v := n.(type) {
	case rulepack.Leaf:
		return v.Expr.AlwaysFalse
	case rulepack.And:
		return needsAlwaysFalse(v.L) || needsAlwaysFalse(v.R)
	case rulepack.Or:
		return needsAlwaysFalse(v.L) && needsAlwaysFalse(v.R)
	case rulepack.Not:
		// "L NOT R" needs L; R only has to be false, which an
		// always-false expression satisfies for free.
		return needsAlwaysFalse(v.L)
	}
	return false
}

// bareCode drops the variant marker from a rule label. "R1Q" and "R1Q!" are
// two definitions of the same habitat at different priorities, and
// esy.Result.Winner reports the habitat, not which of its definitions won.
// The match set is still compared with the marker (see labelsOf), so which
// definition fired is not lost. "?" and "+" pass through unchanged.
func bareCode(label string) string { return strings.TrimRight(label, "!") }

func labelsOf(ms []esy.Match) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Code+m.Variant)
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	return len(missing(a, b)) == 0 && len(missing(b, a)) == 0
}

// missing returns the elements of a that are not in b.
func missing(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	var out []string
	for _, x := range a {
		if !in[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
