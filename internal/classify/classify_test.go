package classify

import (
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

const tinyPack = `SECTION 1: Species aggregation
Empetrum nigrum aggr.                                     -  0
     Empetrum nigrum                                         0
SECTION 1: End
SECTION 2: Species groups
### Trees
     Fagus sylvatica
SECTION 2: End
SECTION 3: Group definitions

4          T1H  Broadleaved deciduous plantation
<#TC Trees GR 5>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func newTestService(t *testing.T) *Service {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(pack, nil, map[string]string{"rulepack": "test"})
}

func TestClassifyHappyPath(t *testing.T) {
	s := newTestService(t)
	got, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "euro+med",
		Header:   validHeader(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != "T1H" {
		t.Errorf("result = %q, want T1H", got.Result)
	}
	if len(got.Resolution) != 1 {
		t.Errorf("resolution report missing: %+v", got.Resolution)
	}
}

func TestClassifyRejectsBadCover(t *testing.T) {
	s := newTestService(t)
	for _, c := range []float64{0, -1, 101} {
		_, err := s.Classify(Request{
			Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: c}},
			Backbone: "euro+med",
			Header:   validHeader(),
		})
		if err == nil {
			t.Errorf("cover %v must be rejected", c)
		}
	}
}

func TestClassifyRejectsUnknownBackbone(t *testing.T) {
	s := newTestService(t)
	_, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "wcvp",
		Header:   validHeader(),
	})
	if err == nil {
		t.Error("unknown backbone must be rejected")
	}
}

func TestClassifyAcceptsUnknownTaxonName(t *testing.T) {
	s := newTestService(t)
	got, err := s.Classify(Request{
		Records: []taxa.Record{
			{Name: "Fagus sylvatica", Cover: 30},
			{Name: "Totally unknown plant", Cover: 5},
		},
		Backbone: "euro+med",
		Header:   validHeader(),
	})
	if err != nil {
		t.Fatalf("unknown taxon names must not be rejected: %v", err)
	}
	var unresolved int
	for _, s := range got.Resolution {
		if !s.Resolved {
			unresolved++
		}
	}
	if unresolved != 1 {
		t.Errorf("want 1 unresolved name reported, got %d", unresolved)
	}
}

func TestClassifyFillsMissingDatasetAsEmptyString(t *testing.T) {
	s := newTestService(t)
	h := validHeader()
	delete(h, "Dataset")
	if _, ok := h["Dataset"]; ok {
		t.Fatal("test setup: Dataset must be absent")
	}
	_, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "euro+med",
		Header:   h,
	})
	if err != nil {
		t.Fatalf("omitted optional Dataset must not be rejected: %v", err)
	}
}

// Spec §6 requires the response to name the backbone table it used. Only
// Classify knows which one the request selected, so it adds it per response —
// without mutating the service-wide map, which every response shares.
func TestClassifyVersionsCarryTheBackbone(t *testing.T) {
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]string{"rulepack": "EUNIS-ESy-2025-10-03.txt"}
	tables := map[string]map[string]string{"germansl": {"Buche": "Fagus sylvatica"}}
	s := NewService(pack, tables, base)

	for _, backbone := range []string{"euro+med", "germansl"} {
		got, err := s.Classify(Request{
			Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
			Backbone: backbone,
			Header:   validHeader(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Versions["backbone"] != backbone {
			t.Errorf("backbone %q: versions[backbone] = %q", backbone, got.Versions["backbone"])
		}
		if got.Versions["rulepack"] != base["rulepack"] {
			t.Errorf("backbone %q: rulepack version lost: %q", backbone, got.Versions["rulepack"])
		}
	}
	if _, leaked := base["backbone"]; leaked {
		t.Errorf("the service-wide versions map was mutated: %v", base)
	}
}
