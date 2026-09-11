package classify

import (
	"errors"
	"testing"

	"github.com/jobrunner/habitatus/internal/taxa"
)

// Every way Classify can reject a request must be recognisable as the caller's
// fault, so the adapters can tell it apart from an internal failure.
func TestClassifyErrorsAreInvalidRequests(t *testing.T) {
	s := newTestService(t)
	good := []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}}
	badHeader := validHeader()
	badHeader["Country"] = "Deutschland"

	cases := []struct {
		name string
		req  Request
	}{
		{"empty species list", Request{Backbone: "euro+med", Header: validHeader()}},
		{"cover out of range", Request{
			Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 101}},
			Backbone: "euro+med", Header: validHeader(),
		}},
		{"invalid header", Request{Records: good, Backbone: "euro+med", Header: badHeader}},
		{"unknown backbone", Request{Records: good, Backbone: "wcvp", Header: validHeader()}},
	}
	for _, c := range cases {
		_, err := s.Classify(c.req)
		if err == nil {
			t.Errorf("%s: want an error", c.name)
			continue
		}
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: %v does not report as ErrInvalidRequest", c.name, err)
		}
	}
}

// The message stays the caller's message: wrapping must not prefix it.
func TestValidationErrorKeepsItsMessage(t *testing.T) {
	err := invalidf("Country %q is not an ESy country name", "Deutschland")
	if want := `Country "Deutschland" is not an ESy country name`; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if errors.Is(errors.New("boom"), ErrInvalidRequest) {
		t.Error("a plain error must not report as ErrInvalidRequest")
	}
}
