package rulepack

// Rule is one habitat definition from section 3.
type Rule struct {
	// Priority is the leading digit, 1..8. Higher wins.
	Priority int
	// Code is the EUNIS code without the variant marker, e.g. "N15".
	Code string
	// Variant is "", "!" or "!!" — alternative definitions of the same code.
	Variant string
	// Name is the habitat's plain-text name.
	Name string
	// Raw is the membership formula as text, continuation lines joined.
	Raw string
	// Formula is the parsed formula. Filled in by Load.
	Formula Node
}

// Label returns the code plus its variant marker, e.g. "N15!!".
func (r Rule) Label() string { return r.Code + r.Variant }

// Atom is one term inside a membership expression.
type Atom struct {
	// Kind is the prefix: "#TC", "##Q", "##C", "###", "##D", "#SC", "#T$",
	// "#$$", "$$C", "$$N", "NON", a "#NN" species count, or "" for a bare
	// taxon name. isCountPrefix accepts "#00".."#99"; the 2025-10-03 file
	// uses "#01".."#05".
	Kind string
	// Qualifier is the comparison set marker, e.g. "+04". Empty if absent.
	Qualifier string
	// Name is the group, header field or taxon name. Empty for "#$$"/"#T$"
	// when they stand alone on the right-hand side.
	Name string
	// Inner is the measure a NON atom negates, e.g. "##Q" in "NON ##Q +10 Grp".
	// Empty for every other kind.
	Inner string
}

// Operand is one side of a membership expression: a union of atoms, optionally
// minus an EXCEPT union, or a bare literal (a number, a "$NN" percentage or a
// categorical header value).
type Operand struct {
	Atoms   []Atom
	Except  []Atom
	Literal string
}

// Expr is a membership expression, the content of one <...> pair.
type Expr struct {
	Left  Operand
	Op    string // "GR", "GE", "EQ", or "" when there is no right-hand side
	Right Operand
	// AlwaysFalse marks an expression that upstream never evaluates as a
	// comparison at all: it reduces to a single numeric condition, and
	// v1.2 replaces every numeric expression result with FALSE. See
	// ParseExpr for the two shapes this happens to and the R citation.
	AlwaysFalse bool
}

// Node is a node of a parsed membership formula.
type Node interface{ isNode() }

// And is "x AND y".
type And struct{ L, R Node }

// Or is "x OR y".
type Or struct{ L, R Node }

// Not is "x NOT y", the binary "and not" of the expert-system language.
type Not struct{ L, R Node }

// Leaf is a single membership expression.
type Leaf struct {
	Expr Expr
	Raw  string
}

func (And) isNode()  {}
func (Or) isNode()   {}
func (Not) isNode()  {}
func (Leaf) isNode() {}

// Pack is a fully parsed and assembled ESy rule pack, ready for evaluation.
type Pack struct {
	// Aggregation maps a source taxon name to its target (section 1).
	Aggregation map[string]string
	// Groups maps a group key (Qualifier + " " + Name, or just Name when
	// there is no qualifier) to its member taxon names (section 2).
	Groups map[string][]string
	// Rules are the parsed rules of section 3, each carrying a Formula.
	Rules []Rule
	// Issues collects the defects found while parsing.
	Issues Issues
	// KnownTaxa is the set of taxon names the rule pack can act on: every
	// member of every species group, unioned with every taxon named
	// directly in a rule (a Leaf atom whose Kind is empty). taxa.Resolve
	// uses this to tell a caller which of their names can never satisfy any
	// condition.
	KnownTaxa map[string]bool
}

// Issues counts and names the defects found in a rule file.
type Issues struct {
	// DuplicateSources are source names listed under more than one target.
	// The first occurrence wins, matching upstream's match() semantics.
	DuplicateSources []string
	// Chains are names that are both a source and a target. Resolution is
	// single-pass, so chains are not followed.
	Chains []string
	// UnknownGroups are group names referenced by a rule but never defined.
	UnknownGroups []string
}
