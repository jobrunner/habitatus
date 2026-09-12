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
// loop (ParsingExpertFile.R:19-60); splitting it would hide the correspondence
// to the R source that makes this parser auditable.
//
//nolint:gocognit // one pass over section 1 that mirrors upstream's own single
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
			src := trimEntry(line)
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
		target = trimEntry(line)
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
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

// trimEntry reproduces upstream's entry-line trimming for section 1: R applies
// trim.leading and then trim.trailing to every member line and trim.trailing to
// every header line (spike/ESy-upstream/code/ParsingExpertFile.R:19-25).
//
//	trim.leading  <- function (x) sub("^\\s+", "", x)
//	trim.trailing <- function (x) sub("\\s+$|\\s+\\d$|\\s+\\-\\s+\\d$", "", x)
//
// (spike/ESy-upstream/code/prep.R:53-55)
//
// So exactly one trailing element is removed: a run of whitespace, or a single
// whitespace-preceded digit, or " - <digit>" — and nothing else. Notably, where
// a name is long enough that the layer digit is glued straight onto its last
// character with no separating whitespace, R keeps the digit; nine names in the
// real file are of that shape. Stripping it would repair the source file, which
// the port must not do. The "-" header marker needs no special case here: R's
// third alternative removes it together with the digit.
//
//	"Abies alba          -  0" -> "Abies alba"
//	"     Abies pectinata   0" -> "Abies pectinata"
//	"… var. trichostachyum0"   -> "… var. trichostachyum0"
func trimEntry(line string) string {
	return trimTrailing(trimLeadingSpace(line))
}

func trimLeadingSpace(s string) string {
	i := 0
	for i < len(s) && isSpaceByte(s[i]) {
		i++
	}
	return s[i:]
}

// trimTrailing removes the leftmost suffix that matches one of R's three
// alternatives, mirroring sub()'s leftmost-match semantics. All three are
// anchored at the end of the string, so the leftmost match is also the longest
// and the alternation order cannot matter.
func trimTrailing(s string) string {
	for i := 0; i < len(s); i++ {
		if isTrailer(s[i:]) {
			return s[:i]
		}
	}
	return s
}

// isTrailer reports whether t matches `\s+`, `\s+\d` or `\s+\-\s+\d` in full.
func isTrailer(t string) bool {
	rest, ok := afterSpaceRun(t)
	if !ok {
		return false
	}
	if rest == "" { // \s+$
		return true
	}
	if isDigitOnly(rest) { // \s+\d$
		return true
	}
	if rest[0] != '-' { // \s+\-\s+\d$
		return false
	}
	rest, ok = afterSpaceRun(rest[1:])
	return ok && isDigitOnly(rest)
}

// afterSpaceRun consumes one or more leading whitespace bytes. The run is
// greedy, which loses nothing: every alternative continues with a non-space.
func afterSpaceRun(s string) (string, bool) {
	rest := trimLeadingSpace(s)
	return rest, len(rest) < len(s)
}

func isDigitOnly(s string) bool {
	return len(s) == 1 && s[0] >= '0' && s[0] <= '9'
}

func isSpaceByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}
