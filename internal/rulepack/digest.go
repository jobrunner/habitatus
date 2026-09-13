package rulepack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Digest is the SHA-256 of a rule file, hex-encoded. It is what actually
// identifies a rule pack — the file name is an operational detail, and two
// deployments claiming the same version mean nothing if their bytes differ.
// It appears in `versions` on every response and is what the golden masters'
// staleness guard compares against.
func Digest(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("digest rule file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
