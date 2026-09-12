package mcpapi

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jobrunner/habitatus/internal/classify"
)

// The adapter must not assume every error from Classify is the caller's
// fault: an internal failure reported as invalid params tells the client to
// fix a request that was never wrong.
func TestRPCCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation error", fmt.Errorf("bad country: %w", classify.ErrInvalidRequest), codeInvalidParams},
		{"internal error", errors.New("backbone table unreadable"), codeInternalError},
	}
	for _, c := range cases {
		if got := rpcCodeFor(c.err); got != c.want {
			t.Errorf("%s: rpcCodeFor(%v) = %d, want %d", c.name, c.err, got, c.want)
		}
	}
}
