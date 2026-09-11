package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jobrunner/habitatus/internal/classify"
)

// The adapter must not assume every error from Classify is the caller's
// fault: an internal failure reported as 400 tells the caller to fix a
// request that was never wrong and never shows up as a 5xx.
func TestStatusFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation error", fmt.Errorf("bad country: %w", classify.ErrInvalidRequest), http.StatusBadRequest},
		{"internal error", errors.New("backbone table unreadable"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		if got := statusFor(c.err); got != c.want {
			t.Errorf("%s: statusFor(%v) = %d, want %d", c.name, c.err, got, c.want)
		}
	}
}
