package esy

import "testing"

// TestModeZeroValueIsRepaired pins the default. A caller who forgets to
// choose must get the semantics the service runs in production, never the
// v1.2 regression by accident.
func TestModeZeroValueIsRepaired(t *testing.T) {
	var m Mode
	if m != Repaired {
		t.Fatalf("zero Mode is %v, want repaired", m)
	}
	if (Env{}).Mode != Repaired {
		t.Fatalf("zero Env evaluates in %v, want repaired", (Env{}).Mode)
	}
}

func TestParseMode(t *testing.T) {
	for name, want := range map[string]Mode{"repaired": Repaired, "faithful": Faithful} {
		got, err := ParseMode(name)
		if err != nil || got != want {
			t.Errorf("ParseMode(%q) = %v, %v; want %v, nil", name, got, err, want)
		}
		if want.String() != name {
			t.Errorf("%v.String() = %q, want %q", want, want.String(), name)
		}
	}
	// An unknown name is an error, never a silent fallback: which semantics
	// produced a result is not something a caller may be left guessing about.
	if _, err := ParseMode("Repaired"); err == nil {
		t.Error("ParseMode(\"Repaired\") accepted a name that is not a mode")
	}
	if _, err := ParseMode(""); err == nil {
		t.Error("ParseMode(\"\") accepted the empty name")
	}
}
