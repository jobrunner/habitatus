// Package mcpapi exposes the classification service as a single MCP tool
// over stdio. The protocol surface is small enough — one tool, three
// methods — that it is implemented directly against JSON-RPC 2.0 rather
// than pulling in an MCP SDK, keeping the no-external-dependencies
// constraint on this module intact.
package mcpapi

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// Server speaks a minimal MCP subset over stdio: initialize, tools/list and
// tools/call for the single tool "classify".
type Server struct{ svc *classify.Service }

// NewServer builds an MCP server around the classification service.
func NewServer(s *classify.Service) *Server { return &Server{svc: s} }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 reserved error codes (https://www.jsonrpc.org/specification#error_object).
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// recordJSON is the wire shape of one taxon observation. It is a transport
// DTO deliberately kept separate from taxa.Record, following the same
// pattern httpapi uses for recordJSON/matchJSON/stepJSON: JSON tags are a
// transport concern and do not belong on a domain type.
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

type classifyResultJSON struct {
	Result     string            `json:"result"`
	Matches    []matchJSON       `json:"matches"`
	Resolution []stepJSON        `json:"resolution"`
	Versions   map[string]string `json:"versions"`
}

type classifyArgumentsJSON struct {
	Backbone string            `json:"backbone"`
	Records  []recordJSON      `json:"records"`
	Header   map[string]string `json:"header"`
}

// classifyTool describes the classify tool for tools/list. The eight header
// fields have specific vocabularies that a caller cannot guess, so every
// enumerable one is spelled out here rather than left as a bare string —
// Coast_EEA's six values, Dunes_Bohn's two, and the full 52-country ESy name
// list (English names, not ISO codes or local-language names). The enums
// come from classify.CountryNames/CoastValues/DuneValues, the same
// vocabularies ValidateHeader enforces, so this schema cannot silently
// drift from what a call will actually accept. Ecoreg and the numeric
// fields are described precisely instead. Dataset is optional and free
// text.
var classifyTool = map[string]any{
	"name": "classify",
	"description": "Assign EUNIS habitats to a vegetation plot from its species list, " +
		"percentage covers and eight plot header fields, following the RESY/ESy expert system.",
	"inputSchema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"backbone": map[string]any{
				"type":        "string",
				"description": "Name of the taxonomic backbone to resolve species names against, e.g. \"euro+med\".",
			},
			"records": map[string]any{
				"type":        "array",
				"description": "The plot's species list.",
				"minItems":    1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Taxon name as it appears in the backbone, compared exactly (no case folding, no fuzzy matching).",
						},
						"cover": map[string]any{
							"type":             "number",
							"description":      "Percentage cover of this taxon in the plot, 0 < cover <= 100.",
							"exclusiveMinimum": 0,
							"maximum":          100,
						},
					},
					"required": []string{"name", "cover"},
				},
			},
			"header": map[string]any{
				"type":        "object",
				"description": "Plot header. Country, Coast_EEA, Dunes_Bohn, Ecoreg, \"Altitude (m)\", DEG_LAT and DEG_LON are required; Dataset is optional free text.",
				"properties": map[string]any{
					"Country": map[string]any{
						"type":        "string",
						"description": "Exact English ESy country name (not an ISO code, not a local-language name).",
						"enum":        classify.CountryNames(),
					},
					"Coast_EEA": map[string]any{
						"type": "string",
						"enum": classify.CoastValues(),
					},
					"Dunes_Bohn": map[string]any{
						"type": "string",
						"enum": classify.DuneValues(),
					},
					"Ecoreg": map[string]any{
						"type":        "string",
						"description": "Integer ECO_ID of the WWF terrestrial ecoregion the plot lies in, given as a string.",
						"pattern":     `^-?[0-9]+$`,
					},
					"Altitude (m)": map[string]any{
						"type":        "string",
						"description": "Plot altitude in metres above sea level, given as a string.",
					},
					"DEG_LAT": map[string]any{
						"type":        "string",
						"description": "Latitude in decimal degrees, given as a string, -90 to 90.",
					},
					"DEG_LON": map[string]any{
						"type":        "string",
						"description": "Longitude in decimal degrees, given as a string, -180 to 180.",
					},
					"Dataset": map[string]any{
						"type":        "string",
						"description": "Optional free-text dataset identifier.",
					},
				},
				"required": []string{"Country", "Coast_EEA", "Dunes_Bohn", "Ecoreg", "Altitude (m)", "DEG_LAT", "DEG_LON"},
			},
		},
		"required": []string{"backbone", "records", "header"},
	},
}

// nullID is the "id" JSON-RPC 2.0 requires on an error response when the
// request's own id could not be determined — here, when the line did not
// even parse as JSON.
var nullID = json.RawMessage("null")

// Serve reads newline-delimited JSON-RPC 2.0 requests from in and writes
// one response line per request to out, until in is exhausted or a write
// fails — with one exception mandated by the spec: a request with no "id"
// member is a notification, and a notification gets no response at all,
// successful or not.
//
// Neither an unparseable line nor a Classify error (a bad header, an
// unknown backbone, an invalid cover — a caller mistake, not a server
// fault) stops the loop; both come back as ordinary JSON-RPC error
// responses (except when the erroring request was itself a notification).
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// The line does not parse at all, so whether it was a
			// notification cannot be known. The spec's answer is a
			// parse-error response carrying id: null.
			if err := enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      nullID,
				Error:   &rpcError{Code: codeParseError, Message: "parse error: " + err.Error()},
			}); err != nil {
				return err
			}
			continue
		}
		if req.ID == nil {
			// A request with no "id" member is a notification. Dispatch it
			// for any side effect a recognised method might have, but never
			// respond — not even with an error — regardless of outcome.
			s.dispatch(req)
			continue
		}
		if err := enc.Encode(s.dispatch(req)); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) dispatch(req rpcRequest) rpcResponse {
	res := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "habitatus", "version": "0.1.0"},
		}
	case "tools/list":
		res.Result = map[string]any{"tools": []any{classifyTool}}
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			res.Error = err
			return res
		}
		res.Result = result
	case "":
		res.Error = &rpcError{Code: codeInvalidRequest, Message: "missing method"}
	default:
		res.Error = &rpcError{Code: codeMethodNotFound, Message: "unknown method " + req.Method}
	}
	return res
}

func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string                `json:"name"`
		Arguments classifyArgumentsJSON `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	if p.Name != "classify" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool " + p.Name}
	}

	recs := make([]taxa.Record, len(p.Arguments.Records))
	for i, r := range p.Arguments.Records {
		recs[i] = taxa.Record{Name: r.Name, Cover: r.Cover}
	}

	out, err := s.svc.Classify(classify.Request{
		Records:  recs,
		Backbone: p.Arguments.Backbone,
		Header:   p.Arguments.Header,
	})
	if err != nil {
		// A validation error from Classify — a misspelled header value, an
		// out-of-range cover, an unknown backbone, an empty species list —
		// is an answer to the caller, not a broken request.
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}

	result := classifyResultJSON{Result: out.Result, Versions: out.Versions}
	for _, m := range out.Matches {
		result.Matches = append(result.Matches, matchJSON{Code: m.Code, Variant: m.Variant, Priority: m.Priority})
	}
	for _, st := range out.Resolution {
		result.Resolution = append(result.Resolution, stepJSON{
			Input:         st.Input,
			AfterBackbone: st.AfterBackbone,
			Final:         st.Final,
			Resolved:      st.Resolved,
		})
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": string(body)}},
	}, nil
}
