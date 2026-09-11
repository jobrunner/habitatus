package rulepack

import (
	"sort"
	"strings"
)

// ParseAggregation parses section 1 (species aggregation): an unindented
// header line names the target concept, the indented lines below it are the
// source names that map onto it.
//
// Blocks are delimited by indentation, not by blank lines — the backbone
// translation tables have no separator lines at all.
//
// When a source name appears under two targets, the first occurrence wins.
// That is not a choice: upstream resolves names with match(), which returns the
// first hit. Self-mappings are dropped, as upstream does with
// AGG[AGG$values != AGG$ind, ].
func ParseAggregation(lines []string) (map[string]string, Issues) {
	out := make(map[string]string, len(lines))
	var issues Issues
	dupes := map[string]bool{}
	targets := map[string]bool{}

	target := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isIndented(line) {
			if target == "" {
				continue // member without a header — ignore
			}
			src := trimEntry(line, false)
			if src == "" || src == target {
				continue
			}
			if _, seen := out[src]; seen {
				if !dupes[src] {
					dupes[src] = true
					issues.DuplicateSources = append(issues.DuplicateSources, src)
				}
				continue // first entry wins
			}
			out[src] = target
			continue
		}
		target = trimEntry(line, true)
		targets[target] = true
	}

	for src := range out {
		if targets[src] {
			issues.Chains = append(issues.Chains, src)
		}
	}
	sort.Strings(issues.DuplicateSources)
	sort.Strings(issues.Chains)
	return out, issues
}

func isIndented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

// trimEntry strips the trailing layer number from an entry line.
// For headers, it also strips the "-" marker.
// "Abies alba          -  0" -> "Abies alba" (header, isHeader=true)
// "     Abies pectinata   0" -> "Abies pectinata" (member, isHeader=false)
func trimEntry(line string, isHeader bool) string {
	s := strings.TrimSpace(line)
	// drop trailing digits
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return s // no trailing number
	}
	s = strings.TrimRight(s[:i], " \t")
	if isHeader {
		s = strings.TrimSuffix(s, "-")
	}
	return strings.TrimRight(s, " \t")
}
