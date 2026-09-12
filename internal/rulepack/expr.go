package rulepack

import (
	"fmt"
	"strings"
)

var atomPrefixes = []string{
	"$$C", "$$N", "###", "##D", "##C", "##Q", "#TC", "#SC", "#T$", "#$$", "NON",
}

// operatorTokens are the comparison operators that may split an expression.
var operatorTokens = []string{"GR", "GE", "EQ"}

// ParseExpr parses the inside of one <...> membership expression.
//
// The operator is located as the LAST free-standing GR/GE/EQ occurrence, not
// the first: header field names contain spaces and parentheses ("Altitude
// (m)") and categorical values are multi-word ("United Kingdom"), so a
// left-to-right scan would cut in the wrong place. The operator is usually
// preceded and followed by a space, but in exactly one expression in the real
// rule file it is glued to its right-hand side ("<#TC Cliff-ferns GR05>"), so
// the operator is found by substring search rather than by splitting on
// spaces: the last occurrence of GR/GE/EQ that is preceded by a space and
// followed by either a space or directly by a digit, '#', '$' or '-'. No
// group name or taxon name in the rule file contains GR, GE or EQ as a
// substring, which makes this safe.
func ParseExpr(s string) (Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Expr{}, fmt.Errorf("empty expression")
	}
	left, op, right := splitOnOperator(s)
	e := Expr{Op: op}
	var err error
	if e.Left, err = parseOperand(left, false); err != nil {
		return Expr{}, fmt.Errorf("left of %q: %w", s, err)
	}
	if op != "" {
		// An operator with nothing to its right used to yield a zero-valued
		// operand, which compares as an empty literal and quietly evaluates
		// FALSE. There is no such expression in the rule file, and there is
		// no reading of one that we could defend.
		if strings.TrimSpace(right) == "" {
			return Expr{}, fmt.Errorf("%q: operator %s has no right-hand side", s, op)
		}
		if e.Right, err = parseOperand(right, true); err != nil {
			return Expr{}, fmt.Errorf("right of %q: %w", s, err)
		}
		applySCExcept(&e)
	}
	e.AlwaysFalse = isAlwaysFalse(s)
	if e.AlwaysFalse && op != "" {
		// The glued-operator shape ("#TC Cliff-ferns GR05"). Upstream splits
		// expressions into conditions on the SPACED operators only, so this
		// one is never split: the whole text, operator and all, is ONE
		// condition, and the group name it looks up is "Cliff-ferns GR05" —
		// substr(condition, 5, nchar(condition)) — which matches no group,
		// so the condition's value stays at plot.cond's zero default for
		// every plot (step3and5…R:113-122; fmatch returns NA, groups[[NA]]
		// is NULL, and nothing is written back).
		//
		// The parser must therefore not act on the operator it found here,
		// or the two modes would part company over more than the coercion:
		// in repaired mode this expression would become "the group has any
		// cover" instead of the constant FALSE that R computes. Reparsing
		// the whole text as a single operand reproduces R's zero for the
		// same reason R gets it — an unresolvable name resolving to an
		// empty member set. The missing space stays a defect of the rule
		// file in both modes; we reproduce it rather than guess at the
		// threshold of 5 % that was presumably meant.
		whole, err := parseOperand(s, false)
		if err != nil {
			return Expr{}, fmt.Errorf("%q as a single condition: %w", s, err)
		}
		e.Left, e.Op, e.Right = whole, "", Operand{}
	}
	return e, nil
}

// applySCExcept reproduces upstream's step 3B (ParsingExpertFile.R:404-430).
// When the right-hand side names a "#SC" group, upstream appends "EXCEPT
// <left-hand side>" to the expression, so the group's maximum cover is taken
// over every member except the taxon or group being compared against it:
//
//	index4 <- grep("#SC", b[[2]])
//	membership.expressions[index6] <- paste(membership.expressions[index6],
//	                                        "EXCEPT", b[[1]][index4[i]], sep=" ")
//
// Without it "<Fagus sylvatica GR #SC Trees>" compares Fagus against a maximum
// that includes Fagus itself and can never be true.
func applySCExcept(e *Expr) {
	if len(e.Right.Except) > 0 {
		return
	}
	for _, a := range e.Right.Atoms {
		if a.Kind == "#SC" {
			e.Right.Except = e.Left.Atoms
			return
		}
	}
}

// isAlwaysFalse reports whether upstream leaves this expression as a bare
// number instead of a comparison. Upstream builds its conditions by splitting
// the expression text on the SPACED operators only
// (ParsingExpertFile.R:432-434):
//
//	membership.conditions2 <- unlist(strsplit(membership.expressions, " GR "))
//
// so an expression that contains no " GR "/" GE "/" EQ " stays a single
// condition, its evaluated form is a bare "colN", and step 8 discards it:
//
//	logi1[which(unlist(lapply(logi1, is.numeric)))] <- FALSE
//
// Two shapes reach that state. An expression whose operator is glued to its
// operand ("#TC Cliff-ferns GR05", the only one in the 2025-10-03 file), and
// the "#NN Group" minimum-species-count form, which has no operator and is
// explicitly excluded from the "GR NON" completion by
// ParsingExpertFile.R:237 testing whether character 3 is a digit. Every other
// operator-less expression gets " GR NON <self>" appended and is evaluated
// normally.
func isAlwaysFalse(s string) bool {
	spaced := strings.Contains(s, " GR ") || strings.Contains(s, " GE ") ||
		strings.Contains(s, " EQ ")
	if spaced {
		return false
	}
	any := strings.Contains(s, "GR") || strings.Contains(s, "GE") ||
		strings.Contains(s, "EQ")
	if any {
		return true
	}
	return len(s) >= 3 && s[2] >= '0' && s[2] <= '9'
}

