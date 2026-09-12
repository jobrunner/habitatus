package mcpapi

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/esy"
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

func testServer(t *testing.T) *Server {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(classify.NewService(pack, nil, map[string]string{"rulepack": "test"}, esy.Repaired))
}

func decodeResponses(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var got []map[string]any
	dec := json.NewDecoder(out)
	for {
		var v map[string]any
		if err := dec.Decode(&v); err != nil {
			break
		}
		got = append(got, v)
	}
	return got
}

func TestToolsListAdvertisesClassify(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"classify"`) {
		t.Errorf("tools/list did not advertise classify: %s", out.String())
	}
}

func TestToolsListSchemaEnumeratesHeaderVocabularies(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	body, err := json.Marshal(resps[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"N_COAST", "ARC_COAST", "ATL_COAST", "BAL_COAST", "BLA_COAST", "MED_COAST",
		"Y_DUNES", "N_DUNES",
		"Germany", "United Kingdom",
		"Ecoreg", "DEG_LAT", "DEG_LON", "Altitude (m)", "Dataset",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("tools/list schema missing %q: %s", want, s)
		}
	}
}

func TestToolsCallClassify(t *testing.T) {
	call := map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{
			"name": "classify",
			"arguments": map[string]any{
				"backbone": "euro+med",
				"records":  []map[string]any{{"name": "Fagus sylvatica", "cover": 30}},
				"header": map[string]string{
					"Country": "Germany", "Coast_EEA": "N_COAST", "Dunes_Bohn": "N_DUNES",
					"Ecoreg": "664", "Altitude (m)": "250", "DEG_LAT": "49.79", "DEG_LON": "9.93",
				},
			},
		},
	}
	body, _ := json.Marshal(call)
	var out bytes.Buffer
	if err := testServer(t).Serve(bytes.NewReader(append(body, '\n')), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "T1H") {
		t.Errorf("classify did not return T1H: %s", out.String())
	}
}

func TestUnknownMethodReturnsError(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"nope"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"error"`) {
		t.Errorf("unknown method must produce an error response: %s", out.String())
	}
}

// A validation error from Classify — a misspelled country, an unknown
// backbone — is an answer to the caller, not a broken request: it must come
// back as a JSON-RPC error object, and the server must not stop.
func TestToolsCallClassifyValidationErrorIsResultNotFailure(t *testing.T) {
	call := map[string]any{
		"jsonrpc": "2.0", "id": 4, "method": "tools/call",
		"params": map[string]any{
			"name": "classify",
			"arguments": map[string]any{
				"backbone": "euro+med",
				"records":  []map[string]any{{"name": "Fagus sylvatica", "cover": 30}},
				"header": map[string]string{
					"Country": "Deutschland", "Coast_EEA": "N_COAST", "Dunes_Bohn": "N_DUNES",
					"Ecoreg": "664", "Altitude (m)": "250", "DEG_LAT": "49.79", "DEG_LON": "9.93",
				},
			},
		},
	}
	body, _ := json.Marshal(call)
	next := `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`
	var out bytes.Buffer
	in := string(body) + "\n" + next + "\n"
	if err := testServer(t).Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 2 {
		t.Fatalf("want 2 responses (validation error must not stop the loop), got %d: %s", len(resps), out.String())
	}
	if resps[0]["error"] == nil {
		t.Errorf("validation error must be a JSON-RPC error object, got %v", resps[0])
	}
	if resps[0]["result"] != nil {
		t.Errorf("a rejected request must not also carry a result, got %v", resps[0])
	}
	if resps[1]["result"] == nil {
		t.Errorf("the request after a validation error must still be served, got %v", resps[1])
	}
}

// An unparseable line must not kill the loop either.
func TestUnparseableLineDoesNotStopTheLoop(t *testing.T) {
	in := "not json at all\n" + `{"jsonrpc":"2.0","id":9,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := testServer(t).Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 2 {
		t.Fatalf("want 2 responses (parse error must not stop the loop), got %d: %s", len(resps), out.String())
	}
	if resps[0]["error"] == nil {
		t.Errorf("unparseable line must produce a JSON-RPC error object, got %v", resps[0])
	}
	if resps[1]["result"] == nil {
		t.Errorf("the request after a parse error must still be served, got %v", resps[1])
	}
}

func TestInitializeRespondsWithServerInfo(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"habitatus"`) {
		t.Errorf("initialize must report server info: %s", out.String())
	}
}

