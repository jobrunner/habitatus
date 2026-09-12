package esy

import "fmt"

// Mode selects the evaluation semantics for the 128 membership expressions
// the R implementation leaves without a comparison operator, i.e. the ones
// rulepack.Expr.AlwaysFalse marks (127 of the form "#NN Group", plus
// "#TC Cliff-ferns GR05", whose operator is glued to its operand). Every
// other expression evaluates identically in both modes; nothing else
// distinguishes them.
//
// ESy v1.2 discards such an expression's numeric result wholesale
// (step3and5_extract-and-solve-membership-conditions.R:493):
//
//	logi1[which(unlist(lapply(logi1, is.numeric)))] <- FALSE
//
// That line was added on 2025-02-03 (upstream commit 376cffc). Before it,
// the numeric vectors went into R's "&" and "|" unchanged and R coerced
// them — 0 is FALSE, anything else TRUE — which is exactly what the line
// commented out directly above it restores:
//
//	# logi1[...] <- lapply(logi1[...], function(x) ifelse(x == 0, FALSE, TRUE))
//
// The difference is not cosmetic: under v1.2, 100 of the 312 rules of the
// 2025-10-03 rule file can never fire, among them the whole grassland block
// R11-R57. See docs/superpowers/specs/2026-09-12-zwei-semantiken.md.
type Mode int8

const (
	// Repaired evaluates those expressions by coercing their numeric value,
	// as R itself did up to and including ESy v1.1. It is the zero value on
	// purpose: a caller who does not choose gets the semantics the service
	// runs in production, never the v1.2 regression by accident.
	Repaired Mode = iota
	// Faithful reproduces ESy v1.2 as shipped: those expressions are FALSE
	// for every plot. It exists for the parity proof against v1.2 and for
	// comparing against results other v1.2 runs produced.
	Faithful
)

// ModeNames maps each mode to the name the command line and the "mode" entry
// of every response use.
var modeNames = map[Mode]string{
	Repaired: "repaired",
	Faithful: "faithful",
}

func (m Mode) String() string {
	if s, ok := modeNames[m]; ok {
		return s
	}
	return fmt.Sprintf("Mode(%d)", int8(m))
}

// ParseMode turns a mode name into a Mode. An unknown name is an error and
// never a silent fallback: which semantics produced a classification is not
// something a caller may be left guessing about.
func ParseMode(s string) (Mode, error) {
	for m, name := range modeNames {
		if s == name {
			return m, nil
		}
	}
	return 0, fmt.Errorf("unknown mode %q, want %q or %q", s, Repaired, Faithful)
}

// ModeNames returns every valid mode name, for use in usage strings.
func ModeNames() []string { return []string{Repaired.String(), Faithful.String()} }