// splitOnOperator finds the last standalone GR/GE/EQ occurrence: preceded by
// a space, followed by a space or directly by a digit, '#', '$' or '-'.
func splitOnOperator(s string) (left, op, right string) {
	bestIdx := -1
	bestOp := ""
	for _, tok := range operatorTokens {
		from := 0
		for {
			i := strings.Index(s[from:], tok)
			if i < 0 {
				break
			}
			i += from
			from = i + 1
			if i == 0 || s[i-1] != ' ' {
				continue
			}
			after := i + len(tok)
			if after < len(s) {
				c := s[after]
				if c != ' ' && !(c >= '0' && c <= '9') && c != '#' && c != '$' && c != '-' {
					continue
				}
			}
			if i > bestIdx {
				bestIdx = i
				bestOp = tok
			}
		}
	}
	if bestIdx < 0 {
		return s, "", ""
	}
	left = strings.TrimSpace(s[:bestIdx])
	right = strings.TrimSpace(s[bestIdx+len(bestOp):])
	return left, bestOp, right
}

// parseOperand parses one side. onRight allows bare literals (numbers, "$50",
// categorical values such as "ATL_COAST" or "United Kingdom").
func parseOperand(s string, onRight bool) (Operand, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Operand{}, nil
	}
	main, except := s, ""
	if i := strings.Index(s, "EXCEPT"); i >= 0 {
		main, except = strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len("EXCEPT"):])
		// Only one EXCEPT per operand is representable. A second one used to
		// be absorbed into the exception's atom name, so the operand kept
		// evaluating — against a group whose name silently included the word
		// EXCEPT and everything after it. None occurs in the rule file.
		if strings.Contains(except, "EXCEPT") {
			return Operand{}, fmt.Errorf("%q: a second EXCEPT is not representable", s)
		}
	}
	if onRight && !looksLikeAtom(main) && except == "" {
		return Operand{Literal: main}, nil
	}
	var op Operand
	var err error
	if op.Atoms, err = parseAtomList(main); err != nil {
		return Operand{}, err
	}
	if except != "" {
		if op.Except, err = parseAtomList(except); err != nil {
			return Operand{}, err
		}
	}
	return op, nil
}

func looksLikeAtom(s string) bool {
	for _, p := range atomPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return isCountPrefix(s)
}

// isCountPrefix matches "#01".."#99": the "at least N species" form.
func isCountPrefix(s string) bool {
	return len(s) >= 3 && s[0] == '#' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
}

func parseAtomList(s string) ([]Atom, error) {
	parts := strings.Split(s, "|")
	out := make([]Atom, 0, len(parts))
	for _, p := range parts {
		a, err := parseAtom(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func parseAtom(s string) (Atom, error) {
	if s == "" {
		return Atom{}, fmt.Errorf("empty atom")
	}
	var a Atom
	switch {
	case isCountPrefix(s):
		a.Kind, s = s[:3], strings.TrimSpace(s[3:])
	default:
		for _, p := range atomPrefixes {
			if !strings.HasPrefix(s, p) {
				continue
			}
			// Every prefix but NON is punctuation and cannot begin a name.
			// NON is three letters, so it needs a word boundary: without one,
			// "NONEA PULLA" reads as NON + "EA PULLA". Upstream has no
			// boundary check either (startsWith/grep, step3and5…R:289, 328),
			// so both readings are defensible and neither is safe to pick
			// silently. No name in the 2025-10-03 file begins with those
			// three capitals; if one ever does, this stops the load and a
			// human decides.
			if p == "NON" && len(s) > len(p) && s[len(p)] != ' ' {
				return Atom{}, fmt.Errorf("%q: NON without a following space is ambiguous", s)
			}
			a.Kind, s = p, strings.TrimSpace(s[len(p):])
			break
		}
	}
	if a.Kind == "NON" {
		inner, err := parseAtom(s)
		if err != nil {
			return Atom{}, fmt.Errorf("NON without an inner measure: %w", err)
		}
		a.Inner, a.Qualifier, a.Name = inner.Kind, inner.Qualifier, inner.Name
		return a, nil
	}
	// A qualifier is "+" followed by two digits.
	if len(s) >= 3 && s[0] == '+' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9' {
		a.Qualifier, s = s[:3], strings.TrimSpace(s[3:])
	}
	a.Name = s
	if a.Kind == "" && a.Name == "" {
		return Atom{}, fmt.Errorf("atom without kind or name")
	}
	return a, nil
}

// extractExpressions returns every <...> body in the formula, in order.
func extractExpressions(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		a := strings.IndexByte(s[i:], '<')
		if a < 0 {
			break
		}
		a += i
		b := strings.IndexByte(s[a:], '>')
		if b < 0 {
			break
		}
		out = append(out, s[a+1:a+b])
		i = a + b + 1
	}
	return out
}
