package httpapi

import (
	"net/http"

	"github.com/jobrunner/habitatus/internal/classify"
)

// metricsResponse is the wire shape of GET /metrics: the two operational
// figures that carry ecological meaning (see classify.Stats) — how often
// the answer is "?" or "+", and which rules never fire, split into a
// static, structural list (Unreachable) and the operationally interesting
// one (NeverFired).
type metricsResponse struct {
	Total         int      `json:"total"`
	Question      int      `json:"question"`
	Plus          int      `json:"plus"`
	QuestionShare float64  `json:"question_share"`
	PlusShare     float64  `json:"plus_share"`
	Unreachable   []string `json:"unreachable_rules"`
	NeverFired    []string `json:"never_fired_rules"`
}

// metricsHandler serves GET /metrics with the counters accumulated over
// every call to Service.Classify so far in this process's lifetime.
func metricsHandler(s *classify.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		st := s.Stats()
		out := metricsResponse{
			Total:       st.Total,
			Question:    st.Question,
			Plus:        st.Plus,
			Unreachable: st.Unreachable,
			NeverFired:  st.NeverFired,
		}
		if st.Total > 0 {
			out.QuestionShare = float64(st.Question) / float64(st.Total)
			out.PlusShare = float64(st.Plus) / float64(st.Total)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
