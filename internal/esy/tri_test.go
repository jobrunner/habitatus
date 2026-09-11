package esy

import "testing"

func TestTriKleene(t *testing.T) {
	// Test vectors verified against R (commit 416bab9 upstream).
	cases := []struct {
		a, b       Tri
		and, or    Tri
		andNot     Tri
	}{
		{True, True, True, True, False},
		{True, False, False, True, True},
		{False, True, False, True, False},
		{False, False, False, False, False},
		{True, Unknown, Unknown, True, Unknown},
		{Unknown, True, Unknown, True, False},
		{False, Unknown, False, Unknown, False},
		{Unknown, False, False, Unknown, Unknown},
		{Unknown, Unknown, Unknown, Unknown, Unknown},
	}
	for _, c := range cases {
		if got := c.a.And(c.b); got != c.and {
			t.Errorf("%v AND %v = %v, want %v", c.a, c.b, got, c.and)
		}
		if got := c.a.Or(c.b); got != c.or {
			t.Errorf("%v OR %v = %v, want %v", c.a, c.b, got, c.or)
		}
		if got := c.a.AndNot(c.b); got != c.andNot {
			t.Errorf("%v NOT %v = %v, want %v", c.a, c.b, got, c.andNot)
		}
	}
}

func TestTriAndOrCommutative(t *testing.T) {
	// AND and OR must be commutative (Kleene logic property).
	// ANDNOT is intentionally NOT commutative.
	cases := []struct {
		a, b Tri
	}{
		{True, True},
		{True, False},
		{True, Unknown},
		{False, True},
		{False, False},
		{False, Unknown},
		{Unknown, True},
		{Unknown, False},
		{Unknown, Unknown},
	}
	for _, c := range cases {
		if got := c.a.And(c.b); got != c.b.And(c.a) {
			t.Errorf("AND not commutative: %v AND %v = %v, but %v AND %v = %v",
				c.a, c.b, got, c.b, c.a, c.b.And(c.a))
		}
		if got := c.a.Or(c.b); got != c.b.Or(c.a) {
			t.Errorf("OR not commutative: %v OR %v = %v, but %v OR %v = %v",
				c.a, c.b, got, c.b, c.a, c.b.Or(c.a))
		}
	}
}

func TestTriIsTrueTreatsUnknownAsNoMatch(t *testing.T) {
	if Unknown.IsTrue() {
		t.Error("Unknown must not count as a match — upstream selects with == TRUE")
	}
}
