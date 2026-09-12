// Package rulepack parses ESy expert-system files into an immutable model.
package rulepack

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SplitSections splits an ESy file into its numbered sections. The SECTION
// marker lines themselves are dropped; every other line is preserved verbatim,
// including blank and whitespace-only lines, because block structure depends on
// them in some sections and on indentation in others.
func SplitSections(r io.Reader) (map[int][]string, error) {
	out := map[int][]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	current := 0
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if n, end, ok := parseSectionMarker(line); ok {
			if end {
				if n != current {
					return nil, fmt.Errorf("section %d ends while section %d is open", n, current)
				}
				current = 0
				continue
			}
			if current != 0 {
				return nil, fmt.Errorf("section %d starts while section %d is open", n, current)
			}
			current = n
			if _, seen := out[n]; seen {
				return nil, fmt.Errorf("section %d appears twice", n)
			}
			out[n] = []string{}
			continue
		}
		if current != 0 {
			out[current] = append(out[current], line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if current != 0 {
		return nil, fmt.Errorf("section %d is not terminated", current)
	}
	return out, nil
}

// parseSectionMarker recognises "SECTION <n>: End" and "SECTION <n>: <title>".
func parseSectionMarker(line string) (n int, end, ok bool) {
	if !strings.HasPrefix(line, "SECTION ") {
		return 0, false, false
	}
	rest := line[len("SECTION "):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return 0, false, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:colon]))
	if err != nil {
		return 0, false, false
	}
	return n, strings.TrimSpace(rest[colon+1:]) == "End", true
}
