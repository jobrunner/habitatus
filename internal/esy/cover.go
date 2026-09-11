package esy

import "github.com/jobrunner/habitatus/internal/cover"

// TotalCover merges cover values assuming independent random overlap, the
// Jennings-Fischer union used throughout ESy. See internal/cover.Union for
// the formula, its rationale and its tests; this is a thin re-export kept
// here because it is package esy's public entry point and every caller in
// this package uses it under this name.
func TotalCover(covers []float64) float64 {
	return cover.Union(covers)
}
