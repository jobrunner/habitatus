package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/rulepack"
)

const tinyPack = `SECTION 1: Species aggregation
SECTION 1: End
SECTION 2: Species groups
### Trees
     Fagus sylvatica
SECTION 2: End
SECTION 3: Group definitions

4          T1H  Broadleaved deciduous plantation
<#TC Trees GR 5>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func testServer(t *testing.T) http.Handler {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(classify.NewService(pack, nil, map[string]string{"rulepack": "test"}))
}

const goodBody = `{
  "backbone": "euro+med",
  "records": [{"name": "Fagus sylvatica", "cover": 30}],
  "header": {"Country": "Germany", "Coast_EEA": "N_COAST", "Dunes_Bohn": "N_DUNES",
             "Ecoreg": "664", "Altitude (m)": "250", "DEG_LAT": "49.79", "DEG_LON": "9.93"}
}`

func TestClassifyEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(goodBody))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var got struct {
		Result  string `json:"result"`
		Matches []struct {
			Code string `json:"code"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result != "T1H" {
		t.Errorf("result = %q, want T1H", got.Result)
	}
}

func TestClassifyEndpointRejectsBadHeader(t *testing.T) {
	body := strings.Replace(goodBody, `"Germany"`, `"Deutschland"`, 1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(body))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("error body must be JSON: %v (%s)", err, rec.Body)
	}
	if got["error"] == "" {
		t.Errorf("error body must carry a message, got %v", got)
	}
}

func TestClassifyEndpointRejectsGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/classify", nil)
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Errorf("Allow header = %q, want %q", got, http.MethodPost)
	}
}

func TestClassifyEndpointRejectsOversizedBody(t *testing.T) {
	huge := `{"backbone": "euro+med", "records": [{"name": "` +
		strings.Repeat("x", 3<<20) + `", "cover": 30}], "header": {}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(huge))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("error body must be JSON: %v (%s)", err, rec.Body)
	}
	if !strings.Contains(got["error"], "too large") {
		t.Errorf("error = %q, want a message mentioning the body is too large", got["error"])
	}
}

func TestClassifyEndpointRejectsMalformedJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(`{not json`))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("error body must be JSON: %v (%s)", err, rec.Body)
	}
	if strings.Contains(got["error"], "httpapi.") || strings.Contains(got["error"], "recordJSON") {
		t.Errorf("error message leaks internal Go type names: %q", got["error"])
	}
}

func TestClassifyEndpointRejectsEmptyRecords(t *testing.T) {
	body := `{"backbone": "euro+med", "records": [], "header": {}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(body))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestClassifyEndpointRejectsBadCover(t *testing.T) {
	body := `{"backbone": "euro+med", "records": [{"name": "Fagus sylvatica", "cover": 150}],
	  "header": {"Country": "Germany", "Coast_EEA": "N_COAST", "Dunes_Bohn": "N_DUNES",
	             "Ecoreg": "664", "Altitude (m)": "250", "DEG_LAT": "49.79", "DEG_LON": "9.93"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(body))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestClassifyEndpointRejectsUnknownBackbone(t *testing.T) {
	body := strings.Replace(goodBody, `"euro+med"`, `"nonexistent"`, 1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(body))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestClassifyEndpointReportsTruncatedAt10(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(goodBody))
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["truncated_at_10"]; !ok {
		t.Errorf("response is missing truncated_at_10: %v", got)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := testServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/classify", strings.NewReader(goodBody))
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("classify status = %d, body = %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", rec.Code, rec.Body)
	}

	var got struct {
		Total         int      `json:"total"`
		Question      int      `json:"question"`
		Plus          int      `json:"plus"`
		QuestionShare float64  `json:"question_share"`
		PlusShare     float64  `json:"plus_share"`
		Unreachable   []string `json:"unreachable_rules"`
		NeverFired    []string `json:"never_fired_rules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 {
		t.Errorf("total = %d, want 1", got.Total)
	}
	if got.Unreachable == nil {
		t.Errorf("unreachable_rules must be present, even if empty, got nil")
	}
	// tinyPack has exactly one rule, T1H, and the classify call above made
	// it fire, so NeverFired must be an empty slice, not nil — and must
	// therefore serialise as "[]", not "null". A strict client should not
	// have to treat the two cases differently.
	if got.NeverFired == nil {
		t.Errorf("never_fired_rules must be present, even if empty, got nil")
	}
	if strings.Contains(rec.Body.String(), `"never_fired_rules":null`) {
		t.Errorf("never_fired_rules must serialise as [], not null, when every reachable rule has fired: %s", rec.Body)
	}
}

func TestMetricsEndpointRejectsPost(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestReadyEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestReadyEndpointRejectsPost(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health/ready", nil)
	testServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow header = %q, want %q", got, http.MethodGet)
	}
}
