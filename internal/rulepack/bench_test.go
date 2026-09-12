package rulepack

import (
	"os"
	"strings"
	"testing"
)

// Parsing the rule file happens once per start, but it is 8 MB and 165,260
// lines: a regression here shows up directly as start-up latency, which
// matters for a container that is rescheduled often.
func BenchmarkLoadRealFile(b *testing.B) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		b.Skip("ESY_FILE not set")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := Load(strings.NewReader(string(data))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseExpr(b *testing.B) {
	const raw = "#SC Wet-meadows GR #SC Dry-meadows"
	for b.Loop() {
		if _, err := ParseExpr(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFormula(b *testing.B) {
	const raw = "(<#TC Shrubs GR 25> NOT (<#TC Trees GR 25> OR <#TC Native-light-canopy-trees GR 15>)) AND <$$C Dunes_Bohn EQ Y_DUNES>"
	for b.Loop() {
		if _, err := ParseFormula(raw); err != nil {
			b.Fatal(err)
		}
	}
}
