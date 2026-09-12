// Package taxa resolves input taxon names onto ESy target concepts.
package taxa

import (
	"sort"

	"github.com/jobrunner/habitatus/internal/cover"
)

// Record is one taxon observation.
type Record struct {
	Name  string
	Cover float64
}

// Step documents how one input name was resolved, for the response report.
type Step struct {
	Input         string
	AfterBackbone string
	Final         string
	Resolved      bool
}

// corrections fixes defects in the official rule file that are demonstrably
// data errors rather than intent. See spec section 8.
//
// "Populus x canadensis + P. nigra" is listed under the grass "Polypogon
// monspeliensis x viridis"; Polypogon and Populus are adjacent in an
// alphabetically sorted list, so the record sits one block too early.
//
// This is the one deliberate deviation from upstream behaviour in the whole
// port: R applies the rule file as written and maps that name to the grass.
// A plot containing it would therefore differ from the golden master — no
// plot among the 11,337 does, which is why the fixtures stay at zero. Every
// other quirk of the original is reproduced rather than repaired; this one
// was decided in spec §8 and must not be extended to further "obvious" fixes
// without the same explicit decision.
var corrections = map[string]string{
	"Populus x canadensis + P. nigra": "Populus x canadensis",
}

// Resolve maps input names onto ESy concepts in two stages — first the source
// nomenclature's translation table, then section 1 aggregation — merging covers
// after each stage.
//
// known is the set of names the rule pack can actually act on: every member of
// a species group plus every taxon named directly in a rule. A name whose final
// concept is not in that set can never satisfy any condition, and that is what
// Step.Resolved reports. Pass nil to mark every step resolved.
//
// Matching is an exact string lookup with no normalisation, mirroring
// upstream's match(). Resolution is single-pass: chains are never followed.
func Resolve(in []Record, backbone, aggregation map[string]string, known map[string]bool) ([]Record, []Step) {
	steps := make([]Step, len(in))
	staged := make([]Record, len(in))
	for i, r := range in {
		steps[i].Input = r.Name
		afterBackbone := r.Name
		if t, ok := backbone[r.Name]; ok {
			afterBackbone = t
		}
		steps[i].AfterBackbone = afterBackbone
		staged[i] = Record{Name: afterBackbone, Cover: r.Cover}
	}
	staged = merge(staged)

	final := make([]Record, len(staged))
	for i, r := range staged {
		final[i] = Record{Name: finalName(r.Name, aggregation), Cover: r.Cover}
	}

	for i := range steps {
		steps[i].Final = finalName(steps[i].AfterBackbone, aggregation)
		steps[i].Resolved = known == nil || known[steps[i].Final]
	}

	return merge(final), steps
}

// finalName applies stage-3 resolution (corrections, then aggregation) to a
// single staged name.
func finalName(name string, aggregation map[string]string) string {
	if c, ok := corrections[name]; ok {
		return c
	}
	if t, ok := aggregation[name]; ok {
		return t
	}
	return name
}

// merge sums duplicate concepts with the Jennings-Fischer union and returns a
// deterministically ordered slice.
func merge(rs []Record) []Record {
	byName := map[string][]float64{}
	for _, r := range rs {
		byName[r.Name] = append(byName[r.Name], r.Cover)
	}
	out := make([]Record, 0, len(byName))
	for name, covers := range byName {
		out = append(out, Record{Name: name, Cover: cover.Union(covers)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
