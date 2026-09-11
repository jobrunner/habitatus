// Package httpapi exposes the classification service over HTTP.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// maxRequestBytes caps the size of a classify request body. A real
// vegetation plot has a few hundred taxa at most; this limit is generous by
// several orders of magnitude while still ruling out an unbounded body being
// buffered and decoded, which would otherwise let any caller exhaust memory.
const maxRequestBytes = 2 << 20 // 2 MiB

type classifyRequest struct {
	Backbone string            `json:"backbone"`
	Records  []recordJSON      `json:"records"`
	Header   map[string]string `json:"header"`
}

type recordJSON struct {
	Name  string  `json:"name"`
	Cover float64 `json:"cover"`
}

type matchJSON struct {
	Code     string `json:"code"`
	Variant  string `json:"variant,omitempty"`
	Priority int    `json:"priority"`
}

type stepJSON struct {
	Input         string `json:"input"`
	AfterBackbone string `json:"after_backbone"`
	Final         string `json:"final"`
	Resolved      bool   `json:"resolved"`
}

type classifyResponse struct {
	Result     string            `json:"result"`
	Matches    []matchJSON       `json:"matches"`
	Resolution []stepJSON        `json:"resolution"`
	Versions   map[string]string `json:"versions"`
}

// NewServer wires the HTTP routes: POST /api/v1/classify and GET
// /health/ready. Every error that Service.Classify or JSON decoding raises is
// the caller's fault, not the server's, and is reported as 400.
func NewServer(s *classify.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/classify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)

		var req classifyRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusBadRequest, "request body too large, must not exceed 2 MiB")
				return
			}
			// The decoder's own error names internal Go types
			// (httpapi.recordJSON, json field names) that a caller cannot
			// act on, so it is not passed through.
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		recs := make([]taxa.Record, len(req.Records))
		for i, rr := range req.Records {
			recs[i] = taxa.Record{Name: rr.Name, Cover: rr.Cover}
		}

		res, err := s.Classify(classify.Request{
			Records:  recs,
			Backbone: req.Backbone,
			Header:   req.Header,
		})
		if err != nil {
			// Everything Classify rejects — a misspelled header value, an
			// out-of-range cover, an unknown backbone, an empty species
			// list — is the caller's fault, not the server's.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		out := classifyResponse{Result: res.Result, Versions: res.Versions}
		for _, m := range res.Matches {
			out.Matches = append(out.Matches, matchJSON{Code: m.Code, Variant: m.Variant, Priority: m.Priority})
		}
		for _, st := range res.Resolution {
			out.Resolution = append(out.Resolution, stepJSON{
				Input:         st.Input,
				AfterBackbone: st.AfterBackbone,
				Final:         st.Final,
				Resolved:      st.Resolved,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
