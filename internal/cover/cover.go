// Package cover holds the Jennings-Fischer cover-union arithmetic shared by
// internal/esy and internal/taxa.
//
// It exists only to break an import cycle: internal/esy's Plot (Task 9) needs
// taxa.Record, but internal/taxa's merge step (Task 7 and earlier) needs the
// same union formula esy.TotalCover already implements. esy importing taxa
// while taxa imports esy is not possible in Go, so the shared arithmetic
// lives here instead, and both packages depend on it. esy.TotalCover remains
// the public entry point package esy and its callers use; it delegates here.
package cover

import "math"

// Union merges cover values assuming independent random overlap, the
// Jennings-Fischer union used throughout ESy:
//
//	total = 1 - prod(1 - c/100)
//
// Cover is projected area, not amount, so adding values would double-count
// the overlap and can exceed 100%. Upstream rounds to 10 decimals (commit
// 416bab9) to keep threshold comparisons stable, and this reproduces that
// exactly.
func Union(covers []float64) float64 {
	if len(covers) == 0 {
		return 0
	}
	rest := 1.0
	for _, c := range covers {
		rest *= 1 - c/100
	}
	return RoundTo((1-rest)*100, 10)
}

// RoundTo rounds v to the given number of decimal places.
func RoundTo(v float64, decimals int) float64 {
	f := math.Pow(10, float64(decimals))
	return math.Round(v*f) / f
}
