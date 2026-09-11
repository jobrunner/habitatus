package rulepack

import "strings"

// ParseGroups parses section 2 (species groups). A non-indented, non-empty
// line opens a group; the indented lines below it are its members, in file
// order.
//
// Section 2 uses two header forms: "### <name>" and "##D +NN <name>". Both
// are four characters wide before the name starts, so the group key is
// everything from character 5 onward (line[4:]), trimmed. This matches
// upstream, which derives group names as
// substr(names(groups), 5, nchar(...)) — the qualifier in the "##D" form
// stays part of the key, because later rule references look groups up by
// "Qualifier Name".
//
// Member lines keep their trailing whitespace. Upstream applies only
// trim.leading to them (ParsingExpertFile.R:37-38), so a member written with
// trailing blanks never matches the taxon name in a plot; trimming here would
// repair the rule file instead of porting it. 27 members of the 2025-10-03
// file are affected. Upstream trims member names in exactly one place — the
// "#TC <group>|<group> EXCEPT <x>" branch of step3and5...R:184-189 — but no
// group reachable from that branch has such a member, so the inconsistency
// has no effect on this file.
//
// The final member of the final group must survive — upstream lost exactly
// that entry until commit fb86835.
func ParseGroups(lines []string) map[string][]string {
	out := map[string][]string{}
	name := ""
	for _, line := range lines {
		if !isIndented(line) && strings.TrimSpace(line) != "" {
			rest := ""
			if len(line) > 4 {
				rest = line[4:]
			}
			name = strings.TrimSpace(rest)
			if _, ok := out[name]; !ok {
				out[name] = nil
			}
			continue
		}
		if name == "" || strings.TrimSpace(line) == "" {
			continue
		}
		out[name] = append(out[name], strings.TrimLeft(line, " \t"))
	}
	return out
}
