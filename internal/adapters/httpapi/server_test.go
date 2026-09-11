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
