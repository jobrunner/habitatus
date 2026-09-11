package esy

import "math"

// TotalCover merges cover values assuming independent random overlap, the
// Jennings-Fischer union used throughout ESy:
//
//	total = 1 - prod(1 - c/100)
//
// Cover is projected area, not amount, so adding values would double-count the
// overlap and can exceed 100%. Upstream rounds to 10 decimals (commit 416bab9)
// to keep threshold comparisons stable, and we reproduce that exactly.
func TotalCover(covers []float64) float64 {
	if len(covers) == 0 {
		return 0
	}
	rest := 1.0
	for _, c := range covers {
		rest *= 1 - c/100
	}
	return roundTo((1-rest)*100, 10)
}

func roundTo(v float64, decimals int) float64 {
	f := math.Pow(10, float64(decimals))
	return math.Round(v*f) / f
}
