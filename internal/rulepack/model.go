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
}

// Label returns the code plus its variant marker, e.g. "N15!!".
func (r Rule) Label() string { return r.Code + r.Variant }

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