func TestUnparseableLineCarriesNullID(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("not json at all\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d: %s", len(resps), out.String())
	}
	idVal, ok := resps[0]["id"]
	if !ok {
		t.Errorf("a parse-error response must carry an \"id\" member (null), got %v", resps[0])
	}
	if idVal != nil {
		t.Errorf("a parse-error response's id must be null, got %v", idVal)
	}
}

// A JSON-RPC request with no "id" member is a notification and must get no
// response at all, per spec — not even an error.
func TestNotificationGetsNoResponse(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("a notification must produce no output, got %s", out.String())
	}
}

// A notification must not stop the loop: the request after it is still
// served normally.
func TestNotificationDoesNotStopTheLoop(t *testing.T) {
	notification := `{"jsonrpc":"2.0","method":"tools/list"}`
	ordinary := `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`
	in := strings.NewReader(notification + "\n" + ordinary + "\n")
	var out bytes.Buffer
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want exactly 1 response (the notification produces none), got %d: %s", len(resps), out.String())
	}
	if resps[0]["result"] == nil {
		t.Errorf("the ordinary request after a notification must still be served, got %v", resps[0])
	}
}

// A notification with an unrecognised method also gets no response — the
// no-response rule for notifications holds regardless of outcome.
func TestUnknownMethodNotificationGetsNoResponse(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("an unrecognised notification must still produce no output, got %s", out.String())
	}
}

// A bare JSON "null" and an empty object "{}" both decode to a zero
// rpcRequest: no id, no method. They are not notifications — a
// notification is still a request, and every request carries a method —
// so they must not be swallowed silently. JSON-RPC 2.0 calls an object
// with no method an invalid request: -32600, with id: null.
func TestNullLineIsInvalidRequestNotNotification(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("null\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d: %s", len(resps), out.String())
	}
	assertInvalidRequestWithNullID(t, resps[0])
}

func TestEmptyObjectLineIsInvalidRequestNotNotification(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("{}\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d: %s", len(resps), out.String())
	}
	assertInvalidRequestWithNullID(t, resps[0])
}

func assertInvalidRequestWithNullID(t *testing.T, resp map[string]any) {
	t.Helper()
	idVal, ok := resp["id"]
	if !ok {
		t.Errorf("must carry an \"id\" member (null), got %v", resp)
	}
	if idVal != nil {
		t.Errorf("id must be null, got %v", idVal)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("must carry a JSON-RPC error object, got %v", resp)
	}
	code, _ := errObj["code"].(float64)
	if code != -32600 {
		t.Errorf("error code = %v, want -32600 (invalid request)", errObj["code"])
	}
}

// A well-formed notification (has a method, no id) must remain silent even
// after the null/{} fix — the two cases are distinguished by the presence
// of a method, not by anything else.
func TestWellFormedNotificationStillSilentAfterInvalidRequestFix(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list"}` + "\n")
	if err := testServer(t).Serve(in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("a well-formed notification must still produce no output, got %s", out.String())
	}
}

func TestToolsCallUnknownToolReturnsInvalidParams(t *testing.T) {
	call := map[string]any{
		"jsonrpc": "2.0", "id": 8, "method": "tools/call",
		"params": map[string]any{
			"name":      "not-classify",
			"arguments": map[string]any{},
		},
	}
	body, _ := json.Marshal(call)
	var out bytes.Buffer
	if err := testServer(t).Serve(bytes.NewReader(append(body, '\n')), &out); err != nil {
		t.Fatal(err)
	}
	resps := decodeResponses(t, &out)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown tool must produce a JSON-RPC error object, got %v", resps[0])
	}
	code, _ := errObj["code"].(float64)
	if code != -32602 {
		t.Errorf("unknown tool error code = %v, want -32602 (invalid params)", errObj["code"])
	}
}
