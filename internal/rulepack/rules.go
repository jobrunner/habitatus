package rulepack

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	codeStart = 11 // 0-based: column 12
	codeEnd   = 16 // 0-based exclusive: columns 12..16
)

// ParseRuleHeaders parses section 3 into rules.
//
// The header line is fixed-width: priority in column 1, the code in columns
// 12..16, the habitat name from column 17 on. Splitting on spaces would read
// "N15!!Atlantic" as the code, because the variant marker "!!" fills the field
// and leaves no separator.
//
// A formula may span several lines; every line until the next header belongs to
// the current rule and is joined with a single space.
func ParseRuleHeaders(lines []string) ([]Rule, error) {
	var out []Rule
	var buf []string

	flush := func() {
		if len(out) == 0 {
			return
		}
		out[len(out)-1].Raw = strings.Join(buf, " ")
		buf = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isRuleHeader(line) {
			flush()
			r, err := parseRuleHeader(line)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
			continue
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("formula line before any rule header: %q", line)
		}
		buf = append(buf, strings.TrimSpace(line))
	}
	flush()
	return out, nil
}

// isRuleHeader reports whether a line starts a rule: a single digit followed by
// a space. Formula lines start with "(" or "<".
func isRuleHeader(line string) bool {
	return len(line) > 1 && line[0] >= '0' && line[0] <= '9' && line[1] == ' '
}

func parseRuleHeader(line string) (Rule, error) {
	prio, err := strconv.Atoi(line[:1])
	if err != nil {
		return Rule{}, fmt.Errorf("rule header %q: bad priority: %w", line, err)
	}
	if len(line) < codeEnd {
		return Rule{}, fmt.Errorf("rule header %q is shorter than the code field", line)
	}
	field := strings.TrimSpace(line[codeStart:codeEnd])
	code := strings.TrimRight(field, "!")
	variant := field[len(code):]
	if code == "" {
		return Rule{}, fmt.Errorf("rule header %q has an empty code field", line)
	}
	name := ""
	if len(line) > codeEnd {
		name = strings.TrimSpace(line[codeEnd:])
	}
	return Rule{Priority: prio, Code: code, Variant: variant, Name: name}, nil
}
