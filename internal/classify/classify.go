package classify

import (
	"fmt"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

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
}

// Service holds the loaded rule pack and the backbone tables.
type Service struct {
	pack      *rulepack.Pack
	backbones map[string]map[string]string
	versions  map[string]string
}

// NewService builds a service. backbones maps a backbone id to its translation
// table; the id "euro+med" is the identity and needs no table.
func NewService(pack *rulepack.Pack, backbones map[string]map[string]string, versions map[string]string) *Service {
	return &Service{pack: pack, backbones: backbones, versions: versions}
}

// Classify validates, resolves and evaluates one plot.
func (s *Service) Classify(req Request) (Response, error) {
	if len(req.Records) == 0 {
		return Response{}, fmt.Errorf("at least one taxon record is required")
	}
	for _, r := range req.Records {
		if r.Cover <= 0 || r.Cover > 100 {
			return Response{}, fmt.Errorf("cover for %q is %v, must be in (0, 100]", r.Name, r.Cover)
		}
	}
	if err := ValidateHeader(req.Header); err != nil {
		return Response{}, err
	}
	var table map[string]string
	if req.Backbone != "euro+med" {
		t, ok := s.backbones[req.Backbone]
		if !ok {
			return Response{}, fmt.Errorf("unknown backbone %q", req.Backbone)
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
	header := make(map[string]string, len(esy.KnownHeaderFields))
	for field := range esy.KnownHeaderFields {
		header[field] = req.Header[field]
	}

	env := esy.Env{Groups: s.pack.Groups}
	res := env.Evaluate(s.pack.Rules, esy.Plot{Records: resolved, Header: header})
	return Response{
		Result:     res.Winner,
		Matches:    res.Matches,
		Resolution: steps,
		Versions:   s.versions,
	}, nil
}
