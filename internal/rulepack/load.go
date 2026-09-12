package rulepack

import (
	"fmt"
	"io"
	"sort"
)

// Load parses a complete ESy file. Parse errors abort; data defects are
// collected in Issues and never abort, because the official rule file contains
// defects and must still be usable.
func Load(r io.Reader) (*Pack, error) {
	secs, err := SplitSections(r)
	if err != nil {
		return nil, err
	}
	// Sections 1-3 carry the nomenclature table, the groups and the rules
	// themselves; a file missing any of them is not a data defect but a
	// parse failure, and must stop the start rather than load as a partial
	// pack that silently answers "?" for every plot. Section 4 is empty in
	// the real file and stays optional.
	for _, n := range []int{1, 2, 3} {
		if _, ok := secs[n]; !ok {
			return nil, fmt.Errorf("section %d is missing", n)
		}
	}
	agg, issues := ParseAggregation(secs[1])
	groups := ParseGroups(secs[2])
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		return nil, err
	}

	// The section-1 taxon universe: every name section 1 talks about, as
	// either an aggregation source or an aggregation target. An atom's name
	// found here is a taxon, never a misclassified group, no matter what
	// characters it contains (e.g. "Abies borisii-regis").
	taxonUniverse := make(map[string]bool, 2*len(agg))
	for src, tgt := range agg {
		taxonUniverse[src] = true
		taxonUniverse[tgt] = true
	}

	knownTaxa := map[string]bool{}
	for _, members := range groups {
		for _, m := range members {
			knownTaxa[m] = true
		}
	}

	unknown := map[string]bool{}
	for i := range rules {
		n, err := ParseFormula(rules[i].Raw)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rules[i].Label(), err)
		}
		rules[i].Formula = n
		for _, raw := range extractExpressions(rules[i].Raw) {
			e, err := ParseExpr(raw)
			if err != nil {
				return nil, fmt.Errorf("rule %s: %w", rules[i].Label(), err)
			}
			for _, a := range append(append([]Atom{}, e.Left.Atoms...), e.Right.Atoms...) {
				if a.Kind == "$$C" || a.Kind == "$$N" || a.Name == "" {
					continue
				}
				if a.Kind == "" {
					knownTaxa[a.Name] = true
				}
				key := groupKeyOf(a)
				if _, ok := groups[key]; ok {
					continue
				}
				if a.Qualifier == "" {
					// Unqualified atoms fall back to being a bare taxon
					// name when they name no group (see resolveInto in
					// esy/condition.go) — that is always a valid reference,
					// never a defect.
					continue
				}
				if taxonUniverse[a.Name] {
					continue
				}
				unknown[a.Name] = true
			}
		}
	}
	for n := range unknown {
		issues.UnknownGroups = append(issues.UnknownGroups, n)
	}
	sort.Strings(issues.UnknownGroups)
	return &Pack{
		Aggregation: agg,
		Groups:      groups,
		Rules:       rules,
		Issues:      issues,
		KnownTaxa:   knownTaxa,
	}, nil
}

// groupKeyOf is the lookup key ParseGroups produces for a group header: the
// qualifier plus the name when the atom carries a qualifier, the bare name
// otherwise. This is what makes qualified references resolvable — dropping
// the qualifier would fail every one of them.
func groupKeyOf(a Atom) string {
	if a.Qualifier != "" {
		return a.Qualifier + " " + a.Name
	}
	return a.Name
}
