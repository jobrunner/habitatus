package cover

import (
	"math"
	"testing"
)

func TestUnionJenningsFischer(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{30}, 30},
		{[]float64{3, 4}, 6.88},
		{[]float64{10, 9, 8}, 24.652},
		{[]float64{100, 50}, 100},
	}
	for _, c := range cases {
		if got := Union(c.in); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Union(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// The union is commutative, so input order must not matter.
func TestUnionIsOrderIndependent(t *testing.T) {
	a := Union([]float64{12, 7, 33, 1})
	b := Union([]float64{1, 33, 7, 12})
	if a != b {
		t.Errorf("order changed the result: %v vs %v", a, b)
	}
}

// Upstream rounds to 10 decimals (commit 416bab9) to stabilise threshold cases.
func TestUnionRoundsToTenDecimals(t *testing.T) {
	got := Union([]float64{1.0 / 3.0, 1.0 / 3.0})
	if got != RoundTo(got, 10) {
		t.Errorf("result is not rounded to 10 decimals: %v", got)
	}
}
