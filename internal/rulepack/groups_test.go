package rulepack

import "testing"

func TestParseGroups(t *testing.T) {
	lines := []string{
		"### Acidophilous-oak-forest-trees",
		"     Castanea sativa",
		"     Quercus petraea",
		"",
		"### Alnus-Fraxinus-excelsior",
		"     Alnus glutinosa",
		"     Fraxinus excelsior",
	}
	got := ParseGroups(lines)
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d", len(got))
	}
	oak := got["Acidophilous-oak-forest-trees"]
	if len(oak) != 2 || oak[0] != "Castanea sativa" || oak[1] != "Quercus petraea" {
		t.Errorf("oak group wrong: %q", oak)
	}
}

// Upstream commit fb86835 fixed a bug that dropped the last species of the last
// group. This test pins that behaviour.
func TestParseGroupsKeepsLastMemberOfLastGroup(t *testing.T) {
	lines := []string{
		"### Trees",
		"     Abies alba",
		"     Quercus robur",
	}
	got := ParseGroups(lines)
	if len(got["Trees"]) != 2 {
		t.Fatalf("want 2 members, got %q", got["Trees"])
	}
	if got["Trees"][1] != "Quercus robur" {
		t.Errorf("last member lost, got %q", got["Trees"])
	}
}

// Member lines in section 2 carry no trailing number, and some groups end with
// a whitespace-only line before the next header.
func TestParseGroupsIgnoresWhitespaceLines(t *testing.T) {
	lines := []string{
		"### Alnus-Fraxinus-excelsior",
		"     Alnus glutinosa",
		"     ",
		"### Next",
		"     Betula pendula",
	}
	got := ParseGroups(lines)
	if len(got["Alnus-Fraxinus-excelsior"]) != 1 {
		t.Errorf("whitespace line became a member: %q", got["Alnus-Fraxinus-excelsior"])
	}
}

// Section 2 also uses "##D +NN <name>" as a group header, alongside "### ".
// The key is everything from character 5 onward (line[4:]), trimmed, which
// unifies both header forms: upstream computes group names as
// substr(names(groups), 5, nchar(...)).
func TestParseGroupsQualifiedHeaderForm(t *testing.T) {
	lines := []string{
		"##D +01 MA211-Arctic-coastal-saltmarsh",
		"     Agrostis stolonifera",
		"     Carex glareosa",
		"### Next",
		"     Betula pendula",
	}
	got := ParseGroups(lines)
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d", len(got))
	}
	members := got["+01 MA211-Arctic-coastal-saltmarsh"]
	if len(members) != 2 || members[0] != "Agrostis stolonifera" || members[1] != "Carex glareosa" {
		t.Errorf("qualified group wrong: %q", members)
	}
	if len(got["Next"]) != 1 || got["Next"][0] != "Betula pendula" {
		t.Errorf("following group wrong: %q", got["Next"])
	}
}
