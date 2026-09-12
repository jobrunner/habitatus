package classify

import (
	"os"
	"testing"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
)

// TestReachabilityRealFile pins the headline figure the review verified
// independently: against the real 2025-10-03 rule file in esy.Faithful,
// exactly 100 of the 312 rules are structurally unreachable (every
// satisfying assignment needs an "#NN Group" expression v1.2 forces to
// FALSE), and the reachable set correctly includes some rules while
// excluding whole known-defective blocks. In esy.Repaired the same 100 are
// reachable again — that number collapsing to zero is the sharpest evidence
// that the mode does what it claims. Nothing else in the committed test suite exercises reach()/
// ruleReachable() on the real, deeply nested And/Or/Not formulas — the unit
// tests cover single-leaf and simple two-node cases only — so this is the
// one place a regression in the tree walk over real-world formula shapes
// would be caught.
func TestReachabilityRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pack, err := rulepack.Load(f)
	if err != nil {
		t.Fatal(err)
	}

	if len(pack.Rules) != 312 {
		t.Fatalf("got %d rules, want 312", len(pack.Rules))
	}

	svc := NewService(pack, nil, nil, esy.Faithful)

	if len(svc.unreachable) != 100 {
		t.Errorf("faithful: got %d unreachable labels, want 100: %v", len(svc.unreachable), svc.unreachable)
	}

	// In repaired mode nothing pins an expression to FALSE, so no rule is
	// structurally unreachable at all.
	repaired := NewService(pack, nil, nil, esy.Repaired)
	if len(repaired.unreachable) != 0 {
		t.Errorf("repaired: got %d unreachable labels, want 0: %v",
			len(repaired.unreachable), repaired.unreachable)
	}

	// The upstream defect that forces "#NN Group" expressions to FALSE
	// kills the whole grassland block (R11-R57) and the whole
	// sparsely-vegetated block (U21-U72) — every rule in each range must
	// be unreachable.
	for _, label := range []string{
		"R11", "R12", "R13", "R14", "R16", "R17", "R18", "R19",
		"R1A", "R1B", "R1B!", "R1C", "R1D", "R1E", "R1F", "R1G", "R1H",
		"R1K", "R1M", "R1P", "R1Q", "R1R",
		"R21", "R22", "R23", "R23!", "R24", "R24!",
		"R31", "R32", "R33", "R34", "R35", "R36", "R37",
		"R41", "R41!", "R42", "R43", "R44", "R45",
		"R51", "R52", "R57",
		"U21", "U22", "U23", "U24", "U25", "U26", "U27", "U28", "U29",
		"U2A",
		"U31", "U32", "U33", "U34", "U35", "U36", "U37", "U38", "U3C", "U3D",
		"U52",
		"U61", "U62",
		"U71", "U71!", "U72",
	} {
		if !svc.unreachable[label] {
			t.Errorf("faithful: %q is in the grassland/sparsely-vegetated block killed by the upstream defect, must be unreachable", label)
		}
	}

	// A handful of rules known to be reachable must NOT be in the set.
	for _, label := range []string{"T17", "T1H", "N1A"} {
		if svc.unreachable[label] {
			t.Errorf("%q is a known-reachable rule, must not be listed as unreachable", label)
		}
	}
}
