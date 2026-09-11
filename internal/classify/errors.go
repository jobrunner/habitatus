package classify

import (
	"errors"
	"fmt"
)

// ErrInvalidRequest marks an error that is the caller's fault: a malformed
// species list, a cover outside (0, 100], an unknown backbone id, a header
// field the rules cannot compare against.
//
// Every error Classify returns today is of that kind, and both adapters used
// to assume it — httpapi answered 400 and mcpapi -32602 unconditionally. That
// assumption is invisible and would silently misreport the first internal
// error ever added to Classify as the caller's mistake. Adapters must branch
// on errors.Is(err, ErrInvalidRequest) instead, so an internal error surfaces
// as 5xx / -32603 by default.
var ErrInvalidRequest = errors.New("invalid request")

// invalidf builds a validation error. The message is passed through verbatim —
// it is written for the caller — while errors.Is reports ErrInvalidRequest.
func invalidf(format string, a ...any) error {
	return validationError{err: fmt.Errorf(format, a...)}
}

type validationError struct{ err error }

func (e validationError) Error() string { return e.err.Error() }

func (e validationError) Unwrap() error { return e.err }

func (e validationError) Is(target error) bool { return target == ErrInvalidRequest }
