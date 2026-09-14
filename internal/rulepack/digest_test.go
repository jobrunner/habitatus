package rulepack

import (
	"errors"
	"strings"
	"testing"
)

func TestDigest(t *testing.T) {
	// The empty input's SHA-256 is a fixed, published value: if this changes,
	// the hash is not what it claims to be.
	got, err := Digest(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	const wantEmpty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != wantEmpty {
		t.Errorf("Digest(\"\") = %q, want %q", got, wantEmpty)
	}

	a, err := Digest(strings.NewReader("SECTION 1: Species aggregation\n"))
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if a == wantEmpty {
		t.Error("different input produced the empty digest")
	}
	if len(a) != 64 {
		t.Errorf("digest length = %d, want 64 hex characters", len(a))
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

// A read failure must surface: a digest silently computed over a truncated
// file would identify a rule pack that was never loaded.
func TestDigestReadError(t *testing.T) {
	if _, err := Digest(failingReader{}); err == nil {
		t.Fatal("Digest accepted a failing reader")
	}
}
