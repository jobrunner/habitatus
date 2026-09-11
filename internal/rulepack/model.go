package rulepack

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
