package taxa

import (
	"math"
	"testing"
)

func TestResolveTwoStages(t *testing.T) {
	backbone := map[string]string{"Abies pectinata": "Abies alba"}
	agg := map[string]string{"Abies alba": "Abies alba aggr."}
	got, steps := Resolve([]Record{{"Abies pectinata", 20}}, backbone, agg, nil)
	if len(got) != 1 || got[0].Name != "Abies alba aggr." {
		t.Fatalf("two-stage resolution failed: %+v", got)
	}
	if steps[0].AfterBackbone != "Abies alba" || steps[0].Final != "Abies alba aggr." {
		t.Errorf("step report wrong: %+v", steps[0])
	}
}

func TestResolveMergesCoversAfterAggregation(t *testing.T) {
	agg := map[string]string{
		"Empetrum nigrum":         "Empetrum nigrum aggr.",
		"Empetrum hermaphroditum": "Empetrum nigrum aggr.",
	}
	got, _ := Resolve([]Record{
		{"Empetrum nigrum", 3},
		{"Empetrum hermaphroditum", 4},
	}, nil, agg, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 merged record, got %d", len(got))
	}
	if math.Abs(got[0].Cover-6.88) > 1e-9 {
		t.Errorf("cover = %v, want 6.88 (Jennings-Fischer, not 7)", got[0].Cover)
	}
}

func TestResolveIsSinglePass(t *testing.T) {
	// Elytrigia species is both a source and a target. One pass only.
	agg := map[string]string{
		"Elymus pycnanthus": "Elytrigia species",
		"Elytrigia species": "Elymus species",
	}
	got, _ := Resolve([]Record{{"Elymus pycnanthus", 10}}, nil, agg, nil)
	if got[0].Name != "Elytrigia species" {
		t.Errorf("chain was followed: got %q, want %q", got[0].Name, "Elytrigia species")
	}
}

func TestResolveKeepsUnknownNames(t *testing.T) {
	got, steps := Resolve([]Record{{"Nonexistent name", 5}}, nil, nil, nil)
	if len(got) != 1 || got[0].Name != "Nonexistent name" {
		t.Fatalf("unknown names must pass through: %+v", got)
	}
	if !steps[0].Resolved {
		t.Error("nil known must mark every step resolved")
	}
}

func TestResolveAppliesPopulusCorrection(t *testing.T) {
	// Spec section 8: the file maps this poplar onto a grass. Corrected here.
	agg := map[string]string{
		"Populus x canadensis + P. nigra": "Polypogon monspeliensis x viridis",
	}
	got, _ := Resolve([]Record{{"Populus x canadensis + P. nigra", 10}}, nil, agg, nil)
	if got[0].Name != "Populus x canadensis" {
		t.Errorf("Populus correction not applied: %q", got[0].Name)
	}
}

func TestResolveOutputIsDeterministic(t *testing.T) {
	agg := map[string]string{"b": "z", "a": "z", "c": "y"}
	in := []Record{{"a", 1}, {"b", 2}, {"c", 3}}
	first, _ := Resolve(in, nil, agg, nil)
	for i := 0; i < 20; i++ {
		again, _ := Resolve(in, nil, agg, nil)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("non-deterministic output at %d: %+v vs %+v", j, first[j], again[j])
			}
		}
	}
}

// TestResolveKnownSetDeterminesResolution reproduces the plot from a later
// task: Fagus sylvatica is a member of a species group (known to the rule
// pack) but has no entry in section 1, while "Totally unknown plant" is not
// known at all. Only the latter must be reported unresolved.
func TestResolveKnownSetDeterminesResolution(t *testing.T) {
	known := map[string]bool{"Fagus sylvatica": true}
	got, steps := Resolve([]Record{
		{"Fagus sylvatica", 30},
		{"Totally unknown plant", 5},
	}, nil, nil, known)

	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d: %+v", len(got), got)
	}

	unresolvedCount := 0
	for _, s := range steps {
		if !s.Resolved {
			unresolvedCount++
			if s.Input != "Totally unknown plant" {
				t.Errorf("wrong name reported unresolved: %q", s.Input)
			}
		}
	}
	if unresolvedCount != 1 {
		t.Fatalf("want exactly 1 unresolved name, got %d: %+v", unresolvedCount, steps)
	}
}
