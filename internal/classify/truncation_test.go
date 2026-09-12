package classify

import (
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

func TestResponseMarksWhereUpstreamWouldTruncate(t *testing.T) {
	if !truncatedAt10(make([]int, 11)) {
		t.Error("11 matches must be marked as truncated")
	}
	if truncatedAt10(make([]int, 10)) {
		t.Error("exactly 10 matches is not truncation")
	}
}

func TestClassifySetsTruncatedAt10OnResponse(t *testing.T) {
	s := newTestService(t)
	got, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "euro+med",
		Header:   validHeader(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TruncatedAt10 {
		t.Errorf("one match must not be reported as truncated")
	}
}

// statsPack has three rules: one that can never fire (its only expression is
// an "#NN Group" form, which upstream always forces to FALSE), one that is
// reachable but that no request in this test triggers, and one that fires.
const statsPack = `SECTION 1: Species aggregation
SECTION 1: End
SECTION 2: Species groups
### Trees
     Fagus sylvatica
SECTION 2: End
SECTION 3: Group definitions

4          T1H  Broadleaved deciduous plantation
<#TC Trees GR 5>
4          T1J  Unreachable stub
<#02 Trees>
4          T1K  Reachable but never triggered here
<#TC Trees GR 99>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func newStatsTestService(t *testing.T) *Service {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(statsPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(pack, nil, map[string]string{"rulepack": "test"})
}

func TestStatsSeparatesUnreachableFromNeverFired(t *testing.T) {
	s := newStatsTestService(t)
	if _, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "euro+med",
		Header:   validHeader(),
	}); err != nil {
		t.Fatal(err)
	}

	st := s.Stats()
	if st.Total != 1 {
		t.Errorf("Total = %d, want 1", st.Total)
	}
	if len(st.Unreachable) != 1 || st.Unreachable[0] != "T1J" {
		t.Errorf("Unreachable = %v, want [T1J]", st.Unreachable)
	}
	if len(st.NeverFired) != 1 || st.NeverFired[0] != "T1K" {
		t.Errorf("NeverFired = %v, want [T1K] (T1H fired, T1J is unreachable and must be excluded)", st.NeverFired)
	}
}

// duplicateLabelPack reproduces a real rule-file quirk: two distinct rules
// (different formulas) sharing the same code with no variant marker to tell
// them apart, e.g. "T3M" occurs 12 times in the 2025-10-03 file. One
// definition is unreachable, the other is reachable but untriggered here.
const duplicateLabelPack = `SECTION 1: Species aggregation
SECTION 1: End
SECTION 2: Species groups
### Trees
     Fagus sylvatica
SECTION 2: End
SECTION 3: Group definitions

4          DUP  Unreachable definition
<#02 Trees>
4          DUP  Reachable definition
<#TC Trees GR 99>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func TestStatsCollapsesDuplicateLabelsAndPrefersReachable(t *testing.T) {
	pack, err := rulepack.Load(strings.NewReader(duplicateLabelPack))
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(pack, nil, nil)
	st := s.Stats()

	for _, label := range st.Unreachable {
		if label == "DUP" {
			t.Errorf("DUP has a reachable definition, must not be listed as unreachable: %v", st.Unreachable)
		}
	}
	count := 0
	for _, label := range st.NeverFired {
		if label == "DUP" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("DUP must appear exactly once in NeverFired, appeared %d times: %v", count, st.NeverFired)
	}
}

func TestStatsCountsQuestionAndPlus(t *testing.T) {
	s := newStatsTestService(t)
	// No taxa satisfy any rule, so the plot answers "?".
	if _, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 1}},
		Backbone: "euro+med",
		Header:   validHeader(),
	}); err != nil {
		t.Fatal(err)
	}
	st := s.Stats()
	if st.Total != 1 || st.Question != 1 || st.Plus != 0 {
		t.Errorf("Stats = %+v, want Total=1 Question=1 Plus=0", st)
	}
}
