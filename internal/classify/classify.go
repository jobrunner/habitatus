package classify

import (
	"math"
	"sort"
	"sync"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// DefaultBackbone is the nomenclature the ESy rule file itself is written in:
// its section 1 is the Euro+Med translation table, so a request naming this
// backbone needs no extra table.
const DefaultBackbone = "euro+med"

// Request is one classification request.
type Request struct {
	Records  []taxa.Record
	Backbone string
	Header   map[string]string
}

// Response is the outcome plus everything needed to understand it.
type Response struct {
	Result     string
	Matches    []esy.Match
	Resolution []taxa.Step
	Versions   map[string]string
	// TruncatedAt10 reports whether upstream would have dropped matches:
	// it stores only the first ten hits per plot. We return all of them,
	// so a caller comparing against published ESy output needs this flag
	// to know why the lists differ.
	TruncatedAt10 bool
}

// truncatedAt10 reports whether upstream would have dropped matches: it
// stores only the first ten hits per plot. We return all of them and set
// this flag so the golden master stays comparable.
func truncatedAt10[T any](ms []T) bool { return len(ms) > 10 }

// Stats are the two operational figures that carry ecological meaning: how
// often the answer is "?" or "+", and which rules never fire. Both surface
// errors no test finds — a client that fills one header field wrongly shows
// up here.
//
// NeverFired and Unreachable are reported separately on purpose, and
// Unreachable depends on the evaluation mode. In esy.Faithful, 100 of the 312
// rules in the real rule file can never fire at all: v1.2 forces every
// "#NN Group" expression to FALSE (see rulepack.Expr.AlwaysFalse), and every
// satisfying assignment of those 100 rules needs one to be true. In
// esy.Repaired those expressions carry a real truth value again and the set
// collapses. Either way it is a static property of the rule pack and the
// mode, computed once at construction and exposed as Unreachable. NeverFired
// excludes it, so it stays the operationally interesting signal: a reachable
// rule that has not fired in any request so far.
type Stats struct {
	Total       int
	Question    int
	Plus        int
	Unreachable []string
	NeverFired  []string
}

// Service holds the loaded rule pack and the backbone tables.
type Service struct {
	pack      *rulepack.Pack
	backbones map[string]map[string]string
	versions  map[string]string
	mode      esy.Mode

	// allLabels and unreachable are fixed at construction time: allLabels
	// is every rule label in file order, unreachable is the subset that
	// can never fire, a static property of the formulas (see Stats).
	allLabels   []string
	unreachable map[string]bool

	mu       sync.Mutex
	total    int
	question int
	plus     int
	fired    map[string]bool
}

// NewService builds a service. backbones maps a backbone id to its translation
// table; the id DefaultBackbone is the identity and needs no table. mode selects
// the evaluation semantics (see esy.Mode) and is reported in the versions map
// of every response: a result whose semantics the caller cannot identify is
// not interpretable.
func NewService(pack *rulepack.Pack, backbones map[string]map[string]string, versions map[string]string, mode esy.Mode) *Service {
	// Several distinct rules can share the same code with no variant
	// marker to tell them apart — a rule-file data quirk (e.g. "T3M"
	// occurs 12 times in the 2025-10-03 file), not a habitatus artefact.
	// A label is unreachable only when EVERY rule carrying it is
	// unreachable; one reachable definition makes the whole label
	// reachable, since Match and the fired set are keyed on the label,
	// not on which physical definition fired.
	seen := map[string]bool{}
	var allLabels []string
	reachableLabel := map[string]bool{}
	for _, r := range pack.Rules {
		label := r.Label()
		if !seen[label] {
			seen[label] = true
			allLabels = append(allLabels, label)
		}
		if ruleReachable(r.Formula, mode) {
			reachableLabel[label] = true
		}
	}
	unreachable := map[string]bool{}
	for _, label := range allLabels {
		if !reachableLabel[label] {
			unreachable[label] = true
		}
	}
	// The mode goes into the shared versions map, so every response carries
	// it without the request path having to remember to add it.
	withMode := make(map[string]string, len(versions)+1)
	for k, v := range versions {
		withMode[k] = v
	}
	withMode["mode"] = mode.String()
	return &Service{
		pack:        pack,
		backbones:   backbones,
		versions:    withMode,
		mode:        mode,
		allLabels:   allLabels,
		unreachable: unreachable,
		fired:       map[string]bool{},
	}
}

// ruleReachable reports whether a rule's formula can ever evaluate TRUE. It
// treats every leaf that is not pinned to FALSE by the mode as an independent
// free variable and asks whether TRUE is reachable at the root — a structural
// property of the formula's shape, not of any plot.
//
// In esy.Faithful an AlwaysFalse leaf (an "#NN Group" expression, see
// rulepack.Expr.AlwaysFalse) can only ever be FALSE, so a rule whose every
// path to TRUE runs through one is unreachable. In esy.Repaired the same leaf
// has a real truth value and is a free variable like any other.
func ruleReachable(n rulepack.Node, mode esy.Mode) bool {
	reachTrue, _ := reach(n, mode)
	return reachTrue
}

// reach returns whether TRUE and whether FALSE are each reachable at n for
// some assignment of its free leaves.
func reach(n rulepack.Node, mode esy.Mode) (reachTrue, reachFalse bool) {
	switch v := n.(type) {
	case rulepack.Leaf:
		if v.Expr.AlwaysFalse && mode == esy.Faithful {
			return false, true
		}
		return true, true
	case rulepack.And:
		lt, lf := reach(v.L, mode)
		rt, rf := reach(v.R, mode)
		return lt && rt, lf || rf
	case rulepack.Or:
		lt, lf := reach(v.L, mode)
		rt, rf := reach(v.R, mode)
		return lt || rt, lf && rf
	case rulepack.Not:
		// Not is "L AND NOT R".
		lt, lf := reach(v.L, mode)
		rt, rf := reach(v.R, mode)
		return lt && rf, lf || rt
	}
	return false, true
}

// Classify validates, resolves and evaluates one plot.
func (s *Service) Classify(req Request) (Response, error) {
	if len(req.Records) == 0 {
		return Response{}, invalidf("at least one taxon record is required")
	}
	for _, r := range req.Records {
		if math.IsNaN(r.Cover) || math.IsInf(r.Cover, 0) {
			return Response{}, invalidf("cover for %q is %v, must be a finite number", r.Name, r.Cover)
		}
		if r.Cover <= 0 || r.Cover > 100 {
			return Response{}, invalidf("cover for %q is %v, must be in (0, 100]", r.Name, r.Cover)
		}
	}
	if err := ValidateHeader(req.Header); err != nil {
		return Response{}, err
	}
	var table map[string]string
	if req.Backbone != DefaultBackbone {
		t, ok := s.backbones[req.Backbone]
		if !ok {
			return Response{}, invalidf("unknown backbone %q", req.Backbone)
		}
		table = t
	}
	resolved, steps := taxa.Resolve(req.Records, table, s.pack.Aggregation, s.pack.KnownTaxa)

	// Dataset is the one optional header field. Hand the evaluator all eight
	// header fields always, so an omitted Dataset is an explicit empty
	// string rather than an absent key — see esy.compareCategorical, which
	// treats an absent key among the known fields the same as an empty
	// value (both FALSE), but only an explicit total mapping here removes
	// any chance of a later refactor turning "omitted" into the
	// unknown-field TRUE branch.
	fields := esy.HeaderFields()
	header := make(map[string]string, len(fields))
	for _, field := range fields {
		header[field] = req.Header[field]
	}

	env := esy.Env{Groups: s.pack.Groups, Mode: s.mode}
	res := env.Evaluate(s.pack.Rules, esy.Plot{Records: resolved, Header: header})
	s.record(res)
	return Response{
		Result:        res.Winner,
		Matches:       res.Matches,
		Resolution:    steps,
		Versions:      s.versionsWith(req.Backbone),
		TruncatedAt10: truncatedAt10(res.Matches),
	}, nil
}

// versionsWith returns the service-wide versions plus the backbone table this
// request resolved to, which spec §6 requires the response to name. The
// service map is shared by every response and must not be written to, so the
// per-request entry goes into a copy.
func (s *Service) versionsWith(backbone string) map[string]string {
	out := make(map[string]string, len(s.versions)+1)
	for k, v := range s.versions {
		out[k] = v
	}
	out["backbone"] = backbone
	return out
}

// record updates the operational counters and the set of rule labels seen
// to fire, for Stats.
func (s *Service) record(res esy.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	switch res.Winner {
	case "?":
		s.question++
	case "+":
		s.plus++
	}
	for _, m := range res.Matches {
		s.fired[m.Code+m.Variant] = true
	}
}

// Stats returns the operational figures accumulated over every call to
// Classify so far.
func (s *Service) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	unreachable := make([]string, 0, len(s.unreachable))
	for label := range s.unreachable {
		unreachable = append(unreachable, label)
	}
	sort.Strings(unreachable)

	neverFired := make([]string, 0, len(s.allLabels))
	for _, label := range s.allLabels {
		if s.unreachable[label] || s.fired[label] {
			continue
		}
		neverFired = append(neverFired, label)
	}
	sort.Strings(neverFired)

	return Stats{
		Total:       s.total,
		Question:    s.question,
		Plus:        s.plus,
		Unreachable: unreachable,
		NeverFired:  neverFired,
	}
}
