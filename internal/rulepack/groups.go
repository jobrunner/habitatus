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
		out[name] = append(out[name], strings.TrimSpace(line))
	}
	return out
}
