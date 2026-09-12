package classify

import (
	"bytes"
	"os"
	"testing"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// benchPlot is a real relevé: a dry sandy grassland from the Tüxen archive,
// the same one the container smoke test uses.
func benchPlot() Request {
	return Request{
		Backbone: DefaultBackbone,
		Records: []taxa.Record{
			{Name: "Festuca ovina", Cover: 30},
			{Name: "Potentilla argentea", Cover: 10},
			{Name: "Dianthus deltoides", Cover: 5},
			{Name: "Viola tricolor aggr.", Cover: 3},
		},
		Header: map[string]string{
			fieldCountry: "Germany", fieldCoast: valueNoCoast, fieldDunes: valueNoDunes,
			fieldEcoreg: "664", fieldAltitude: "1", fieldLat: "53.06", fieldLon: "10.6",
		},
	}
}

// The per-request path: name resolution, cover merging and 312 rules evaluated
// against the real rule file. This is what a client waits for.
func BenchmarkClassifyRealFile(b *testing.B) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		b.Skip("ESY_FILE not set")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		b.Fatal(err)
	}
	pack, err := rulepack.Load(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	svc := NewService(pack, nil, map[string]string{"rulepack": "bench"}, esy.Repaired)
	req := benchPlot()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := svc.Classify(req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateHeader(b *testing.B) {
	h := benchPlot().Header
	for b.Loop() {
		if err := ValidateHeader(h); err != nil {
			b.Fatal(err)
		}
	}
}
