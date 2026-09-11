// Package esy evaluates parsed ESy rule packs against a vegetation plot.
package esy

// Tri is a three-valued truth value. Upstream computes in R, where a missing
// header value produces NA and NA propagates through & and |. The final
// selection is which(matrix == TRUE), so NA never counts as a match.
type Tri int8

const (
	False Tri = iota
	True
	Unknown
)

func (t Tri) String() string {
	switch t {
	case False:
		return "FALSE"
	case True:
		return "TRUE"
	default:
		return "NA"
	}
}

// And is R's "&": FALSE on the left dominates, otherwise NA propagates.
// Asymmetric: Unknown AND False = Unknown, but False AND Unknown = False.
func (t Tri) And(o Tri) Tri {
	if t == False {
		return False
	}
	if o == False {
		if t == Unknown {
			return Unknown
		}
		return False
	}
	if t == Unknown || o == Unknown {
		return Unknown
	}
	return True
}

// Or is R's "|": TRUE dominates, otherwise NA propagates.
func (t Tri) Or(o Tri) Tri {
	if t == True || o == True {
		return True
	}
	if t == Unknown || o == Unknown {
		return Unknown
	}
	return False
}

// AndNot is the expert system's binary NOT, i.e. R's "&!".
// Special case: Unknown AND NOT True = False (R's NA & !TRUE = FALSE).
func (t Tri) AndNot(o Tri) Tri {
	if t == Unknown && o == True {
		return False
	}
	return t.And(o.not())
}

func (t Tri) not() Tri {
	switch t {
	case True:
		return False
	case False:
		return True
	default:
		return Unknown
	}
}

// IsTrue reports whether this value counts as a match.
func (t Tri) IsTrue() bool { return t == True }

// FromBool lifts a Go bool.
func FromBool(b bool) Tri {
	if b {
		return True
	}
	return False
}
