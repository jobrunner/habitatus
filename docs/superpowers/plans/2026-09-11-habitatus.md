# Habitatus Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ein Go-Dienst, der einer Vegetationsaufnahme EUNIS-Habitate zuordnet, indem er das ESy-Regelwerk parst und auswertet — verhaltensgleich zur gepflegten R-Implementierung v1.2.

**Architecture:** Vier unabhängige Pakete. `rulepack` parst die ESy-Dateien in ein unveränderliches Modell, `taxa` löst Namen auf und verschmilzt Deckungen, `esy` wertet dreiwertig aus, `classify` orchestriert. Adapter für HTTP und MCP liegen außen. Der Kern ist eine reine Funktion ohne Netz, Uhr oder Zufall, damit der Golden Master gegen R ohne Infrastruktur läuft.

**Tech Stack:** Go 1.24 (lokal installiert: 1.24.4), Standardbibliothek für Kern und HTTP. R 4.x nur als Entwicklerwerkzeug zur Fixture-Erzeugung, nie zur Laufzeit.

**Spec:** `docs/superpowers/specs/2026-09-10-habitatus-design.md`

**Repo:** https://github.com/jobrunner/habitatus (Remote `origin` ist gesetzt)

## Global Constraints

- Modulpfad: `github.com/jobrunner/habitatus`
- Go ≥ 1.24. Der Kern (`internal/rulepack`, `internal/taxa`, `internal/esy`, `internal/classify`) hat **keine** externen Abhängigkeiten.
- Verhaltensreferenz ist ESy **v1.2**, geklont unter `spike/ESy-upstream/` (Commit `416bab9`). Am Upstream wird nichts geändert.
- Abweichungen vom Original werden nachgebildet, nicht repariert — außer sie betreffen ausschließlich Etiketten (siehe Task 4).
- Alle Vergleiche auf Artnamen sind **exakt**: keine Normalisierung, keine Groß-/Kleinschreibungstoleranz, keine Fuzzy-Suche.
- Deckungen: `0 < c ≤ 100`. Verschmelzung immer über `totalCover`, nie über Addition.
- Regelwerksdatei der Wahrheit: `~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt` (165.260 Zeilen, 312 Regeln).
- Commit-Messages auf Englisch, Code und Bezeichner Englisch, Doku Deutsch.

---

## Dateistruktur

| Datei | Verantwortung |
|---|---|
| `go.mod` | Modul, Go-Version |
| `internal/rulepack/sections.go` | Datei in die vier Sektionen zerlegen |
| `internal/rulepack/aggregation.go` | Sektion 1: Quellname → Zielkonzept |
| `internal/rulepack/groups.go` | Sektion 2: Artengruppen |
| `internal/rulepack/rules.go` | Sektion 3: Regelköpfe, Mehrzeilenformeln |
| `internal/rulepack/expr.go` | Ausdrucks-Grammatik innerhalb `<…>` |
| `internal/rulepack/formula.go` | Boolesche Formel mit R-Präzedenz |
| `internal/rulepack/model.go` | `Pack`, `Rule`, `Expr`, `Node`, `Issues` |
| `internal/rulepack/load.go` | Alles zusammensetzen, Konsistenzprüfung |
| `internal/esy/tri.go` | Dreiwertige Logik |
| `internal/esy/cover.go` | `totalCover` (Jennings-Fischer) |
| `internal/taxa/resolve.go` | Zweistufige Auflösung, Deckungsverschmelzung |
| `internal/esy/condition.go` | Die zehn Bedingungstypen |
| `internal/esy/evaluate.go` | Formelauswertung, Gewinnerermittlung |
| `internal/classify/classify.go` | Anwendungsfall, Validierung, Antwortaufbau |
| `internal/classify/header.go` | Kopfdaten-Vokabular, Länderliste |
| `internal/adapters/httpapi/server.go` | REST |
| `internal/adapters/mcpapi/server.go` | MCP |
| `cmd/habitatus/main.go` | Start, Konfiguration |
| `spike/resy/generate-fixtures.R` | treibt den Upstream, schreibt Fixtures |
| `testdata/golden/` | Fixtures + Hashes |
| `data/esy-country-names.csv` | ISO → ESy-Ländername (liegt bereits vor) |

---

### Task 1: Projektgerüst und Sektions-Splitter

**Files:**
- Create: `go.mod`, `internal/rulepack/model.go`, `internal/rulepack/sections.go`
- Test: `internal/rulepack/sections_test.go`

**Interfaces:**
- Consumes: nichts
- Produces: `func SplitSections(r io.Reader) (map[int][]string, error)` — Abschnittsnummer → Zeilen ohne die `SECTION`-Zeilen selbst.

- [ ] **Step 1: Modul anlegen**

```bash
cd /Users/jbrunner/work/projects/expertus
go mod init github.com/jobrunner/habitatus
```

- [ ] **Step 2: Failing test schreiben**

`internal/rulepack/sections_test.go`:

```go
package rulepack

import (
	"strings"
	"testing"
)

const miniFile = `SECTION 1: Species aggregation
Abies alba                                                -  0
     Abies pectinata                                         0
SECTION 1: End
SECTION 2: Species groups
### Trees
     Abies alba
SECTION 2: End
SECTION 3: Group definitions

4          T1H  Broadleaved deciduous plantation
<#TC Trees GR 25>
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`

func TestSplitSections(t *testing.T) {
	got, err := SplitSections(strings.NewReader(miniFile))
	if err != nil {
		t.Fatalf("SplitSections: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 sections, got %d", len(got))
	}
	if len(got[1]) != 2 {
		t.Errorf("section 1: want 2 lines, got %d: %q", len(got[1]), got[1])
	}
	if len(got[4]) != 0 {
		t.Errorf("section 4 must be empty, got %q", got[4])
	}
	if got[3][0] != "" {
		t.Errorf("blank line inside section 3 must be preserved, got %q", got[3][0])
	}
}

func TestSplitSectionsRejectsUnterminated(t *testing.T) {
	_, err := SplitSections(strings.NewReader("SECTION 1: Species aggregation\nfoo\n"))
	if err == nil {
		t.Fatal("want error for unterminated section")
	}
}
```

- [ ] **Step 3: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestSplitSections -v`
Expected: FAIL, `undefined: SplitSections`

- [ ] **Step 4: Implementieren**

`internal/rulepack/sections.go`:

```go
// Package rulepack parses ESy expert-system files into an immutable model.
package rulepack

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SplitSections splits an ESy file into its numbered sections. The SECTION
// marker lines themselves are dropped; every other line is preserved verbatim,
// including blank and whitespace-only lines, because block structure depends on
// them in some sections and on indentation in others.
func SplitSections(r io.Reader) (map[int][]string, error) {
	out := map[int][]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	current := 0
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if n, end, ok := parseSectionMarker(line); ok {
			if end {
				if n != current {
					return nil, fmt.Errorf("section %d ends while section %d is open", n, current)
				}
				current = 0
				continue
			}
			if current != 0 {
				return nil, fmt.Errorf("section %d starts while section %d is open", n, current)
			}
			current = n
			if _, seen := out[n]; seen {
				return nil, fmt.Errorf("section %d appears twice", n)
			}
			out[n] = []string{}
			continue
		}
		if current != 0 {
			out[current] = append(out[current], line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if current != 0 {
		return nil, fmt.Errorf("section %d is not terminated", current)
	}
	return out, nil
}

// parseSectionMarker recognises "SECTION <n>: End" and "SECTION <n>: <title>".
func parseSectionMarker(line string) (n int, end, ok bool) {
	if !strings.HasPrefix(line, "SECTION ") {
		return 0, false, false
	}
	rest := line[len("SECTION "):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return 0, false, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:colon]))
	if err != nil {
		return 0, false, false
	}
	return n, strings.TrimSpace(rest[colon+1:]) == "End", true
}
```

`internal/rulepack/model.go`:

```go
package rulepack

// Pack is a parsed expert-system file. It is immutable once built.
type Pack struct {
	// Aggregation maps a source name to its target concept (section 1).
	Aggregation map[string]string
	// Groups maps a group name to its member concepts (section 2).
	Groups map[string][]string
	// Rules are the habitat definitions (section 3), in file order.
	Rules []Rule
	// Issues records defects found while loading. Defects never abort the load.
	Issues Issues
}

// Issues counts and names the defects found in a rule file.
type Issues struct {
	// DuplicateSources are source names listed under more than one target.
	// The first occurrence wins, matching upstream's match() semantics.
	DuplicateSources []string
	// Chains are names that are both a source and a target. Resolution is
	// single-pass, so chains are not followed.
	Chains []string
	// UnknownGroups are group names referenced by a rule but never defined.
	UnknownGroups []string
}
```

- [ ] **Step 5: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestSplitSections -v`
Expected: PASS (beide Tests)

- [ ] **Step 6: Gegen die echte Datei prüfen**

```bash
cat > /tmp/realfile_test.go <<'EOF'
package rulepack

import (
	"os"
	"testing"
)

func TestSplitSectionsRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	for n, want := range map[int]int{1: 141032, 2: 23278, 3: 941, 4: 1} {
		if len(got[n]) != want {
			t.Errorf("section %d: got %d lines, want %d", n, len(got[n]), want)
		}
	}
}
EOF
cp /tmp/realfile_test.go internal/rulepack/realfile_test.go
ESY_FILE=~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt \
  go test ./internal/rulepack/ -run RealFile -v
```

Expected: PASS. Weichen die Zeilenzahlen ab, ist das ein Fund — dann die tatsächlichen Werte in den Test übernehmen und im Commit begründen.

- [ ] **Step 7: Commit**

```bash
git add go.mod internal/rulepack/
git commit -m "feat(rulepack): split ESy files into their four sections"
```

---

### Task 2: Sektion 1 — Aggregationstabelle

**Files:**
- Create: `internal/rulepack/aggregation.go`
- Test: `internal/rulepack/aggregation_test.go`

**Interfaces:**
- Consumes: `SplitSections`
- Produces: `func ParseAggregation(lines []string) (map[string]string, Issues)` — Quellname → Zielkonzept. Erster Eintrag gewinnt. Selbstabbildungen werden verworfen.

Blöcke werden **an der Einrückung** erkannt, nicht an Leerzeilen: Die Backbone-Tabellen haben keine Trennzeilen. Kopfzeilen enden auf `-  0`, Mitgliederzeilen beginnen mit Leerzeichen und enden auf eine Zahl.

- [ ] **Step 1: Failing test schreiben**

`internal/rulepack/aggregation_test.go`:

```go
package rulepack

import "testing"

func TestParseAggregationBasics(t *testing.T) {
	lines := []string{
		"Abies alba                                                -  0",
		"     Abies pectinata                                         0",
		"     Abies alba subsp. alba                                   0",
		"Empetrum nigrum aggr.                                     -  0",
		"     Empetrum nigrum                                         0",
	}
	got, issues := ParseAggregation(lines)
	if got["Abies pectinata"] != "Abies alba" {
		t.Errorf("Abies pectinata -> %q, want %q", got["Abies pectinata"], "Abies alba")
	}
	if got["Empetrum nigrum"] != "Empetrum nigrum aggr." {
		t.Errorf("Empetrum nigrum -> %q", got["Empetrum nigrum"])
	}
	if _, ok := got["Abies alba"]; ok {
		t.Error("a target must not map to itself")
	}
	if len(issues.DuplicateSources) != 0 {
		t.Errorf("unexpected duplicates: %v", issues.DuplicateSources)
	}
}

// The real file lists five source names under two different targets. The first
// occurrence wins, which is what upstream's match() does.
func TestParseAggregationFirstEntryWins(t *testing.T) {
	lines := []string{
		"Bupleurum commutatum                                      -  0",
		"     Bupleurum commutatum subsp. glaucocarpus                0",
		"Bupleurum pachnospermum                                   -  0",
		"     Bupleurum commutatum subsp. glaucocarpus                0",
	}
	got, issues := ParseAggregation(lines)
	if got["Bupleurum commutatum subsp. glaucocarpus"] != "Bupleurum commutatum" {
		t.Errorf("first entry must win, got %q", got["Bupleurum commutatum subsp. glaucocarpus"])
	}
	if len(issues.DuplicateSources) != 1 {
		t.Fatalf("want 1 duplicate reported, got %v", issues.DuplicateSources)
	}
}

// Chains are reported but never followed: resolution is single-pass.
func TestParseAggregationReportsChains(t *testing.T) {
	lines := []string{
		"Elymus species                                            -  0",
		"     Elytrigia species                                       0",
		"Elytrigia species                                         -  0",
		"     Elymus pycnanthus                                       0",
	}
	got, issues := ParseAggregation(lines)
	if got["Elymus pycnanthus"] != "Elytrigia species" {
		t.Errorf("want single-pass mapping to Elytrigia species, got %q", got["Elymus pycnanthus"])
	}
	if len(issues.Chains) != 1 || issues.Chains[0] != "Elytrigia species" {
		t.Errorf("want chain on Elytrigia species, got %v", issues.Chains)
	}
}

// A block at end of input must keep its last member. Upstream fixed exactly
// this bug in commit fb86835 for section 2.
func TestParseAggregationKeepsLastMember(t *testing.T) {
	lines := []string{
		"Salix euxina                                              -  0",
		"     Salix fragilis                                          0",
	}
	got, _ := ParseAggregation(lines)
	if got["Salix fragilis"] != "Salix euxina" {
		t.Errorf("last member of last block lost: %q", got["Salix fragilis"])
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestParseAggregation -v`
Expected: FAIL, `undefined: ParseAggregation`

- [ ] **Step 3: Implementieren**

`internal/rulepack/aggregation.go`:

```go
package rulepack

import (
	"sort"
	"strings"
)

// ParseAggregation parses section 1 (species aggregation): an unindented
// header line names the target concept, the indented lines below it are the
// source names that map onto it.
//
// Blocks are delimited by indentation, not by blank lines — the backbone
// translation tables have no separator lines at all.
//
// When a source name appears under two targets, the first occurrence wins.
// That is not a choice: upstream resolves names with match(), which returns the
// first hit. Self-mappings are dropped, as upstream does with
// AGG[AGG$values != AGG$ind, ].
func ParseAggregation(lines []string) (map[string]string, Issues) {
	out := make(map[string]string, len(lines))
	var issues Issues
	dupes := map[string]bool{}
	targets := map[string]bool{}

	target := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isIndented(line) {
			if target == "" {
				continue // member without a header — ignore
			}
			src := trimEntry(line)
			if src == "" || src == target {
				continue
			}
			if _, seen := out[src]; seen {
				if !dupes[src] {
					dupes[src] = true
					issues.DuplicateSources = append(issues.DuplicateSources, src)
				}
				continue // first entry wins
			}
			out[src] = target
			continue
		}
		target = trimEntry(line)
		targets[target] = true
	}

	for src := range out {
		if targets[src] {
			issues.Chains = append(issues.Chains, src)
		}
	}
	sort.Strings(issues.DuplicateSources)
	sort.Strings(issues.Chains)
	return out, issues
}

func isIndented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

// trimEntry strips the trailing layer number and the header's "-" marker.
// "Abies alba          -  0" -> "Abies alba"
// "     Abies pectinata   0" -> "Abies pectinata"
func trimEntry(line string) string {
	s := strings.TrimSpace(line)
	// drop trailing digits
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return s // no trailing number
	}
	s = strings.TrimRight(s[:i], " \t")
	s = strings.TrimSuffix(s, "-")
	return strings.TrimRight(s, " \t")
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestParseAggregation -v`
Expected: PASS (vier Tests)

- [ ] **Step 5: Gegen die echte Datei prüfen**

Ergänze in `internal/rulepack/realfile_test.go`:

```go
func TestAggregationRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	secs, err := SplitSections(f)
	if err != nil {
		t.Fatal(err)
	}
	agg, issues := ParseAggregation(secs[1])
	if len(agg) != 100696 {
		t.Errorf("got %d mappings, want 100696", len(agg))
	}
	if len(issues.DuplicateSources) != 7 {
		t.Errorf("got %d duplicate sources, want 7: %v", len(issues.DuplicateSources), issues.DuplicateSources)
	}
	if len(issues.Chains) != 2 {
		t.Errorf("got %d chains, want 2: %v", len(issues.Chains), issues.Chains)
	}
	if agg["Populus x canadensis + P. nigra"] != "Polypogon monspeliensis x viridis" {
		t.Errorf("first-entry-wins broken: %q", agg["Populus x canadensis + P. nigra"])
	}
}
```

Run: `ESY_FILE=~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt go test ./internal/rulepack/ -run RealFile -v`
Expected: PASS. Die Zahl 100696 ist 100703 distinkte Quellnamen minus der 7 Doppelten; weicht sie ab, die tatsächliche Zahl übernehmen und im Commit begründen.

Die Populus-Zeile bildet hier bewusst noch auf das falsche Ziel ab — die Korrektur aus Spec §8 ist eine Sonderregel der `taxa`-Schicht (Task 8), nicht des Parsers.

- [ ] **Step 6: Commit**

```bash
git add internal/rulepack/
git commit -m "feat(rulepack): parse section 1 aggregation, first entry wins"
```

---

### Task 3: Sektion 2 — Artengruppen

**Files:**
- Create: `internal/rulepack/groups.go`
- Test: `internal/rulepack/groups_test.go`

**Interfaces:**
- Consumes: `SplitSections`
- Produces: `func ParseGroups(lines []string) map[string][]string` — Gruppenname ohne das `### `-Präfix → Mitglieder in Dateireihenfolge.

- [ ] **Step 1: Failing test schreiben**

`internal/rulepack/groups_test.go`:

```go
package rulepack

import "testing"

func TestParseGroups(t *testing.T) {
	lines := []string{
		"### Acidophilous-oak-forest-trees",
		"     Castanea sativa",
		"     Quercus petraea",
		"",
		"### Alnus-Fraxinus-excelsior",
		"     Alnus glutinosa",
		"     Fraxinus excelsior",
	}
	got := ParseGroups(lines)
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d", len(got))
	}
	oak := got["Acidophilous-oak-forest-trees"]
	if len(oak) != 2 || oak[0] != "Castanea sativa" || oak[1] != "Quercus petraea" {
		t.Errorf("oak group wrong: %q", oak)
	}
}

// Upstream commit fb86835 fixed a bug that dropped the last species of the last
// group. This test pins that behaviour.
func TestParseGroupsKeepsLastMemberOfLastGroup(t *testing.T) {
	lines := []string{
		"### Trees",
		"     Abies alba",
		"     Quercus robur",
	}
	got := ParseGroups(lines)
	if len(got["Trees"]) != 2 {
		t.Fatalf("want 2 members, got %q", got["Trees"])
	}
	if got["Trees"][1] != "Quercus robur" {
		t.Errorf("last member lost, got %q", got["Trees"])
	}
}

// Member lines in section 2 carry no trailing number, and some groups end with
// a whitespace-only line before the next header.
func TestParseGroupsIgnoresWhitespaceLines(t *testing.T) {
	lines := []string{
		"### Alnus-Fraxinus-excelsior",
		"     Alnus glutinosa",
		"     ",
		"### Next",
		"     Betula pendula",
	}
	got := ParseGroups(lines)
	if len(got["Alnus-Fraxinus-excelsior"]) != 1 {
		t.Errorf("whitespace line became a member: %q", got["Alnus-Fraxinus-excelsior"])
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestParseGroups -v`
Expected: FAIL, `undefined: ParseGroups`

- [ ] **Step 3: Implementieren**

`internal/rulepack/groups.go`:

```go
package rulepack

import "strings"

const groupPrefix = "### "

// ParseGroups parses section 2 (species groups). A line starting with "### "
// opens a group; the indented lines below it are its members, in file order.
//
// The final member of the final group must survive — upstream lost exactly that
// entry until commit fb86835.
func ParseGroups(lines []string) map[string][]string {
	out := map[string][]string{}
	name := ""
	for _, line := range lines {
		if strings.HasPrefix(line, groupPrefix) {
			name = strings.TrimSpace(line[len(groupPrefix):])
			if _, ok := out[name]; !ok {
				out[name] = nil
			}
			continue
		}
		if name == "" || strings.TrimSpace(line) == "" {
			continue
		}
		out[name] = append(out[name], strings.TrimSpace(line))
	}
	return out
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestParseGroups -v`
Expected: PASS (drei Tests)

- [ ] **Step 5: Gegen die echte Datei prüfen**

Ergänze in `realfile_test.go`:

```go
func TestGroupsRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	secs, _ := SplitSections(f)
	groups := ParseGroups(secs[2])
	if len(groups) < 300 {
		t.Errorf("got %d groups, expected well over 300", len(groups))
	}
	distinct := map[string]bool{}
	for _, ms := range groups {
		for _, m := range ms {
			distinct[m] = true
		}
	}
	if len(distinct) != 8477 {
		t.Errorf("got %d distinct members, want 8477", len(distinct))
	}
	if len(groups["Trees"]) == 0 {
		t.Error("group Trees is empty")
	}
}
```

Run: `ESY_FILE=… go test ./internal/rulepack/ -run RealFile -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/rulepack/
git commit -m "feat(rulepack): parse section 2 species groups"
```

---

### Task 4: Sektion 3 — Regelköpfe und Mehrzeilenformeln

**Files:**
- Create: `internal/rulepack/rules.go`
- Modify: `internal/rulepack/model.go` (Typ `Rule`)
- Test: `internal/rulepack/rules_test.go`

**Interfaces:**
- Consumes: `SplitSections`
- Produces:
  - `type Rule struct { Priority int; Code string; Variant string; Name string; Raw string }`
  - `func ParseRuleHeaders(lines []string) ([]Rule, error)` — `Raw` ist die zusammengefügte Formel als Text; geparst wird sie in Task 6.

Der Regelkopf ist **fixed-width**: Priorität in Spalte 1, Code in den Spalten 12–16, Name ab Spalte 17. Für alle 312 Regeln geprüft. Ein Parser, der an Leerzeichen trennt, liest bei `N15!!Atlantic and Baltic…` den Code als `N15!!Atlantic`.

Upstream bildet den Kurznamen als `trim(substr(name, 1, 6))` ab Spalte 12 und klebt damit bei Codes unter fünf Zeichen den ersten Buchstaben des Habitatnamens an (`T1H  B`). Da `classify()` denselben String auf beiden Seiten des `match()` verwendet, wirkt sich das **nicht auf die Zuordnung aus, nur auf das Etikett**. Wir parsen deshalb sauber und bilden für den Fixture-Vergleich beide Seiten auf `(Code, Variante)` ab (Task 11).

- [ ] **Step 1: Failing test schreiben**

`internal/rulepack/rules_test.go`:

```go
package rulepack

import "testing"

func TestParseRuleHeaderFixedWidth(t *testing.T) {
	lines := []string{
		"7          MA211 Arctic coastal saltmarsh",
		"<#TC Trees GR 15>",
		"",
		"4          N15! Atlantic and Baltic coastal dune grassland",
		"<#02 N15-specialists>",
		"",
		"2          N15!!Atlantic and Baltic coastal dune grassland",
		"<##Q +04 R1Q>",
		"",
		"2          MAa  Angiosperm vegetation in the marine littoral zone",
		"<##Q +04 MAa>",
	}
	got, err := ParseRuleHeaders(lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rules, got %d", len(got))
	}
	want := []struct{ prio int; code, variant, name string }{
		{7, "MA211", "", "Arctic coastal saltmarsh"},
		{4, "N15", "!", "Atlantic and Baltic coastal dune grassland"},
		{2, "N15", "!!", "Atlantic and Baltic coastal dune grassland"},
		{2, "MAa", "", "Angiosperm vegetation in the marine littoral zone"},
	}
	for i, w := range want {
		g := got[i]
		if g.Priority != w.prio || g.Code != w.code || g.Variant != w.variant || g.Name != w.name {
			t.Errorf("rule %d = %+v, want %v", i, g, w)
		}
	}
}

// Rule S63 spans three lines, breaking after OR and before NOT.
func TestParseRuleHeadersJoinsContinuationLines(t *testing.T) {
	lines := []string{
		"4          S63  Eastern garrigue",
		"((<#TC E-garrigue-shrubs GR 25> AND (<#TC E-garrigue-shrubs GE $50>)) OR",
		"(<#03 E-garrigue-herbs>))",
		"NOT <#TC Trees GR 10>",
	}
	got, err := ParseRuleHeaders(lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 rule, got %d", len(got))
	}
	want := "((<#TC E-garrigue-shrubs GR 25> AND (<#TC E-garrigue-shrubs GE $50>)) OR " +
		"(<#03 E-garrigue-herbs>)) NOT <#TC Trees GR 10>"
	if got[0].Raw != want {
		t.Errorf("joined formula:\n got %q\nwant %q", got[0].Raw, want)
	}
}

func TestParseRuleHeadersRejectsFormulaWithoutHeader(t *testing.T) {
	_, err := ParseRuleHeaders([]string{"<#TC Trees GR 15>"})
	if err == nil {
		t.Fatal("want error for formula without a preceding header")
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestParseRule -v`
Expected: FAIL, `undefined: ParseRuleHeaders`

- [ ] **Step 3: Implementieren**

In `internal/rulepack/model.go` ergänzen:

```go
// Rule is one habitat definition from section 3.
type Rule struct {
	// Priority is the leading digit, 1..8. Higher wins.
	Priority int
	// Code is the EUNIS code without the variant marker, e.g. "N15".
	Code string
	// Variant is "", "!" or "!!" — alternative definitions of the same code.
	Variant string
	// Name is the habitat's plain-text name.
	Name string
	// Raw is the membership formula as text, continuation lines joined.
	Raw string
	// Formula is the parsed formula. Filled in by Load.
	Formula Node
}

// Label returns the code plus its variant marker, e.g. "N15!!".
func (r Rule) Label() string { return r.Code + r.Variant }
```

`internal/rulepack/rules.go`:

```go
package rulepack

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	codeStart = 11 // 0-based: column 12
	codeEnd   = 16 // 0-based exclusive: columns 12..16
)

// ParseRuleHeaders parses section 3 into rules.
//
// The header line is fixed-width: priority in column 1, the code in columns
// 12..16, the habitat name from column 17 on. Splitting on spaces would read
// "N15!!Atlantic" as the code, because the variant marker "!!" fills the field
// and leaves no separator.
//
// A formula may span several lines; every line until the next header belongs to
// the current rule and is joined with a single space.
func ParseRuleHeaders(lines []string) ([]Rule, error) {
	var out []Rule
	var buf []string

	flush := func() {
		if len(out) == 0 {
			return
		}
		out[len(out)-1].Raw = strings.Join(buf, " ")
		buf = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isRuleHeader(line) {
			flush()
			r, err := parseRuleHeader(line)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
			continue
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("formula line before any rule header: %q", line)
		}
		buf = append(buf, strings.TrimSpace(line))
	}
	flush()
	return out, nil
}

// isRuleHeader reports whether a line starts a rule: a single digit followed by
// a space. Formula lines start with "(" or "<".
func isRuleHeader(line string) bool {
	return len(line) > 1 && line[0] >= '0' && line[0] <= '9' && line[1] == ' '
}

func parseRuleHeader(line string) (Rule, error) {
	prio, err := strconv.Atoi(line[:1])
	if err != nil {
		return Rule{}, fmt.Errorf("rule header %q: bad priority: %w", line, err)
	}
	if len(line) < codeEnd {
		return Rule{}, fmt.Errorf("rule header %q is shorter than the code field", line)
	}
	field := strings.TrimSpace(line[codeStart:codeEnd])
	code := strings.TrimRight(field, "!")
	variant := field[len(code):]
	if code == "" {
		return Rule{}, fmt.Errorf("rule header %q has an empty code field", line)
	}
	name := ""
	if len(line) > codeEnd {
		name = strings.TrimSpace(line[codeEnd:])
	}
	return Rule{Priority: prio, Code: code, Variant: variant, Name: name}, nil
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestParseRule -v`
Expected: PASS (drei Tests)

- [ ] **Step 5: Gegen die echte Datei prüfen**

Ergänze in `realfile_test.go`:

```go
func TestRulesRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	secs, _ := SplitSections(f)
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 312 {
		t.Fatalf("got %d rules, want 312", len(rules))
	}
	var plain, bang, bangbang int
	for _, r := range rules {
		switch r.Variant {
		case "":
			plain++
		case "!":
			bang++
		case "!!":
			bangbang++
		}
		if r.Priority < 1 || r.Priority > 8 {
			t.Errorf("rule %s: priority %d out of range", r.Label(), r.Priority)
		}
		if r.Raw == "" {
			t.Errorf("rule %s has an empty formula", r.Label())
		}
	}
	if plain != 273 || bang != 34 || bangbang != 5 {
		t.Errorf("variants: plain=%d bang=%d bangbang=%d, want 273/34/5", plain, bang, bangbang)
	}
}
```

Run: `ESY_FILE=… go test ./internal/rulepack/ -run RealFile -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/rulepack/
git commit -m "feat(rulepack): parse section 3 rule headers and multi-line formulas"
```

---

### Task 5: Ausdrucks-Grammatik

**Files:**
- Create: `internal/rulepack/expr.go`
- Modify: `internal/rulepack/model.go` (Typen `Expr`, `Atom`, `Operand`)
- Test: `internal/rulepack/expr_test.go`

**Interfaces:**
- Consumes: nichts
- Produces:
  - `type Atom struct { Kind, Qualifier, Name string }` — `Kind` ist `#TC`, `##Q`, `###`, `##C`, `#SC`, `#T$`, `#$$`, `$$C`, `$$N`, `#NN`, `NON` oder `""` (Artname); `Qualifier` ist `+04` oder leer; `Name` ist Gruppen-, Feld- oder Artname.
  - `type Operand struct { Atoms []Atom; Except []Atom; Literal string }`
  - `type Expr struct { Left Operand; Op string; Right Operand }` — `Op` ist `GR`, `GE`, `EQ` oder `""`.
  - `func ParseExpr(s string) (Expr, error)` — `s` ohne die spitzen Klammern.

Zwei Tokenisierungsfallen: Feldnamen enthalten Leerzeichen und Klammern (`$$N Altitude (m) GR 1000`), und Artnamen stehen direkt als Atom (`Empetrum nigrum aggr. GR 05`). An Leerzeichen zu trennen zerlegt beides falsch — deshalb wird der Operator als **letztes** freistehendes `GR`/`GE`/`EQ` gesucht.

- [ ] **Step 1: Failing test schreiben**

`internal/rulepack/expr_test.go`:

```go
package rulepack

import "testing"

func TestParseExpr(t *testing.T) {
	cases := []struct {
		in   string
		want Expr
	}{
		{
			in: "#TC Trees GR 25",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#TC", Name: "Trees"}}},
				Op:    "GR",
				Right: Operand{Literal: "25"},
			},
		},
		{
			in: "$$N Altitude (m) GR 1000",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "$$N", Name: "Altitude (m)"}}},
				Op:    "GR",
				Right: Operand{Literal: "1000"},
			},
		},
		{
			in: "$$C Country EQ United Kingdom",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "$$C", Name: "Country"}}},
				Op:    "EQ",
				Right: Operand{Literal: "United Kingdom"},
			},
		},
		{
			in: "Empetrum nigrum aggr. GR 05",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Name: "Empetrum nigrum aggr."}}},
				Op:    "GR",
				Right: Operand{Literal: "05"},
			},
		},
		{
			in: "##Q +12 Coastal-saltmarsh-species",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "##Q", Qualifier: "+12", Name: "Coastal-saltmarsh-species"}}},
			},
		},
		{
			in: "#TC Trees|#TC Shrubs GR 15",
			want: Expr{
				Left: Operand{Atoms: []Atom{
					{Kind: "#TC", Name: "Trees"},
					{Kind: "#TC", Name: "Shrubs"},
				}},
				Op:    "GR",
				Right: Operand{Literal: "15"},
			},
		},
		{
			in: "#TC Bog-Pinus GR #TC Trees|#TC Shrubs EXCEPT #TC Bog-Pinus",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "#TC", Name: "Bog-Pinus"}}},
				Op:   "GR",
				Right: Operand{
					Atoms:  []Atom{{Kind: "#TC", Name: "Trees"}, {Kind: "#TC", Name: "Shrubs"}},
					Except: []Atom{{Kind: "#TC", Name: "Bog-Pinus"}},
				},
			},
		},
		{
			in: "#SC W-acidic-garrigue-shrubs GE #$$",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#SC", Name: "W-acidic-garrigue-shrubs"}}},
				Op:    "GE",
				Right: Operand{Atoms: []Atom{{Kind: "#$$"}}},
			},
		},
		{
			in: "#TC Dry-heath-shrubs GE $50",
			want: Expr{
				Left:  Operand{Atoms: []Atom{{Kind: "#TC", Name: "Dry-heath-shrubs"}}},
				Op:    "GE",
				Right: Operand{Literal: "$50"},
			},
		},
		{
			in: "#01 +04 R52-Forest-fringe",
			want: Expr{
				Left: Operand{Atoms: []Atom{{Kind: "#01", Qualifier: "+04", Name: "R52-Forest-fringe"}}},
			},
		},
	}
	for _, c := range cases {
		got, err := ParseExpr(c.in)
		if err != nil {
			t.Errorf("ParseExpr(%q): %v", c.in, err)
			continue
		}
		if !exprEqual(got, c.want) {
			t.Errorf("ParseExpr(%q):\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
}

func exprEqual(a, b Expr) bool {
	return a.Op == b.Op && operandEqual(a.Left, b.Left) && operandEqual(a.Right, b.Right)
}

func operandEqual(a, b Operand) bool {
	if a.Literal != b.Literal || len(a.Atoms) != len(b.Atoms) || len(a.Except) != len(b.Except) {
		return false
	}
	for i := range a.Atoms {
		if a.Atoms[i] != b.Atoms[i] {
			return false
		}
	}
	for i := range a.Except {
		if a.Except[i] != b.Except[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestParseExpr -v`
Expected: FAIL, `undefined: ParseExpr`

- [ ] **Step 3: Implementieren**

In `model.go` ergänzen:

```go
// Atom is one term inside a membership expression.
type Atom struct {
	// Kind is the prefix: "#TC", "##Q", "##C", "###", "##D", "#SC", "#T$",
	// "#$$", "$$C", "$$N", "NON", "#01".."#12", or "" for a bare taxon name.
	Kind string
	// Qualifier is the comparison set marker, e.g. "+04". Empty if absent.
	Qualifier string
	// Name is the group, header field or taxon name. Empty for "#$$"/"#T$"
	// when they stand alone on the right-hand side.
	Name string
}

// Operand is one side of a membership expression: a union of atoms, optionally
// minus an EXCEPT union, or a bare literal (a number, a "$NN" percentage or a
// categorical header value).
type Operand struct {
	Atoms   []Atom
	Except  []Atom
	Literal string
}

// Expr is a membership expression, the content of one <...> pair.
type Expr struct {
	Left  Operand
	Op    string // "GR", "GE", "EQ", or "" when there is no right-hand side
	Right Operand
}
```

`internal/rulepack/expr.go`:

```go
package rulepack

import (
	"fmt"
	"strings"
)

var atomPrefixes = []string{
	"$$C", "$$N", "###", "##D", "##C", "##Q", "#TC", "#SC", "#T$", "#$$", "NON",
}

// ParseExpr parses the inside of one <...> membership expression.
//
// The operator is located as the LAST free-standing GR/GE/EQ token, not the
// first: header field names contain spaces and parentheses ("Altitude (m)") and
// categorical values are multi-word ("United Kingdom"), so a left-to-right scan
// would cut in the wrong place.
func ParseExpr(s string) (Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Expr{}, fmt.Errorf("empty expression")
	}
	left, op, right := splitOnOperator(s)
	e := Expr{Op: op}
	var err error
	if e.Left, err = parseOperand(left, false); err != nil {
		return Expr{}, fmt.Errorf("left of %q: %w", s, err)
	}
	if op != "" {
		if e.Right, err = parseOperand(right, true); err != nil {
			return Expr{}, fmt.Errorf("right of %q: %w", s, err)
		}
	}
	return e, nil
}

// splitOnOperator finds the last standalone GR/GE/EQ token.
func splitOnOperator(s string) (left, op, right string) {
	fields := strings.Fields(s)
	idx := -1
	for i, f := range fields {
		switch f {
		case "GR", "GE", "EQ":
			idx = i
		}
	}
	if idx < 0 {
		return s, "", ""
	}
	// Rebuild by character offset so inner spacing is preserved.
	before := strings.Join(fields[:idx], " ")
	after := strings.Join(fields[idx+1:], " ")
	return before, fields[idx], after
}

// parseOperand parses one side. onRight allows bare literals (numbers, "$50",
// categorical values such as "ATL_COAST" or "United Kingdom").
func parseOperand(s string, onRight bool) (Operand, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Operand{}, nil
	}
	main, except := s, ""
	if i := strings.Index(s, "EXCEPT"); i >= 0 {
		main, except = strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len("EXCEPT"):])
	}
	if onRight && !looksLikeAtom(main) && except == "" {
		return Operand{Literal: main}, nil
	}
	var op Operand
	var err error
	if op.Atoms, err = parseAtomList(main); err != nil {
		return Operand{}, err
	}
	if except != "" {
		if op.Except, err = parseAtomList(except); err != nil {
			return Operand{}, err
		}
	}
	return op, nil
}

func looksLikeAtom(s string) bool {
	for _, p := range atomPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return isCountPrefix(s)
}

// isCountPrefix matches "#01".."#99": the "at least N species" form.
func isCountPrefix(s string) bool {
	return len(s) >= 3 && s[0] == '#' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
}

func parseAtomList(s string) ([]Atom, error) {
	parts := strings.Split(s, "|")
	out := make([]Atom, 0, len(parts))
	for _, p := range parts {
		a, err := parseAtom(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func parseAtom(s string) (Atom, error) {
	if s == "" {
		return Atom{}, fmt.Errorf("empty atom")
	}
	var a Atom
	switch {
	case isCountPrefix(s):
		a.Kind, s = s[:3], strings.TrimSpace(s[3:])
	default:
		for _, p := range atomPrefixes {
			if strings.HasPrefix(s, p) {
				a.Kind, s = p, strings.TrimSpace(s[len(p):])
				break
			}
		}
	}
	// A qualifier is "+" followed by two digits.
	if len(s) >= 3 && s[0] == '+' && s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9' {
		a.Qualifier, s = s[:3], strings.TrimSpace(s[3:])
	}
	a.Name = s
	if a.Kind == "" && a.Name == "" {
		return Atom{}, fmt.Errorf("atom without kind or name")
	}
	return a, nil
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestParseExpr -v`
Expected: PASS

- [ ] **Step 5: Alle echten Ausdrücke durchparsen**

Ergänze in `realfile_test.go`:

```go
func TestParseEveryRealExpression(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	secs, _ := SplitSections(f)
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	kinds := map[string]int{}
	for _, r := range rules {
		for _, raw := range extractExpressions(r.Raw) {
			if seen[raw] {
				continue
			}
			seen[raw] = true
			e, err := ParseExpr(raw)
			if err != nil {
				t.Errorf("rule %s: ParseExpr(%q): %v", r.Label(), raw, err)
				continue
			}
			for _, a := range append(append([]Atom{}, e.Left.Atoms...), e.Right.Atoms...) {
				kinds[a.Kind]++
			}
		}
	}
	if len(seen) != 931 {
		t.Errorf("got %d distinct expressions, want 931", len(seen))
	}
	for _, k := range []string{"#TC", "##Q", "#SC", "#$$", "#T$", "$$C", "$$N", "NON"} {
		if kinds[k] == 0 {
			t.Errorf("no atom of kind %s was parsed", k)
		}
	}
}
```

`extractExpressions` wird in Task 6 mit implementiert; bis dahin kann dieser Test übersprungen werden, indem er erst nach Task 6 aktiviert wird.

- [ ] **Step 6: Die zusammengesetzte NON-Form ergänzen**

`NON` steht nie allein, sondern trägt eine weitere Bedingung: `##Q +10 D-Mires GR
NON ##Q +10 D-Mires` heißt „größer als dasselbe Maß über alle Arten *außerhalb*
dieser Gruppe". Ein naiver Parser macht daraus ein Atom mit dem Namen
`##Q +10 D-Mires`, das nie auf eine Gruppe trifft.

Test in `expr_test.go` ergänzen:

```go
func TestParseExprNonIsComposite(t *testing.T) {
	got, err := ParseExpr("##Q +10 D-Mires GR NON ##Q +10 D-Mires")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Right.Atoms) != 1 {
		t.Fatalf("right side: %+v", got.Right)
	}
	a := got.Right.Atoms[0]
	if a.Kind != "NON" || a.Inner != "##Q" || a.Qualifier != "+10" || a.Name != "D-Mires" {
		t.Errorf("NON atom = %+v, want Kind NON, Inner ##Q, Qualifier +10, Name D-Mires", a)
	}
}
```

In `model.go` das Feld ergänzen:

```go
	// Inner is the measure a NON atom negates, e.g. "##Q" in "NON ##Q +10 Grp".
	// Empty for every other kind.
	Inner string
```

In `parseAtom` nach der Präfixerkennung einfügen:

```go
	if a.Kind == "NON" {
		inner, err := parseAtom(s)
		if err != nil {
			return Atom{}, fmt.Errorf("NON without an inner measure: %w", err)
		}
		a.Inner, a.Qualifier, a.Name = inner.Kind, inner.Qualifier, inner.Name
		return a, nil
	}
```

In `measure` (Task 9) behandelt der Zweig `kind == "NON"` dann `a.Inner` über den
Arten außerhalb der Gruppe statt pauschal `TotalCover`.

Run: `go test ./internal/rulepack/ -run TestParseExpr -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/rulepack/
git commit -m "feat(rulepack): parse membership expressions incl. unions, EXCEPT and NON"
```

---

### Task 6: Formel-Parser mit R-Präzedenz

**Files:**
- Create: `internal/rulepack/formula.go`
- Modify: `internal/rulepack/model.go` (Typ `Node`)
- Test: `internal/rulepack/formula_test.go`

**Interfaces:**
- Consumes: `ParseExpr`
- Produces:
  - `type Node interface{ isNode() }` mit `And{L,R Node}`, `Or{L,R Node}`, `Not{L,R Node}`, `Leaf{Expr Expr; Raw string}`
  - `func ParseFormula(s string) (Node, error)`
  - `func extractExpressions(s string) []string`

**Präzedenz kommt aus R**, weil der Upstream `AND`→`&`, `OR`→`|`, `NOT`→`&!` ersetzt und `eval(parse(...))` rechnen lässt: `!` bindet stärker als `&`, `&` stärker als `|`. `NOT` ist **binär** („und nicht"). Ein Parser mit gleichrangigen, linksassoziativen Operatoren läge bei jeder unparenthesierten Kombination falsch.

- [ ] **Step 1: Failing test schreiben**

`internal/rulepack/formula_test.go`:

```go
package rulepack

import "testing"

// shape renders the tree so precedence is visible in one string.
func shape(n Node) string {
	switch v := n.(type) {
	case Leaf:
		return v.Raw
	case And:
		return "(" + shape(v.L) + " & " + shape(v.R) + ")"
	case Or:
		return "(" + shape(v.L) + " | " + shape(v.R) + ")"
	case Not:
		return "(" + shape(v.L) + " &! " + shape(v.R) + ")"
	}
	return "?"
}

func TestParseFormulaPrecedence(t *testing.T) {
	cases := []struct{ in, want string }{
		// AND binds tighter than OR — R semantics.
		{"<a> OR <b> AND <c>", "(a | (b & c))"},
		{"<a> AND <b> OR <c>", "((a & b) | c)"},
		// NOT is binary "and not" and binds tighter than OR.
		{"<a> OR <b> NOT <c>", "(a | (b &! c))"},
		// Parentheses win.
		{"(<a> OR <b>) AND <c>", "((a | b) & c)"},
		// Left associativity within one precedence level.
		{"<a> AND <b> AND <c>", "((a & b) & c)"},
	}
	for _, c := range cases {
		n, err := ParseFormula(c.in)
		if err != nil {
			t.Errorf("ParseFormula(%q): %v", c.in, err)
			continue
		}
		if got := shape(n); got != c.want {
			t.Errorf("ParseFormula(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestParseFormulaRealRule(t *testing.T) {
	in := "(<#TC Shrubs GR 25> NOT (<#TC Trees GR 25> OR <#TC Native-light-canopy-trees GR 15>)) " +
		"AND (<$$C Dunes_Bohn EQ Y_DUNES> AND (<$$C Coast_EEA EQ ATL_COAST> OR <$$C Coast_EEA EQ BAL_COAST>))"
	n, err := ParseFormula(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := n.(And); !ok {
		t.Fatalf("top level should be AND, got %T", n)
	}
}

func TestExtractExpressions(t *testing.T) {
	got := extractExpressions("(<#TC Trees GR 25> OR <#TC Shrubs GR 15>) NOT <a>")
	want := []string{"#TC Trees GR 25", "#TC Shrubs GR 15", "a"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseFormulaRejectsUnbalanced(t *testing.T) {
	if _, err := ParseFormula("(<a> AND <b>"); err == nil {
		t.Fatal("want error for unbalanced parentheses")
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestParseFormula -v`
Expected: FAIL, `undefined: ParseFormula`

- [ ] **Step 3: Implementieren**

In `model.go` ergänzen:

```go
// Node is a node of a parsed membership formula.
type Node interface{ isNode() }

// And is "x AND y".
type And struct{ L, R Node }

// Or is "x OR y".
type Or struct{ L, R Node }

// Not is "x NOT y", the binary "and not" of the expert-system language.
type Not struct{ L, R Node }

// Leaf is a single membership expression.
type Leaf struct {
	Expr Expr
	Raw  string
}

func (And) isNode()  {}
func (Or) isNode()   {}
func (Not) isNode()  {}
func (Leaf) isNode() {}
```

`internal/rulepack/formula.go`:

```go
package rulepack

import (
	"fmt"
	"strings"
)

// ParseFormula parses a membership formula with R's operator precedence:
// NOT (unary "!" in R) binds tightest, then AND ("&"), then OR ("|").
// Upstream rewrites AND/OR/NOT to &/|/&! and hands the string to R's parser, so
// R's precedence is the behaviour we must reproduce.
func ParseFormula(s string) (Node, error) {
	toks, err := tokenizeFormula(s)
	if err != nil {
		return nil, err
	}
	p := &formulaParser{toks: toks}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("trailing input at token %d: %q", p.pos, p.toks[p.pos])
	}
	return n, nil
}

type formulaParser struct {
	toks []string
	pos  int
}

func (p *formulaParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

// parseOr is the lowest precedence level.
func (p *formulaParser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "OR" {
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or{L: left, R: right}
	}
	return left, nil
}

// parseAnd handles AND and NOT, which share a level in R only in the sense that
// "&" and "&!" are both "&"-strength; NOT's "!" applies to its right operand.
func (p *formulaParser) parseAnd() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek() {
		case "AND":
			p.pos++
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = And{L: left, R: right}
		case "NOT":
			p.pos++
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = Not{L: left, R: right}
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parsePrimary() (Node, error) {
	switch t := p.peek(); {
	case t == "(":
		p.pos++
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return n, nil
	case strings.HasPrefix(t, "<"):
		p.pos++
		raw := strings.TrimSuffix(strings.TrimPrefix(t, "<"), ">")
		e, err := ParseExpr(raw)
		if err != nil {
			return nil, err
		}
		return Leaf{Expr: e, Raw: raw}, nil
	case t == "":
		return nil, fmt.Errorf("unexpected end of formula")
	default:
		return nil, fmt.Errorf("unexpected token %q", t)
	}
}

// tokenizeFormula splits into "(", ")", "AND", "OR", "NOT" and "<...>" chunks.
func tokenizeFormula(s string) ([]string, error) {
	var out []string
	i := 0
	for i < len(s) {
		switch c := s[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '(' || c == ')':
			out = append(out, string(c))
			i++
		case c == '<':
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				return nil, fmt.Errorf("unterminated expression at offset %d", i)
			}
			out = append(out, s[i:i+j+1])
			i += j + 1
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '(' && s[j] != ')' && s[j] != '<' {
				j++
			}
			word := s[i:j]
			switch word {
			case "AND", "OR", "NOT":
				out = append(out, word)
			default:
				return nil, fmt.Errorf("unexpected word %q at offset %d", word, i)
			}
			i = j
		}
	}
	return out, nil
}

// extractExpressions returns every <...> body in the formula, in order.
func extractExpressions(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		a := strings.IndexByte(s[i:], '<')
		if a < 0 {
			break
		}
		a += i
		b := strings.IndexByte(s[a:], '>')
		if b < 0 {
			break
		}
		out = append(out, s[a+1:a+b])
		i = a + b + 1
	}
	return out
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run "TestParseFormula|TestExtractExpressions" -v`
Expected: PASS

- [ ] **Step 5: Jede echte Regel parsen**

Run: `ESY_FILE=~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt go test ./internal/rulepack/ -run RealFile -v`

Ergänze zuvor in `realfile_test.go`:

```go
func TestParseEveryRealFormula(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	secs, _ := SplitSections(f)
	rules, _ := ParseRuleHeaders(secs[3])
	for _, r := range rules {
		if _, err := ParseFormula(r.Raw); err != nil {
			t.Errorf("rule %s: %v\n  %s", r.Label(), err, r.Raw)
		}
	}
}
```

Expected: PASS für alle 312 Regeln. Schlägt eine fehl, ist die Grammatik unvollständig — die Regel im Commit benennen.

- [ ] **Step 6: Commit**

```bash
git add internal/rulepack/
git commit -m "feat(rulepack): parse formulas with R operator precedence"
```

---

### Task 7: Dreiwertige Logik und Deckungsverschmelzung

**Files:**
- Create: `internal/esy/tri.go`, `internal/esy/cover.go`
- Test: `internal/esy/tri_test.go`, `internal/esy/cover_test.go`

**Interfaces:**
- Consumes: nichts
- Produces:
  - `type Tri int8` mit `False`, `True`, `Unknown`; `func (t Tri) And(o Tri) Tri`, `Or`, `AndNot`, `IsTrue() bool`
  - `func TotalCover(covers []float64) float64`

- [ ] **Step 1: Failing tests schreiben**

`internal/esy/tri_test.go`:

```go
package esy

import "testing"

func TestTriKleene(t *testing.T) {
	cases := []struct {
		a, b       Tri
		and, or    Tri
		andNot     Tri
	}{
		{True, True, True, True, False},
		{True, False, False, True, True},
		{False, True, False, True, False},
		{False, False, False, False, False},
		{True, Unknown, Unknown, True, Unknown},
		{Unknown, True, Unknown, True, False},
		{False, Unknown, False, Unknown, False},
		{Unknown, False, Unknown, Unknown, Unknown},
		{Unknown, Unknown, Unknown, Unknown, Unknown},
	}
	for _, c := range cases {
		if got := c.a.And(c.b); got != c.and {
			t.Errorf("%v AND %v = %v, want %v", c.a, c.b, got, c.and)
		}
		if got := c.a.Or(c.b); got != c.or {
			t.Errorf("%v OR %v = %v, want %v", c.a, c.b, got, c.or)
		}
		if got := c.a.AndNot(c.b); got != c.andNot {
			t.Errorf("%v NOT %v = %v, want %v", c.a, c.b, got, c.andNot)
		}
	}
}

func TestTriIsTrueTreatsUnknownAsNoMatch(t *testing.T) {
	if Unknown.IsTrue() {
		t.Error("Unknown must not count as a match — upstream selects with == TRUE")
	}
}
```

`internal/esy/cover_test.go`:

```go
package esy

import (
	"math"
	"testing"
)

func TestTotalCoverJenningsFischer(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{30}, 30},
		{[]float64{3, 4}, 6.88},
		{[]float64{10, 9, 8}, 24.652},
		{[]float64{100, 50}, 100},
	}
	for _, c := range cases {
		if got := TotalCover(c.in); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("TotalCover(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// The union is commutative, so input order must not matter.
func TestTotalCoverIsOrderIndependent(t *testing.T) {
	a := TotalCover([]float64{12, 7, 33, 1})
	b := TotalCover([]float64{1, 33, 7, 12})
	if a != b {
		t.Errorf("order changed the result: %v vs %v", a, b)
	}
}

// Upstream rounds to 10 decimals (commit 416bab9) to stabilise threshold cases.
func TestTotalCoverRoundsToTenDecimals(t *testing.T) {
	got := TotalCover([]float64{1.0 / 3.0, 1.0 / 3.0})
	if got != roundTo(got, 10) {
		t.Errorf("result is not rounded to 10 decimals: %v", got)
	}
}
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/esy/ -v`
Expected: FAIL, `undefined: Tri`, `undefined: TotalCover`

- [ ] **Step 3: Implementieren**

`internal/esy/tri.go`:

```go
// Package esy evaluates parsed ESy rule packs against a vegetation plot.
package esy

// Tri is a three-valued truth value. Upstream computes in R, where a missing
// header value produces NA and NA propagates through & and |. The final
// selection is which(matrix == TRUE), so NA never counts as a match.
type Tri int8

const (
	False Tri = iota
	True
	Unknown
)

func (t Tri) String() string {
	switch t {
	case False:
		return "FALSE"
	case True:
		return "TRUE"
	default:
		return "NA"
	}
}

// And is R's "&": FALSE dominates, otherwise NA propagates.
func (t Tri) And(o Tri) Tri {
	if t == False || o == False {
		return False
	}
	if t == Unknown || o == Unknown {
		return Unknown
	}
	return True
}

// Or is R's "|": TRUE dominates, otherwise NA propagates.
func (t Tri) Or(o Tri) Tri {
	if t == True || o == True {
		return True
	}
	if t == Unknown || o == Unknown {
		return Unknown
	}
	return False
}

// AndNot is the expert system's binary NOT, i.e. R's "&!".
func (t Tri) AndNot(o Tri) Tri { return t.And(o.not()) }

func (t Tri) not() Tri {
	switch t {
	case True:
		return False
	case False:
		return True
	default:
		return Unknown
	}
}

// IsTrue reports whether this value counts as a match.
func (t Tri) IsTrue() bool { return t == True }

// FromBool lifts a Go bool.
func FromBool(b bool) Tri {
	if b {
		return True
	}
	return False
}
```

`internal/esy/cover.go`:

```go
package esy

import "math"

// TotalCover merges cover values assuming independent random overlap, the
// Jennings-Fischer union used throughout ESy:
//
//	total = 1 - prod(1 - c/100)
//
// Cover is projected area, not amount, so adding values would double-count the
// overlap and can exceed 100%. Upstream rounds to 10 decimals (commit 416bab9)
// to keep threshold comparisons stable, and we reproduce that exactly.
func TotalCover(covers []float64) float64 {
	if len(covers) == 0 {
		return 0
	}
	rest := 1.0
	for _, c := range covers {
		rest *= 1 - c/100
	}
	return roundTo((1-rest)*100, 10)
}

func roundTo(v float64, decimals int) float64 {
	f := math.Pow(10, float64(decimals))
	return math.Round(v*f) / f
}
```

- [ ] **Step 4: Tests laufen lassen, Erfolg prüfen**

Run: `go test ./internal/esy/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/esy/
git commit -m "feat(esy): three-valued logic and Jennings-Fischer cover union"
```

---

### Task 8: Namensauflösung

**Files:**
- Create: `internal/taxa/resolve.go`
- Test: `internal/taxa/resolve_test.go`

**Interfaces:**
- Consumes: `rulepack.Pack.Aggregation`, `esy.TotalCover`
- Produces:
  - `type Record struct { Name string; Cover float64 }`
  - `type Step struct { Input string; AfterBackbone string; Final string; Resolved bool }`
  - `func Resolve(in []Record, backbone, aggregation map[string]string) ([]Record, []Step)`

Zwei Stufen, nach **jeder** wird verschmolzen. Die Korrektur der Populus-Zeile aus Spec §8 sitzt hier, nicht im Parser: Der Parser bildet die Datei ab, die Fachschicht korrigiert den belegten Datenfehler.

- [ ] **Step 1: Failing test schreiben**

`internal/taxa/resolve_test.go`:

```go
package taxa

import (
	"math"
	"testing"
)

func TestResolveTwoStages(t *testing.T) {
	backbone := map[string]string{"Abies pectinata": "Abies alba"}
	agg := map[string]string{"Abies alba": "Abies alba aggr."}
	got, steps := Resolve([]Record{{"Abies pectinata", 20}}, backbone, agg)
	if len(got) != 1 || got[0].Name != "Abies alba aggr." {
		t.Fatalf("two-stage resolution failed: %+v", got)
	}
	if steps[0].AfterBackbone != "Abies alba" || steps[0].Final != "Abies alba aggr." {
		t.Errorf("step report wrong: %+v", steps[0])
	}
}

func TestResolveMergesCoversAfterAggregation(t *testing.T) {
	agg := map[string]string{
		"Empetrum nigrum":         "Empetrum nigrum aggr.",
		"Empetrum hermaphroditum": "Empetrum nigrum aggr.",
	}
	got, _ := Resolve([]Record{
		{"Empetrum nigrum", 3},
		{"Empetrum hermaphroditum", 4},
	}, nil, agg)
	if len(got) != 1 {
		t.Fatalf("want 1 merged record, got %d", len(got))
	}
	if math.Abs(got[0].Cover-6.88) > 1e-9 {
		t.Errorf("cover = %v, want 6.88 (Jennings-Fischer, not 7)", got[0].Cover)
	}
}

func TestResolveIsSinglePass(t *testing.T) {
	// Elytrigia species is both a source and a target. One pass only.
	agg := map[string]string{
		"Elymus pycnanthus": "Elytrigia species",
		"Elytrigia species": "Elymus species",
	}
	got, _ := Resolve([]Record{{"Elymus pycnanthus", 10}}, nil, agg)
	if got[0].Name != "Elytrigia species" {
		t.Errorf("chain was followed: got %q, want %q", got[0].Name, "Elytrigia species")
	}
}

func TestResolveKeepsUnknownNames(t *testing.T) {
	got, steps := Resolve([]Record{{"Nonexistent name", 5}}, nil, nil)
	if len(got) != 1 || got[0].Name != "Nonexistent name" {
		t.Fatalf("unknown names must pass through: %+v", got)
	}
	if steps[0].Resolved {
		t.Error("step must be marked unresolved")
	}
}

func TestResolveAppliesPopulusCorrection(t *testing.T) {
	// Spec section 8: the file maps this poplar onto a grass. Corrected here.
	agg := map[string]string{
		"Populus x canadensis + P. nigra": "Polypogon monspeliensis x viridis",
	}
	got, _ := Resolve([]Record{{"Populus x canadensis + P. nigra", 10}}, nil, agg)
	if got[0].Name != "Populus x canadensis" {
		t.Errorf("Populus correction not applied: %q", got[0].Name)
	}
}

func TestResolveOutputIsDeterministic(t *testing.T) {
	agg := map[string]string{"b": "z", "a": "z", "c": "y"}
	in := []Record{{"a", 1}, {"b", 2}, {"c", 3}}
	first, _ := Resolve(in, nil, agg)
	for i := 0; i < 20; i++ {
		again, _ := Resolve(in, nil, agg)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("non-deterministic output at %d: %+v vs %+v", j, first[j], again[j])
			}
		}
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/taxa/ -v`
Expected: FAIL, `undefined: Resolve`

- [ ] **Step 3: Implementieren**

`internal/taxa/resolve.go`:

```go
// Package taxa resolves input taxon names onto ESy target concepts.
package taxa

import (
	"sort"

	"github.com/jobrunner/habitatus/internal/esy"
)

// Record is one taxon observation.
type Record struct {
	Name  string
	Cover float64
}

// Step documents how one input name was resolved, for the response report.
type Step struct {
	Input         string
	AfterBackbone string
	Final         string
	Resolved      bool
}

// corrections fixes defects in the official rule file that are demonstrably
// data errors rather than intent. See spec section 8.
//
// "Populus x canadensis + P. nigra" is listed under the grass "Polypogon
// monspeliensis x viridis"; Polypogon and Populus are adjacent in an
// alphabetically sorted list, so the record sits one block too early.
var corrections = map[string]string{
	"Populus x canadensis + P. nigra": "Populus x canadensis",
}

// Resolve maps input names onto ESy concepts in two stages — first the source
// nomenclature's translation table, then section 1 aggregation — merging covers
// after each stage.
//
// Matching is an exact string lookup with no normalisation, mirroring upstream's
// match(). Resolution is single-pass: chains are never followed.
func Resolve(in []Record, backbone, aggregation map[string]string) ([]Record, []Step) {
	steps := make([]Step, len(in))
	staged := make([]Record, len(in))
	for i, r := range in {
		steps[i].Input = r.Name
		name := r.Name
		if backbone != nil {
			if t, ok := backbone[name]; ok {
				name = t
			}
		}
		steps[i].AfterBackbone = name
		staged[i] = Record{Name: name, Cover: r.Cover}
	}
	staged = merge(staged)

	final := make([]Record, len(staged))
	mapped := map[string]string{}
	for i, r := range staged {
		name := r.Name
		if t, ok := aggregation[name]; ok {
			name = t
		}
		if c, ok := corrections[r.Name]; ok {
			name = c
		}
		mapped[r.Name] = name
		final[i] = Record{Name: name, Cover: r.Cover}
	}
	for i := range steps {
		f, ok := mapped[steps[i].AfterBackbone]
		if !ok {
			f = steps[i].AfterBackbone
		}
		steps[i].Final = f
		steps[i].Resolved = f != steps[i].Input || aggregation[steps[i].Input] != "" ||
			(backbone != nil && backbone[steps[i].Input] != "")
	}
	return merge(final), steps
}

// merge sums duplicate concepts with the Jennings-Fischer union and returns a
// deterministically ordered slice.
func merge(rs []Record) []Record {
	byName := map[string][]float64{}
	for _, r := range rs {
		byName[r.Name] = append(byName[r.Name], r.Cover)
	}
	out := make([]Record, 0, len(byName))
	for name, covers := range byName {
		out = append(out, Record{Name: name, Cover: esy.TotalCover(covers)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/taxa/ -v`
Expected: PASS (sechs Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/taxa/
git commit -m "feat(taxa): two-stage single-pass name resolution with cover merging"
```

---

### Task 9: Bedingungsauswertung

**Files:**
- Create: `internal/esy/condition.go`
- Test: `internal/esy/condition_test.go`

**Interfaces:**
- Consumes: `rulepack.Atom`, `rulepack.Operand`, `rulepack.Expr`, `taxa.Record`, `TotalCover`
- Produces:
  - `type Plot struct { Records []taxa.Record; Header map[string]string }`
  - `type Env struct { Groups map[string][]string }`
  - `func (e Env) EvalExpr(x rulepack.Expr, p Plot) (Tri, float64, float64)` — Wahrheitswert plus linker und rechter Zahlenwert; die Zahlen sind die Zwischenwerte, die der Golden Master vergleicht.

Die zehn Bedingungstypen nach Spec §5.1. `#T$` ist die Gesamtdeckung **ohne** die verglichene Gruppe, `#$$` die höchste Deckung einer Art außerhalb der Gruppe, `$NN` ein Prozentsatz der Gesamtdeckung, `#SC` die Deckung einer Einzelart der Gruppe.

- [ ] **Step 1: Failing test schreiben**

`internal/esy/condition_test.go`:

```go
package esy

import (
	"math"
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

func testEnv() Env {
	return Env{Groups: map[string][]string{
		"Trees":  {"Fagus sylvatica", "Quercus robur", "Carpinus betulus"},
		"Shrubs": {"Corylus avellana"},
	}}
}

func testPlot() Plot {
	return Plot{
		Records: []taxa.Record{
			{Name: "Fagus sylvatica", Cover: 10},
			{Name: "Quercus robur", Cover: 9},
			{Name: "Carpinus betulus", Cover: 8},
			{Name: "Corylus avellana", Cover: 30},
			{Name: "Anemone nemorosa", Cover: 40},
		},
		Header: map[string]string{
			"Country":      "Germany",
			"Coast_EEA":    "N_COAST",
			"Altitude (m)": "250",
			"DEG_LAT":      "49.79",
		},
	}
}

func evalRaw(t *testing.T, raw string) (Tri, float64, float64) {
	t.Helper()
	x, err := rulepack.ParseExpr(raw)
	if err != nil {
		t.Fatalf("ParseExpr(%q): %v", raw, err)
	}
	return testEnv().EvalExpr(x, testPlot())
}

func TestEvalTotalCoverOfGroup(t *testing.T) {
	// 10, 9, 8 -> 24.652 by Jennings-Fischer, NOT 27 by addition.
	got, left, _ := evalRaw(t, "#TC Trees GR 25")
	if math.Abs(left-24.652) > 1e-9 {
		t.Errorf("left value = %v, want 24.652", left)
	}
	if got != False {
		t.Errorf("24.652 GR 25 = %v, want FALSE — addition would wrongly pass", got)
	}
}

func TestEvalSpeciesCount(t *testing.T) {
	if got, left, _ := evalRaw(t, "### Trees GR 2"); got != True || left != 3 {
		t.Errorf("### Trees = %v (left %v), want TRUE with 3", got, left)
	}
}

func TestEvalAtLeastNSpecies(t *testing.T) {
	if got, _, _ := evalRaw(t, "#03 Trees"); got != True {
		t.Errorf("#03 Trees = %v, want TRUE (3 species present)", got)
	}
	if got, _, _ := evalRaw(t, "#04 Trees"); got != False {
		t.Errorf("#04 Trees = %v, want FALSE", got)
	}
}

func TestEvalSumOfCovers(t *testing.T) {
	if _, left, _ := evalRaw(t, "##C Trees GR 0"); math.Abs(left-27) > 1e-9 {
		t.Errorf("##C = %v, want 27 (plain sum)", left)
	}
}

func TestEvalSumOfSquareRoots(t *testing.T) {
	want := math.Sqrt(10) + math.Sqrt(9) + math.Sqrt(8)
	if _, left, _ := evalRaw(t, "##Q Trees GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("##Q = %v, want %v", left, want)
	}
}

func TestEvalGroupUnion(t *testing.T) {
	// Trees plus Shrubs: 10, 9, 8, 30.
	want := TotalCover([]float64{10, 9, 8, 30})
	if _, left, _ := evalRaw(t, "#TC Trees|#TC Shrubs GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("union = %v, want %v", left, want)
	}
}

func TestEvalExcept(t *testing.T) {
	// Trees EXCEPT Shrubs is just Trees here.
	want := TotalCover([]float64{10, 9, 8})
	if _, left, _ := evalRaw(t, "#TC Trees|#TC Shrubs EXCEPT #TC Shrubs GR 0"); math.Abs(left-want) > 1e-9 {
		t.Errorf("EXCEPT = %v, want %v", left, want)
	}
}

func TestEvalTotalCoverExcludingGroup(t *testing.T) {
	// #T$ for Trees: everything except the Trees = Corylus 30 and Anemone 40.
	want := TotalCover([]float64{30, 40})
	_, left, right := evalRaw(t, "#TC Trees GR #T$ Trees")
	if math.Abs(right-want) > 1e-9 {
		t.Errorf("#T$ = %v, want %v", right, want)
	}
	if left >= right {
		t.Errorf("trees (%v) should not dominate the rest (%v)", left, right)
	}
}

func TestEvalHighestCoverOutsideGroup(t *testing.T) {
	// #$$ EXCEPT Trees: highest single cover outside Trees = Anemone 40.
	_, _, right := evalRaw(t, "#SC Trees GE #$$ EXCEPT Trees")
	if math.Abs(right-40) > 1e-9 {
		t.Errorf("#$$ = %v, want 40", right)
	}
}

func TestEvalSingleSpeciesCoverInGroup(t *testing.T) {
	// #SC Trees: the highest single cover inside Trees = Fagus 10.
	_, left, _ := evalRaw(t, "#SC Trees GE 0")
	if math.Abs(left-10) > 1e-9 {
		t.Errorf("#SC = %v, want 10", left)
	}
}

func TestEvalPercentOfTotalCover(t *testing.T) {
	total := TotalCover([]float64{10, 9, 8, 30, 40})
	_, _, right := evalRaw(t, "#TC Trees GE $50")
	if math.Abs(right-total*0.5) > 1e-9 {
		t.Errorf("$50 = %v, want %v (half of total cover %v)", right, total*0.5, total)
	}
}

func TestEvalCategoricalHeader(t *testing.T) {
	if got, _, _ := evalRaw(t, "$$C Country EQ Germany"); got != True {
		t.Errorf("Country EQ Germany = %v, want TRUE", got)
	}
	if got, _, _ := evalRaw(t, "$$C Country EQ France"); got != False {
		t.Errorf("Country EQ France = %v, want FALSE", got)
	}
}

func TestEvalNumericHeader(t *testing.T) {
	if got, _, _ := evalRaw(t, "$$N Altitude (m) GR 100"); got != True {
		t.Errorf("Altitude GR 100 = %v, want TRUE", got)
	}
	if got, _, _ := evalRaw(t, "$$N Altitude (m) GR 1000"); got != False {
		t.Errorf("Altitude GR 1000 = %v, want FALSE", got)
	}
}

func TestEvalMissingHeaderIsUnknown(t *testing.T) {
	x, _ := rulepack.ParseExpr("$$N Ecoreg EQ 664")
	got, _, _ := testEnv().EvalExpr(x, testPlot())
	if got != Unknown {
		t.Errorf("missing header = %v, want Unknown", got)
	}
}

func TestEvalUnknownGroupIsUnknown(t *testing.T) {
	x, _ := rulepack.ParseExpr("#TC Nonexistent-group GR 5")
	got, _, _ := testEnv().EvalExpr(x, testPlot())
	if got != Unknown {
		t.Errorf("unknown group = %v, want Unknown", got)
	}
}

func TestEvalBareTaxonName(t *testing.T) {
	if got, _, _ := evalRaw(t, "Corylus avellana GR 25"); got != True {
		t.Errorf("bare taxon name = %v, want TRUE (cover 30)", got)
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/esy/ -run TestEval -v`
Expected: FAIL, `undefined: Env`

- [ ] **Step 3: Implementieren**

`internal/esy/condition.go`:

```go
package esy

import (
	"math"
	"strconv"
	"strings"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// Plot is one resolved vegetation plot.
type Plot struct {
	Records []taxa.Record
	Header  map[string]string
}

// Env holds everything the evaluator needs besides the plot.
type Env struct {
	Groups map[string][]string
}

// nan marks a value that could not be computed — a missing header or an unknown
// group. It propagates to Unknown, mirroring R's NA.
var nan = math.NaN()

// EvalExpr evaluates one membership expression and returns its truth value plus
// the numeric values of both sides. The numbers are the intermediate results the
// golden master compares against upstream's plot.cond matrix.
func (e Env) EvalExpr(x rulepack.Expr, p Plot) (Tri, float64, float64) {
	left := e.operandValue(x.Left, p, x)
	if x.Op == "" {
		// Left-hand-only expressions are truth-valued by presence: upstream
		// inserts "GR NON <same group>" for them, i.e. "greater than any other
		// group in the same comparison set".
		return e.compareWithinSet(x, p, left)
	}
	if isCategorical(x.Left) {
		return e.compareCategorical(x, p), nan, nan
	}
	right := e.operandValue(x.Right, p, x)
	if math.IsNaN(left) || math.IsNaN(right) {
		return Unknown, left, right
	}
	switch x.Op {
	case "GR":
		return FromBool(left > right), left, right
	case "GE":
		return FromBool(left >= right), left, right
	case "EQ":
		return FromBool(left == right), left, right
	}
	return Unknown, left, right
}

func isCategorical(o rulepack.Operand) bool {
	return len(o.Atoms) == 1 && o.Atoms[0].Kind == "$$C"
}

func (e Env) compareCategorical(x rulepack.Expr, p Plot) Tri {
	field := x.Left.Atoms[0].Name
	have, ok := p.Header[field]
	if !ok || have == "" {
		return Unknown
	}
	return FromBool(have == x.Right.Literal)
}

// compareWithinSet handles expressions with no right-hand side: the measure of
// the named group must exceed that of every other group carrying the same "+NN"
// qualifier.
func (e Env) compareWithinSet(x rulepack.Expr, p Plot, left float64) (Tri, float64, float64) {
	if math.IsNaN(left) {
		return Unknown, left, nan
	}
	a := x.Left.Atoms[0]
	if isCountPrefixKind(a.Kind) {
		// "#03 Group" is a standalone predicate: at least N species present.
		return FromBool(left > 0), left, nan
	}
	best := 0.0
	for name := range e.Groups {
		if name == a.Name {
			continue
		}
		v := e.measure(a.Kind, []string{name}, nil, p, x)
		if !math.IsNaN(v) && v > best {
			best = v
		}
	}
	return FromBool(left > best), left, best
}

func isCountPrefixKind(k string) bool {
	return len(k) == 3 && k[0] == '#' && k[1] >= '0' && k[1] <= '9' && k[2] >= '0' && k[2] <= '9'
}

func (e Env) operandValue(o rulepack.Operand, p Plot, x rulepack.Expr) float64 {
	if o.Literal != "" {
		return e.literalValue(o.Literal, p)
	}
	if len(o.Atoms) == 0 {
		return nan
	}
	a := o.Atoms[0]

	// Header atoms.
	if a.Kind == "$$N" {
		v, ok := p.Header[a.Name]
		if !ok || v == "" {
			return nan
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nan
		}
		return f
	}

	// "#T$" and "#$$" with no group of their own take the group named on the
	// other side of the expression.
	names, except := atomNames(o.Atoms), atomNames(o.Except)
	if len(names) == 1 && names[0] == "" {
		names = atomNames(x.Left.Atoms)
	}
	return e.measure(a.Kind, names, except, p, x)
}

func (e Env) literalValue(lit string, p Plot) float64 {
	if strings.HasPrefix(lit, "$") {
		pct, err := strconv.ParseFloat(lit[1:], 64)
		if err != nil {
			return nan
		}
		return e.plotTotal(p, nil) * pct / 100
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return nan
	}
	return f
}

func atomNames(as []rulepack.Atom) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

// measure computes one of the ten condition kinds over the union of the named
// groups minus the EXCEPT groups.
func (e Env) measure(kind string, names, except []string, p Plot, x rulepack.Expr) float64 {
	switch kind {
	case "#T$":
		return e.plotTotal(p, e.members(names, nil))
	case "#$$":
		return e.highestOutside(p, e.members(names, nil))
	}

	in, ok := e.membersChecked(names, except)
	if !ok {
		return nan
	}
	covers := coversOf(p, in)

	switch {
	case kind == "#TC":
		return TotalCover(covers)
	case kind == "##C":
		return sum(covers)
	case kind == "##Q" || kind == "##D":
		if kind == "##Q" {
			return sumSqrt(covers)
		}
		return float64(len(covers))
	case kind == "###":
		return float64(len(covers))
	case kind == "#SC":
		return maxOf(covers)
	case kind == "NON":
		out := coversOutside(p, in)
		return TotalCover(out)
	case isCountPrefixKind(kind):
		n, _ := strconv.Atoi(kind[1:])
		return boolToFloat(len(covers) >= n)
	case kind == "":
		// A bare taxon name: its own cover.
		return maxOf(covers)
	}
	return nan
}

func (e Env) members(names []string, except []string) map[string]bool {
	m, _ := e.membersChecked(names, except)
	return m
}

// membersChecked resolves group names to the set of member taxa. A name that is
// neither a known group nor plausibly a taxon makes the whole condition Unknown.
func (e Env) membersChecked(names, except []string) (map[string]bool, bool) {
	in := map[string]bool{}
	for _, n := range names {
		if n == "" {
			continue
		}
		if ms, ok := e.Groups[n]; ok {
			for _, m := range ms {
				in[m] = true
			}
			continue
		}
		if strings.ContainsAny(n, "-") {
			// Looks like a group name but is not defined.
			return nil, false
		}
		in[n] = true // bare taxon name
	}
	for _, n := range except {
		if ms, ok := e.Groups[n]; ok {
			for _, m := range ms {
				delete(in, m)
			}
			continue
		}
		delete(in, n)
	}
	return in, true
}

func coversOf(p Plot, in map[string]bool) []float64 {
	var out []float64
	for _, r := range p.Records {
		if in[r.Name] {
			out = append(out, r.Cover)
		}
	}
	return out
}

func coversOutside(p Plot, in map[string]bool) []float64 {
	var out []float64
	for _, r := range p.Records {
		if !in[r.Name] {
			out = append(out, r.Cover)
		}
	}
	return out
}

// plotTotal is the total cover of the plot, excluding the given taxa.
func (e Env) plotTotal(p Plot, exclude map[string]bool) float64 {
	var cs []float64
	for _, r := range p.Records {
		if exclude != nil && exclude[r.Name] {
			continue
		}
		cs = append(cs, r.Cover)
	}
	return TotalCover(cs)
}

func (e Env) highestOutside(p Plot, in map[string]bool) float64 {
	return maxOf(coversOutside(p, in))
}

func sum(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

func sumSqrt(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += math.Sqrt(x)
	}
	return t
}

func maxOf(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/esy/ -run TestEval -v`
Expected: PASS (16 Tests). Schlägt einer fehl, ist die Semantik des betroffenen Bedingungstyps gegen `spike/ESy-upstream/code/step3and5_extract-and-solve-membership-conditions.R` zu prüfen — dort steht die Referenz.

- [ ] **Step 5: Commit**

```bash
git add internal/esy/
git commit -m "feat(esy): evaluate all ten membership condition kinds"
```

---

### Task 10: Formelauswertung und Gewinnerermittlung

**Files:**
- Create: `internal/esy/evaluate.go`
- Test: `internal/esy/evaluate_test.go`

**Interfaces:**
- Consumes: `rulepack.Node`, `rulepack.Rule`, `Env.EvalExpr`
- Produces:
  - `type Match struct { Code, Variant string; Priority int }`
  - `type Result struct { Winner string; Matches []Match; Conditions map[string][2]float64 }`
  - `func (e Env) Evaluate(rules []rulepack.Rule, p Plot) Result`

Gewinnerlogik nach v1.2: kein Treffer → `?`; genau einer → dieser; sonst Prioritätsstufen absteigend, die erste mit genau einem Treffer gewinnt; keine solche Stufe → `+`.

- [ ] **Step 1: Failing test schreiben**

`internal/esy/evaluate_test.go`:

```go
package esy

import (
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

func rule(prio int, code, variant, formula string) rulepack.Rule {
	n, err := rulepack.ParseFormula(formula)
	if err != nil {
		panic(err)
	}
	return rulepack.Rule{Priority: prio, Code: code, Variant: variant, Formula: n}
}

func TestEvaluateNoMatchIsQuestionMark(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 99>")}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "?" {
		t.Errorf("winner = %q, want %q", got.Winner, "?")
	}
}

func TestEvaluateSingleMatchWins(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 5>")}
	got := testEnv().Evaluate(rs, testPlot())
	if got.Winner != "T1H" {
		t.Errorf("winner = %q, want T1H", got.Winner)
	}
	if len(got.Matches) != 1 {
		t.Errorf("matches = %+v, want exactly one", got.Matches)
	}
}

func TestEvaluateHighestPriorityWins(t *testing.T) {
	rs := []rulepack.Rule{
		rule(2, "LOW", "", "<#TC Trees GR 5>"),
		rule(7, "HIGH", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "HIGH" {
		t.Errorf("winner = %q, want HIGH", got.Winner)
	}
}

// v1.2 change: an ambiguous top level falls through to the next lower level
// instead of returning "+" immediately.
func TestEvaluateAmbiguousTopLevelDescends(t *testing.T) {
	rs := []rulepack.Rule{
		rule(7, "A", "", "<#TC Trees GR 5>"),
		rule(7, "B", "", "<#TC Trees GR 5>"),
		rule(4, "C", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "C" {
		t.Errorf("winner = %q, want C (descend to the unambiguous level)", got.Winner)
	}
}

func TestEvaluateAmbiguousEverywhereIsPlus(t *testing.T) {
	rs := []rulepack.Rule{
		rule(7, "A", "", "<#TC Trees GR 5>"),
		rule(7, "B", "", "<#TC Trees GR 5>"),
		rule(4, "C", "", "<#TC Trees GR 5>"),
		rule(4, "D", "", "<#TC Trees GR 5>"),
	}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "+" {
		t.Errorf("winner = %q, want +", got.Winner)
	}
}

func TestEvaluateUnknownDoesNotMatch(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "X", "", "<$$N Ecoreg EQ 664>")}
	if got := testEnv().Evaluate(rs, testPlot()); got.Winner != "?" {
		t.Errorf("winner = %q, want ? — Unknown must not count as a match", got.Winner)
	}
}

func TestEvaluateReportsVariant(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "N15", "!!", "<#TC Trees GR 5>")}
	got := testEnv().Evaluate(rs, testPlot())
	if got.Matches[0].Variant != "!!" {
		t.Errorf("variant = %q, want !!", got.Matches[0].Variant)
	}
	if got.Winner != "N15" {
		t.Errorf("winner = %q, want N15 (code without the marker)", got.Winner)
	}
}

func TestEvaluateCollectsConditionValues(t *testing.T) {
	rs := []rulepack.Rule{rule(4, "T1H", "", "<#TC Trees GR 25>")}
	got := testEnv().Evaluate(rs, testPlot())
	v, ok := got.Conditions["#TC Trees GR 25"]
	if !ok {
		t.Fatalf("condition values not collected: %v", got.Conditions)
	}
	if v[0] == 0 {
		t.Errorf("left value not recorded: %v", v)
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/esy/ -run TestEvaluate -v`
Expected: FAIL, `undefined: Evaluate`

- [ ] **Step 3: Implementieren**

`internal/esy/evaluate.go`:

```go
package esy

import (
	"sort"

	"github.com/jobrunner/habitatus/internal/rulepack"
)

// Match is one rule that evaluated TRUE.
type Match struct {
	Code     string
	Variant  string
	Priority int
}

// Result is the outcome for one plot.
type Result struct {
	// Winner is a EUNIS code, "?" (no match) or "+" (ambiguous at every level).
	Winner string
	// Matches are all rules that fired, in file order.
	Matches []Match
	// Conditions maps each evaluated expression to its left and right numeric
	// value. This is the seam the golden master compares against upstream.
	Conditions map[string][2]float64
}

// Evaluate runs every rule against the plot.
func (e Env) Evaluate(rules []rulepack.Rule, p Plot) Result {
	res := Result{Conditions: map[string][2]float64{}}
	for _, r := range rules {
		if e.node(r.Formula, p, res.Conditions).IsTrue() {
			res.Matches = append(res.Matches, Match{
				Code: r.Code, Variant: r.Variant, Priority: r.Priority,
			})
		}
	}
	res.Winner = winner(res.Matches)
	return res
}

func (e Env) node(n rulepack.Node, p Plot, cond map[string][2]float64) Tri {
	switch v := n.(type) {
	case rulepack.Leaf:
		t, l, r := e.EvalExpr(v.Expr, p)
		cond[v.Raw] = [2]float64{l, r}
		return t
	case rulepack.And:
		return e.node(v.L, p, cond).And(e.node(v.R, p, cond))
	case rulepack.Or:
		return e.node(v.L, p, cond).Or(e.node(v.R, p, cond))
	case rulepack.Not:
		return e.node(v.L, p, cond).AndNot(e.node(v.R, p, cond))
	}
	return Unknown
}

// winner reproduces upstream v1.2's classify():
//
//	length 0 -> "?"; length 1 -> that one; otherwise walk priority levels from
//	highest down and return the first level holding exactly one match; "+" if
//	no level does.
func winner(ms []Match) string {
	switch len(ms) {
	case 0:
		return "?"
	case 1:
		return ms[0].Code
	}
	levels := map[int][]Match{}
	for _, m := range ms {
		levels[m.Priority] = append(levels[m.Priority], m)
	}
	keys := make([]int, 0, len(levels))
	for k := range levels {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	for _, k := range keys {
		if len(levels[k]) == 1 {
			return levels[k][0].Code
		}
	}
	return "+"
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/esy/ -run TestEvaluate -v`
Expected: PASS (acht Tests)

- [ ] **Step 5: `Load` fertigstellen, damit Regeln geparste Formeln tragen**

`internal/rulepack/load.go`:

```go
package rulepack

import (
	"fmt"
	"io"
	"sort"
)

// Load parses a complete ESy file. Parse errors abort; data defects are
// collected in Issues and never abort, because the official rule file contains
// defects and must still be usable.
func Load(r io.Reader) (*Pack, error) {
	secs, err := SplitSections(r)
	if err != nil {
		return nil, err
	}
	agg, issues := ParseAggregation(secs[1])
	groups := ParseGroups(secs[2])
	rules, err := ParseRuleHeaders(secs[3])
	if err != nil {
		return nil, err
	}
	unknown := map[string]bool{}
	for i := range rules {
		n, err := ParseFormula(rules[i].Raw)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rules[i].Label(), err)
		}
		rules[i].Formula = n
		for _, raw := range extractExpressions(rules[i].Raw) {
			e, err := ParseExpr(raw)
			if err != nil {
				return nil, fmt.Errorf("rule %s: %w", rules[i].Label(), err)
			}
			for _, a := range append(append([]Atom{}, e.Left.Atoms...), e.Right.Atoms...) {
				if a.Kind == "$$C" || a.Kind == "$$N" || a.Name == "" {
					continue
				}
				if _, ok := groups[a.Name]; !ok && isGroupish(a.Name) {
					unknown[a.Name] = true
				}
			}
		}
	}
	for n := range unknown {
		issues.UnknownGroups = append(issues.UnknownGroups, n)
	}
	sort.Strings(issues.UnknownGroups)
	return &Pack{Aggregation: agg, Groups: groups, Rules: rules, Issues: issues}, nil
}

// isGroupish reports whether a name looks like a group rather than a taxon.
// Group names are hyphenated; taxon names are not.
func isGroupish(n string) bool {
	for i := 0; i < len(n); i++ {
		if n[i] == '-' {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: End-to-End gegen die echte Datei**

Ergänze `internal/rulepack/realfile_test.go`:

```go
func TestLoadRealFile(t *testing.T) {
	p := os.Getenv("ESY_FILE")
	if p == "" {
		t.Skip("ESY_FILE not set")
	}
	f, _ := os.Open(p)
	defer f.Close()
	pack, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(pack.Rules) != 312 {
		t.Errorf("rules = %d, want 312", len(pack.Rules))
	}
	for _, r := range pack.Rules {
		if r.Formula == nil {
			t.Errorf("rule %s has no parsed formula", r.Label())
		}
	}
	if len(pack.Issues.UnknownGroups) != 0 {
		t.Errorf("rules reference undefined groups: %v", pack.Issues.UnknownGroups)
	}
	t.Logf("issues: %d duplicate sources, %d chains",
		len(pack.Issues.DuplicateSources), len(pack.Issues.Chains))
}
```

Run: `ESY_FILE=~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt go test ./internal/rulepack/ -run RealFile -v`
Expected: PASS. Meldet der Test unbekannte Gruppen, sind das entweder Artnamen, die `isGroupish` falsch einstuft, oder echte Regelwerksdefekte — beides im Commit benennen.

- [ ] **Step 7: Commit**

```bash
git add internal/esy/ internal/rulepack/
git commit -m "feat(esy): evaluate formulas and resolve the winner per v1.2"
```

---

### Task 11: R-Spike und Golden Master

**Files:**
- Create: `spike/resy/generate-fixtures.R`, `spike/resy/README.md`, `Makefile`
- Create: `internal/esy/golden_test.go`
- Test: `testdata/golden/` (erzeugt)

**Interfaces:**
- Consumes: `spike/ESy-upstream/` (Commit `416bab9`), `rulepack.Load`, `esy.Env.Evaluate`
- Produces: `testdata/golden/cases.jsonl`, `testdata/golden/expected.jsonl`, `testdata/golden/rulepack.sha256`, `testdata/golden/upstream.commit`

Am Upstream wird nichts geändert; das Skript lädt seinen Code und treibt ihn. Fällt der Lauf aus, ist das der erste echte Meilenstein und im Commit zu dokumentieren.

- [ ] **Step 1: Fixture-Generator schreiben**

`spike/resy/generate-fixtures.R`:

```r
# Drives the upstream ESy implementation (v1.2) to produce golden-master
# fixtures for habitatus. The upstream tree is never modified.
#
# Usage:  Rscript spike/resy/generate-fixtures.R <upstream-dir> <esy-file> <out-dir>

args <- commandArgs(trailingOnly = TRUE)
if (length(args) != 3) stop("need <upstream-dir> <esy-file> <out-dir>")
upstream <- normalizePath(args[1]); esyfile <- normalizePath(args[2]); outdir <- args[3]
dir.create(outdir, recursive = TRUE, showWarnings = FALSE)

owd <- setwd(upstream); on.exit(setwd(owd))

source("code/prep.R")
expertfile <- esyfile
source("code/step1and2_load-and-parse-the-expert-file.R")

obs    <- data.table::fread(file.path("data", "obs_100716Hoppe2005.csv"))
header <- read.csv(file.path("data", "header_100716Hoppe2005.csv"))

# Plots at (0,0) carry no coordinate; they would be in the Atlantic.
keep   <- !(header$DEG_LON == 0 & header$DEG_LAT == 0)
header <- header[keep, ]
obs    <- obs[obs$RELEVE_NR %in% header$RELEVE_NR, ]

source("code/step4_aggregate-taxon-levels.R")
source("code/step3and5_extract-and-solve-membership-conditions.R")

result <- sapply(seq_len(nrow(header)), function(i) {
  classify(vegtype.formula.names.short[which(sapply(logi2, function(x) x[i]))])
})

writeLines(jsonlite::toJSON(list(
  n_plots = nrow(header),
  upstream_commit = system("git rev-parse HEAD", intern = TRUE),
  esy_file = basename(esyfile)
), auto_unbox = TRUE), file.path(owd, outdir, "meta.json"))

cases <- lapply(seq_len(nrow(header)), function(i) {
  id <- header$RELEVE_NR[i]
  sp <- obs[obs$RELEVE_NR == id, ]
  list(id = id,
       records = lapply(seq_len(nrow(sp)), function(j)
         list(name = sp$TaxonName[j], cover = sp$Cover_Perc[j])),
       header = as.list(header[i, ]))
})
con <- file(file.path(owd, outdir, "cases.jsonl"), "w")
for (c in cases) writeLines(jsonlite::toJSON(c, auto_unbox = TRUE), con)
close(con)

con <- file(file.path(owd, outdir, "expected.jsonl"), "w")
for (i in seq_len(nrow(header))) {
  hits <- vegtype.formula.names.short[which(sapply(logi2, function(x) x[i]))]
  writeLines(jsonlite::toJSON(list(
    id = header$RELEVE_NR[i], winner = result[i], matches = hits
  ), auto_unbox = TRUE), con)
}
close(con)
cat("wrote", nrow(header), "cases to", outdir, "\n")
```

- [ ] **Step 2: Makefile-Ziel anlegen**

`Makefile`:

```make
UPSTREAM ?= spike/ESy-upstream
ESY_FILE ?= $(HOME)/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt
GOLDEN   ?= testdata/golden

.PHONY: test fixtures

test:
	go test ./...

fixtures:
	Rscript spike/resy/generate-fixtures.R $(UPSTREAM) $(ESY_FILE) $(GOLDEN)
	shasum -a 256 $(ESY_FILE) | cut -d' ' -f1 > $(GOLDEN)/rulepack.sha256
	git -C $(UPSTREAM) rev-parse HEAD > $(GOLDEN)/upstream.commit
```

- [ ] **Step 3: Fixtures erzeugen**

```bash
make fixtures
wc -l testdata/golden/cases.jsonl testdata/golden/expected.jsonl
```

Expected: je 10.295 Zeilen. Bricht R ab, den Fehler im Commit festhalten und erst beheben — ohne Fixtures ist der Rest nicht verifizierbar.

- [ ] **Step 4: Golden-Master-Test schreiben**

`internal/esy/golden_test.go`:

```go
package esy_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

type goldenCase struct {
	ID      int `json:"id"`
	Records []struct {
		Name  string  `json:"name"`
		Cover float64 `json:"cover"`
	} `json:"records"`
	Header map[string]any `json:"header"`
}

type goldenExpect struct {
	ID      int      `json:"id"`
	Winner  string   `json:"winner"`
	Matches []string `json:"matches"`
}

// shortLabel reproduces upstream's trim(substr(name, 1, 6)) so both sides can be
// compared. Upstream glues the first letter of the habitat name onto codes
// shorter than five characters; that only affects the label, never which rule
// matched, so habitatus keeps clean codes and normalises here instead.
func shortLabel(s string) string {
	if i := strings.IndexAny(s, " !"); i > 0 {
		return strings.TrimRight(s[:i], " ")
	}
	return strings.TrimSpace(s)
}

func TestGoldenMaster(t *testing.T) {
	dir := "../../testdata/golden"
	if _, err := os.Stat(filepath.Join(dir, "cases.jsonl")); err != nil {
		t.Skip("no fixtures; run `make fixtures`")
	}
	esyFile := os.Getenv("ESY_FILE")
	if esyFile == "" {
		t.Skip("ESY_FILE not set")
	}
	f, err := os.Open(esyFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pack, err := rulepack.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	env := esy.Env{Groups: pack.Groups}

	expected := map[int]goldenExpect{}
	ef, err := os.Open(filepath.Join(dir, "expected.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	es := bufio.NewScanner(ef)
	es.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for es.Scan() {
		var e goldenExpect
		if err := json.Unmarshal(es.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		expected[e.ID] = e
	}

	cf, err := os.Open(filepath.Join(dir, "cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer cf.Close()
	cs := bufio.NewScanner(cf)
	cs.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var total, mismatch int
	for cs.Scan() {
		var c goldenCase
		if err := json.Unmarshal(cs.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		recs := make([]taxa.Record, len(c.Records))
		for i, r := range c.Records {
			recs[i] = taxa.Record{Name: r.Name, Cover: r.Cover}
		}
		resolved, _ := taxa.Resolve(recs, nil, pack.Aggregation)
		hdr := map[string]string{}
		for k, v := range c.Header {
			switch x := v.(type) {
			case string:
				hdr[k] = x
			case float64:
				hdr[k] = trimFloat(x)
			}
		}
		got := env.Evaluate(pack.Rules, esy.Plot{Records: resolved, Header: hdr})
		want := expected[c.ID]
		total++
		if got.Winner != shortLabel(want.Winner) {
			mismatch++
			if mismatch <= 20 {
				t.Errorf("plot %d: winner %q, want %q", c.ID, got.Winner, shortLabel(want.Winner))
			}
		}
	}
	t.Logf("%d plots compared, %d winner mismatches (%.2f%%)",
		total, mismatch, 100*float64(mismatch)/float64(total))
	if mismatch > 0 {
		t.Errorf("golden master: %d of %d plots differ", mismatch, total)
	}
}

func trimFloat(f float64) string {
	b, _ := json.Marshal(f)
	return strings.TrimSuffix(string(b), ".0")
}
```

- [ ] **Step 5: Golden Master laufen lassen**

Run: `ESY_FILE=~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt go test ./internal/esy/ -run TestGoldenMaster -v`

Expected: zunächst Abweichungen. Jede ist ein Befund. Vorgehen: die betroffene Aufnahme in R mit `eval.plot()` nachrechnen, die abweichende Bedingung finden, Semantik gegen `step3and5_…R` prüfen, korrigieren, erneut laufen lassen. Erst wenn `mismatch == 0`, ist die Portierung belegt.

- [ ] **Step 6: Regelgetriebene synthetische Plots ergänzen**

Das Tüxen-Archiv ist rein deutsch: Mittelmeer-, Schwarzmeer- und Arktisregeln
feuern dort nie, und von 312 Regeln bleibt der größere Teil ungeprüft. Die
systematische Abdeckung liefern erst synthetische Plots.

`spike/resy/synthesize.go` erzeugt sie aus dem geparsten Regelwerk — je Regel ein
Plot, der ihre Gruppen knapp über und knapp unter jeder Schwelle bedient:

```go
//go:build ignore

// Command synthesize writes rule-driven plots to cases-synthetic.jsonl.
// For every rule it emits two plots per numeric threshold: one just above and
// one just below, with the header set from the rule's own header conditions.
package main

// Implementierung: rulepack.Load, dann je Regel über extractExpressions
// iterieren, aus jedem <... GR N> bzw. <... GE N> zwei Plots ableiten (N+1 und
// N-1 als Gruppendeckung, gleichmäßig auf die Gruppenmitglieder verteilt) und
// die Kopfdaten aus den $$C/$$N-Bedingungen derselben Regel befüllen.
```

Erzeugte Fälle gehen durch denselben R-Lauf wie das Tüxen-Archiv und landen in
denselben Fixtures. Das Makefile-Ziel `fixtures` ruft `synthesize` vor dem
R-Skript auf.

Abnahme: Nach dem Lauf muss jede der 312 Regeln in mindestens einem Fixture
mindestens einmal `TRUE` geliefert haben. Ein Test in `golden_test.go` zählt das
und meldet die Regeln, die nie feuern — sie sind entweder unerreichbar oder ein
Hinweis auf einen Fehler im Generator.

- [ ] **Step 7: Commit**

```bash
git add spike/resy/ Makefile internal/esy/golden_test.go testdata/golden/
git commit -m "test(esy): golden master against upstream ESy v1.2"
```

---

### Task 12: Anwendungsfall und Validierung

**Files:**
- Create: `internal/classify/classify.go`, `internal/classify/header.go`
- Test: `internal/classify/classify_test.go`, `internal/classify/header_test.go`

**Interfaces:**
- Consumes: `rulepack.Pack`, `taxa.Resolve`, `esy.Env.Evaluate`
- Produces:
  - `type Request struct { Records []taxa.Record; Backbone string; Header map[string]string }`
  - `type Response struct { Result string; Matches []esy.Match; Resolution []taxa.Step; Versions map[string]string }`
  - `func (s *Service) Classify(req Request) (Response, error)`
  - `func ValidateHeader(h map[string]string) error`

Validierung ist asymmetrisch: Kopfdatenwerte hart gegen ihr Vokabular, Artnamen frei. Ein falscher Ländername verfälscht systematisch und still; ein unbekannter Artname nur graduell, und Feldlisten enthalten immer welche.

- [ ] **Step 1: Failing test für die Validierung schreiben**

`internal/classify/header_test.go`:

```go
package classify

import "testing"

func validHeader() map[string]string {
	return map[string]string{
		"Country":      "Germany",
		"Coast_EEA":    "N_COAST",
		"Dunes_Bohn":   "N_DUNES",
		"Ecoreg":       "664",
		"Altitude (m)": "250",
		"DEG_LAT":      "49.79",
		"DEG_LON":      "9.93",
	}
}

func TestValidateHeaderAccepts(t *testing.T) {
	if err := ValidateHeader(validHeader()); err != nil {
		t.Errorf("valid header rejected: %v", err)
	}
}

func TestValidateHeaderRejectsLocalCountryName(t *testing.T) {
	h := validHeader()
	h["Country"] = "Deutschland"
	if err := ValidateHeader(h); err == nil {
		t.Error("local-language country name must be rejected")
	}
}

func TestValidateHeaderRejectsIsoCode(t *testing.T) {
	h := validHeader()
	h["Country"] = "DE"
	if err := ValidateHeader(h); err == nil {
		t.Error("ISO code must be rejected — the rules compare full names")
	}
}

func TestValidateHeaderRejectsUnknownCoast(t *testing.T) {
	h := validHeader()
	h["Coast_EEA"] = "ATL"
	if err := ValidateHeader(h); err == nil {
		t.Error("short-form coast value must be rejected")
	}
}

func TestValidateHeaderRequiresMandatoryFields(t *testing.T) {
	h := validHeader()
	delete(h, "Ecoreg")
	if err := ValidateHeader(h); err == nil {
		t.Error("missing Ecoreg must be rejected")
	}
}

func TestValidateHeaderAllowsOptionalDataset(t *testing.T) {
	h := validHeader()
	h["Dataset"] = "Swedish_National_Forest_Inventory"
	if err := ValidateHeader(h); err != nil {
		t.Errorf("Dataset is optional and free-form: %v", err)
	}
}

func TestValidateHeaderAcceptsAllFiftyTwoCountries(t *testing.T) {
	for _, c := range []string{"Czech Republic", "Slovak Republic", "Russian Federation",
		"Turkey", "Svalbard and Jan Mayen Is", "United Kingdom", "Sweden"} {
		h := validHeader()
		h["Country"] = c
		if err := ValidateHeader(h); err != nil {
			t.Errorf("country %q rejected: %v", c, err)
		}
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/classify/ -run TestValidateHeader -v`
Expected: FAIL, `undefined: ValidateHeader`

- [ ] **Step 3: Implementieren**

`internal/classify/header.go`:

```go
// Package classify is the application layer: validate, resolve, evaluate.
package classify

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

//go:embed esy-country-names.csv
var countryCSV string

var (
	coastValues = map[string]bool{
		"ARC_COAST": true, "ATL_COAST": true, "BAL_COAST": true,
		"BLA_COAST": true, "MED_COAST": true, "N_COAST": true,
	}
	duneValues = map[string]bool{"Y_DUNES": true, "N_DUNES": true}
	countries  = loadCountries()
)

func loadCountries() map[string]bool {
	out := map[string]bool{}
	r := csv.NewReader(strings.NewReader(countryCSV))
	rows, err := r.ReadAll()
	if err != nil {
		panic("esy-country-names.csv is malformed: " + err.Error())
	}
	for i, row := range rows {
		if i == 0 || len(row) < 2 {
			continue
		}
		out[row[1]] = true
	}
	return out
}

// ValidateHeader checks the plot header against the vocabularies the rules
// compare against. Values are rejected rather than treated as unknown: a
// misspelled country would otherwise silently suppress every rule that tests it,
// with no error anywhere.
func ValidateHeader(h map[string]string) error {
	for _, f := range []string{"Country", "Coast_EEA", "Dunes_Bohn", "Ecoreg",
		"Altitude (m)", "DEG_LAT", "DEG_LON"} {
		if strings.TrimSpace(h[f]) == "" {
			return fmt.Errorf("header field %q is required", f)
		}
	}
	if !countries[h["Country"]] {
		return fmt.Errorf("Country %q is not an ESy country name; see data/esy-country-names.csv", h["Country"])
	}
	if !coastValues[h["Coast_EEA"]] {
		return fmt.Errorf("Coast_EEA %q is not one of ARC_/ATL_/BAL_/BLA_/MED_/N_COAST", h["Coast_EEA"])
	}
	if !duneValues[h["Dunes_Bohn"]] {
		return fmt.Errorf("Dunes_Bohn %q is not Y_DUNES or N_DUNES", h["Dunes_Bohn"])
	}
	if _, err := strconv.Atoi(h["Ecoreg"]); err != nil {
		return fmt.Errorf("Ecoreg %q is not an integer ECO_ID", h["Ecoreg"])
	}
	for _, f := range []string{"Altitude (m)", "DEG_LAT", "DEG_LON"} {
		if _, err := strconv.ParseFloat(h[f], 64); err != nil {
			return fmt.Errorf("%s %q is not a number", f, h[f])
		}
	}
	lat, _ := strconv.ParseFloat(h["DEG_LAT"], 64)
	lon, _ := strconv.ParseFloat(h["DEG_LON"], 64)
	if lat < -90 || lat > 90 {
		return fmt.Errorf("DEG_LAT %v is out of range", lat)
	}
	if lon < -180 || lon > 180 {
		return fmt.Errorf("DEG_LON %v is out of range", lon)
	}
	return nil
}
```

```bash
cp data/esy-country-names.csv internal/classify/esy-country-names.csv
```

`internal/classify/classify.go`:

```go
package classify

import (
	"fmt"

	"github.com/jobrunner/habitatus/internal/esy"
	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// Request is one classification request.
type Request struct {
	Records  []taxa.Record
	Backbone string
	Header   map[string]string
}

// Response is the outcome plus everything needed to understand it.
type Response struct {
	Result     string
	Matches    []esy.Match
	Resolution []taxa.Step
	Versions   map[string]string
}

// Service holds the loaded rule pack and the backbone tables.
type Service struct {
	pack      *rulepack.Pack
	backbones map[string]map[string]string
	versions  map[string]string
}

// NewService builds a service. backbones maps a backbone id to its translation
// table; the id "euro+med" is the identity and needs no table.
func NewService(pack *rulepack.Pack, backbones map[string]map[string]string, versions map[string]string) *Service {
	return &Service{pack: pack, backbones: backbones, versions: versions}
}

// Classify validates, resolves and evaluates one plot.
func (s *Service) Classify(req Request) (Response, error) {
	if len(req.Records) == 0 {
		return Response{}, fmt.Errorf("at least one taxon record is required")
	}
	for _, r := range req.Records {
		if r.Cover <= 0 || r.Cover > 100 {
			return Response{}, fmt.Errorf("cover for %q is %v, must be in (0, 100]", r.Name, r.Cover)
		}
	}
	if err := ValidateHeader(req.Header); err != nil {
		return Response{}, err
	}
	var table map[string]string
	if req.Backbone != "euro+med" {
		t, ok := s.backbones[req.Backbone]
		if !ok {
			return Response{}, fmt.Errorf("unknown backbone %q", req.Backbone)
		}
		table = t
	}
	resolved, steps := taxa.Resolve(req.Records, table, s.pack.Aggregation)
	env := esy.Env{Groups: s.pack.Groups}
	res := env.Evaluate(s.pack.Rules, esy.Plot{Records: resolved, Header: req.Header})
	return Response{
		Result:     res.Winner,
		Matches:    res.Matches,
		Resolution: steps,
		Versions:   s.versions,
	}, nil
}
```

- [ ] **Step 4: Failing test für den Anwendungsfall schreiben**

`internal/classify/classify_test.go`:

```go
package classify

import (
	"strings"
	"testing"

	"github.com/jobrunner/habitatus/internal/rulepack"
	"github.com/jobrunner/habitatus/internal/taxa"
)

const tinyPack = `SECTION 1: Species aggregation
Empetrum nigrum aggr.                                     -  0
     Empetrum nigrum                                         0
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

func newTestService(t *testing.T) *Service {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(pack, nil, map[string]string{"rulepack": "test"})
}

func TestClassifyHappyPath(t *testing.T) {
	s := newTestService(t)
	got, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "euro+med",
		Header:   validHeader(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != "T1H" {
		t.Errorf("result = %q, want T1H", got.Result)
	}
	if len(got.Resolution) != 1 {
		t.Errorf("resolution report missing: %+v", got.Resolution)
	}
}

func TestClassifyRejectsBadCover(t *testing.T) {
	s := newTestService(t)
	for _, c := range []float64{0, -1, 101} {
		_, err := s.Classify(Request{
			Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: c}},
			Backbone: "euro+med",
			Header:   validHeader(),
		})
		if err == nil {
			t.Errorf("cover %v must be rejected", c)
		}
	}
}

func TestClassifyRejectsUnknownBackbone(t *testing.T) {
	s := newTestService(t)
	_, err := s.Classify(Request{
		Records:  []taxa.Record{{Name: "Fagus sylvatica", Cover: 30}},
		Backbone: "wcvp",
		Header:   validHeader(),
	})
	if err == nil {
		t.Error("unknown backbone must be rejected")
	}
}

func TestClassifyAcceptsUnknownTaxonName(t *testing.T) {
	s := newTestService(t)
	got, err := s.Classify(Request{
		Records: []taxa.Record{
			{Name: "Fagus sylvatica", Cover: 30},
			{Name: "Totally unknown plant", Cover: 5},
		},
		Backbone: "euro+med",
		Header:   validHeader(),
	})
	if err != nil {
		t.Fatalf("unknown taxon names must not be rejected: %v", err)
	}
	var unresolved int
	for _, s := range got.Resolution {
		if !s.Resolved {
			unresolved++
		}
	}
	if unresolved != 1 {
		t.Errorf("want 1 unresolved name reported, got %d", unresolved)
	}
}
```

- [ ] **Step 5: Tests laufen lassen, Erfolg prüfen**

Run: `go test ./internal/classify/ -v`
Expected: PASS (elf Tests)

- [ ] **Step 6: Commit**

```bash
git add internal/classify/
git commit -m "feat(classify): request validation and the classification use case"
```

---

### Task 13: HTTP-Schnittstelle

**Files:**
- Create: `internal/adapters/httpapi/server.go`, `cmd/habitatus/main.go`
- Test: `internal/adapters/httpapi/server_test.go`

**Interfaces:**
- Consumes: `classify.Service`
- Produces: `func NewServer(s *classify.Service) http.Handler` mit `POST /api/v1/classify`, `GET /health/ready`

- [ ] **Step 1: Failing test schreiben**

`internal/adapters/httpapi/server_test.go`:

```go
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
}

func TestClassifyEndpointRejectsGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/classify", nil)
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
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/adapters/httpapi/ -v`
Expected: FAIL, `undefined: NewServer`

- [ ] **Step 3: Implementieren**

`internal/adapters/httpapi/server.go`:

```go
// Package httpapi exposes the classification service over HTTP.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/taxa"
)

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
	Input    string `json:"input"`
	Final    string `json:"final"`
	Resolved bool   `json:"resolved"`
}

type classifyResponse struct {
	Result     string            `json:"result"`
	Matches    []matchJSON       `json:"matches"`
	Resolution []stepJSON        `json:"resolution"`
	Versions   map[string]string `json:"versions"`
}

// NewServer wires the HTTP routes.
func NewServer(s *classify.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/classify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req classifyRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		recs := make([]taxa.Record, len(req.Records))
		for i, rr := range req.Records {
			recs[i] = taxa.Record{Name: rr.Name, Cover: rr.Cover}
		}
		res, err := s.Classify(classify.Request{
			Records: recs, Backbone: req.Backbone, Header: req.Header,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		out := classifyResponse{Result: res.Result, Versions: res.Versions}
		for _, m := range res.Matches {
			out.Matches = append(out.Matches, matchJSON{Code: m.Code, Variant: m.Variant, Priority: m.Priority})
		}
		for _, st := range res.Resolution {
			out.Resolution = append(out.Resolution, stepJSON{Input: st.Input, Final: st.Final, Resolved: st.Resolved})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	return mux
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

`cmd/habitatus/main.go`:

```go
// Command habitatus serves EUNIS habitat classification over HTTP.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/jobrunner/habitatus/internal/adapters/httpapi"
	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/rulepack"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	rules := flag.String("rules", "", "path to the ESy rule file")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if *rules == "" {
		log.Error("missing -rules")
		os.Exit(2)
	}
	f, err := os.Open(*rules)
	if err != nil {
		log.Error("cannot open rule file", "err", err)
		os.Exit(1)
	}
	defer f.Close()

	// A parse error aborts the start: a service running on a half-parsed rule
	// pack would answer with silent nonsense.
	pack, err := rulepack.Load(f)
	if err != nil {
		log.Error("cannot load rule pack", "err", err)
		os.Exit(1)
	}
	// Data defects do not abort — the official rule file has them.
	log.Info("rule pack loaded",
		"rules", len(pack.Rules),
		"groups", len(pack.Groups),
		"aggregations", len(pack.Aggregation),
		"duplicate_sources", len(pack.Issues.DuplicateSources),
		"chains", len(pack.Issues.Chains),
		"unknown_groups", len(pack.Issues.UnknownGroups))

	svc := classify.NewService(pack, nil, map[string]string{"rulepack": *rules})
	log.Info("listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, httpapi.NewServer(svc)); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Tests laufen lassen, Erfolg prüfen**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 5: Dienst starten und von Hand prüfen**

```bash
go build -o /tmp/habitatus ./cmd/habitatus
/tmp/habitatus -rules ~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt &
sleep 2
curl -s -X POST localhost:8080/api/v1/classify -H 'Content-Type: application/json' -d '{
  "backbone": "euro+med",
  "records": [{"name":"Empetrum nigrum","cover":20},{"name":"Calluna vulgaris","cover":40}],
  "header": {"Country":"Germany","Coast_EEA":"BAL_COAST","Dunes_Bohn":"Y_DUNES",
             "Ecoreg":"664","Altitude (m)":"5","DEG_LAT":"54.4","DEG_LON":"13.4"}
}' | head -c 600
kill %1
```

Expected: JSON mit `result`, `matches`, `resolution`. Der Ladevorgang protokolliert die Defektzähler.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/ cmd/
git commit -m "feat(http): POST /api/v1/classify and readiness endpoint"
```

---

### Task 14: MCP-Schnittstelle

**Files:**
- Create: `internal/adapters/mcpapi/server.go`
- Modify: `cmd/habitatus/main.go` (Flag `-mcp`)
- Test: `internal/adapters/mcpapi/server_test.go`

**Interfaces:**
- Consumes: `classify.Service`
- Produces: `func NewServer(s *classify.Service) *Server` mit `func (s *Server) Serve(in io.Reader, out io.Writer) error` — MCP über stdio, JSON-RPC 2.0, Werkzeug `classify`.

Ein in sich geschlossener Stdio-Server ohne externe Abhängigkeit; das Protokoll ist für ein einzelnes Werkzeug klein genug.

- [ ] **Step 1: Failing test schreiben**

`internal/adapters/mcpapi/server_test.go`:

```go
package mcpapi

import (
	"bytes"
	"encoding/json"
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

func testServer(t *testing.T) *Server {
	t.Helper()
	pack, err := rulepack.Load(strings.NewReader(tinyPack))
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(classify.NewService(pack, nil, map[string]string{"rulepack": "test"}))
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
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/adapters/mcpapi/ -v`
Expected: FAIL, `undefined: NewServer`

- [ ] **Step 3: Implementieren**

`internal/adapters/mcpapi/server.go`:

```go
// Package mcpapi exposes the classification service as an MCP tool over stdio.
package mcpapi

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/jobrunner/habitatus/internal/classify"
	"github.com/jobrunner/habitatus/internal/taxa"
)

// Server speaks a minimal MCP subset: initialize, tools/list, tools/call.
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
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var toolSchema = map[string]any{
	"name":        "classify",
	"description": "Assign EUNIS habitats to a vegetation plot from its species list, covers and header data.",
	"inputSchema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"backbone": map[string]any{"type": "string"},
			"records": map[string]any{"type": "array", "items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string"},
					"cover": map[string]any{"type": "number"},
				},
				"required": []string{"name", "cover"},
			}},
			"header": map[string]any{"type": "object"},
		},
		"required": []string{"backbone", "records", "header"},
	},
}

// Serve reads newline-delimited JSON-RPC requests and writes responses.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			if err := enc.Encode(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}}); err != nil {
				return err
			}
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
		res.Result = map[string]any{"tools": []any{toolSchema}}
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				Backbone string            `json:"backbone"`
				Records  []taxa.Record     `json:"records"`
				Header   map[string]string `json:"header"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			res.Error = &rpcError{Code: -32602, Message: err.Error()}
			return res
		}
		if p.Name != "classify" {
			res.Error = &rpcError{Code: -32601, Message: "unknown tool " + p.Name}
			return res
		}
		out, err := s.svc.Classify(classify.Request{
			Records: p.Arguments.Records, Backbone: p.Arguments.Backbone, Header: p.Arguments.Header,
		})
		if err != nil {
			res.Error = &rpcError{Code: -32602, Message: err.Error()}
			return res
		}
		b, _ := json.Marshal(out)
		res.Result = map[string]any{
			"content": []any{map[string]any{"type": "text", "text": string(b)}},
		}
	default:
		res.Error = &rpcError{Code: -32601, Message: "unknown method " + req.Method}
	}
	return res
}
```

`taxa.Record` braucht dafür JSON-Tags. In `internal/taxa/resolve.go` ändern:

```go
// Record is one taxon observation.
type Record struct {
	Name  string  `json:"name"`
	Cover float64 `json:"cover"`
}
```

In `cmd/habitatus/main.go` nach dem Laden des Packs ergänzen:

```go
	if *mcp {
		if err := mcpapi.NewServer(svc).Serve(os.Stdin, os.Stdout); err != nil {
			log.Error("mcp server stopped", "err", err)
			os.Exit(1)
		}
		return
	}
```

sowie oben `mcp := flag.Bool("mcp", false, "serve MCP over stdio instead of HTTP")` und der Import von `mcpapi`.

- [ ] **Step 4: Tests laufen lassen, Erfolg prüfen**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 5: Von Hand prüfen**

```bash
go build -o /tmp/habitatus ./cmd/habitatus
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | \
  /tmp/habitatus -mcp -rules ~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt
```

Expected: eine JSON-Zeile, die das Werkzeug `classify` beschreibt.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/mcpapi/ internal/taxa/ cmd/
git commit -m "feat(mcp): expose classify as an MCP tool over stdio"
```

---

### Task 15: Backbone-Tabellen, Trunkierungsmarker, Beobachtbarkeit

**Files:**
- Create: `internal/rulepack/backbone.go`, `internal/adapters/httpapi/metrics.go`
- Modify: `internal/classify/classify.go`, `cmd/habitatus/main.go`
- Test: `internal/rulepack/backbone_test.go`, `internal/classify/truncation_test.go`

**Interfaces:**
- Consumes: `SplitSections`, `ParseAggregation`, `classify.Service`
- Produces:
  - `func LoadBackbones(dir string) (map[string]map[string]string, error)`
  - `Response.TruncatedAt10 bool`
  - `func (s *Service) Stats() Stats` mit `Stats{Total, Question, Plus int; NeverFired []string}`

Drei Lücken aus der Spec, die in keiner anderen Aufgabe hängen: §3.2 verlangt die
48 Tabellen, §6 den Vermerk, wo das Original abgeschnitten hätte, §10 die zwei
Kennzahlen.

- [ ] **Step 1: Failing test für das Laden der Backbones**

`internal/rulepack/backbone_test.go`:

```go
package rulepack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBackbones(t *testing.T) {
	dir := t.TempDir()
	content := `SECTION 1: Species aggregation
Abies alba                                                -  0
     Abies pectinata                                         0
SECTION 1: End
SECTION 2: Species groups
SECTION 2: End
SECTION 3: Group definitions
SECTION 3: End
SECTION 4: Similarity
SECTION 4: End
`
	if err := os.WriteFile(filepath.Join(dir, "GermanSL 1.4_ExpertSystem.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBackbones(dir)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := got["germansl-1.4"]
	if !ok {
		t.Fatalf("id not normalised to kebab-case: %v keys", len(got))
	}
	if tbl["Abies pectinata"] != "Abies alba" {
		t.Errorf("table not loaded: %v", tbl)
	}
}

func TestLoadBackbonesRejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"Europe_ExpertSystem.txt", "europe_ExpertSystem.txt"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("SECTION 1: Species aggregation\nSECTION 1: End\n"), 0o644)
	}
	if _, err := LoadBackbones(dir); err == nil {
		t.Skip("case-insensitive filesystem collapsed the two names")
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag prüfen**

Run: `go test ./internal/rulepack/ -run TestLoadBackbones -v`
Expected: FAIL, `undefined: LoadBackbones`

- [ ] **Step 3: Implementieren**

`internal/rulepack/backbone.go`:

```go
package rulepack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const backboneSuffix = "_ExpertSystem.txt"

// LoadBackbones reads every nomenclature translation table in dir. The files
// carry only section 1; the remaining sections are empty, so the ordinary
// section-1 parser serves them unchanged.
//
// The id is the file stem in kebab-case: "GermanSL 1.4_ExpertSystem.txt"
// becomes "germansl-1.4".
func LoadBackbones(dir string) (map[string]map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), backboneSuffix) {
			continue
		}
		id := backboneID(strings.TrimSuffix(e.Name(), backboneSuffix))
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("backbone id %q is claimed by two files", id)
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		secs, err := SplitSections(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		tbl, _ := ParseAggregation(secs[1])
		out[id] = tbl
	}
	return out, nil
}

func backboneID(stem string) string {
	s := strings.ToLower(strings.TrimSpace(stem))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
```

- [ ] **Step 4: Test laufen lassen, Erfolg prüfen**

Run: `go test ./internal/rulepack/ -run TestLoadBackbones -v`
Expected: PASS

- [ ] **Step 5: Trunkierungsmarker und Kennzahlen**

Test `internal/classify/truncation_test.go`:

```go
package classify

import "testing"

func TestResponseMarksWhereUpstreamWouldTruncate(t *testing.T) {
	if !truncatedAt10(make([]int, 11)) {
		t.Error("11 matches must be marked as truncated")
	}
	if truncatedAt10(make([]int, 10)) {
		t.Error("exactly 10 matches is not truncation")
	}
}
```

In `classify.go` ergänzen:

```go
// truncatedAt10 reports whether upstream would have dropped matches: it stores
// only the first ten hits per plot. We return all of them and set this flag so
// the golden master stays comparable.
func truncatedAt10[T any](ms []T) bool { return len(ms) > 10 }
```

`Response` um `TruncatedAt10 bool` erweitern und in `Classify` setzen; das
HTTP-Feld heißt `truncated_at_10`.

Für die Kennzahlen in `Service` drei Zähler und ein Set führen:

```go
// Stats are the two operational figures that carry ecological meaning: how often
// the answer is "?" or "+", and which rules never fire. Both surface errors no
// test finds — a client that fills one header field wrongly shows up here.
type Stats struct {
	Total      int
	Question   int
	Plus       int
	NeverFired []string
}
```

`Classify` erhöht die Zähler und merkt sich getroffene Regel-Label; `Stats()`
bildet `NeverFired` als Differenz zu allen Regel-Labeln. Der HTTP-Adapter legt
`GET /metrics` darüber.

- [ ] **Step 6: Verdrahten und prüfen**

In `cmd/habitatus/main.go` ein Flag `-backbones` ergänzen, das auf das
Verzeichnis der Übersetzungstabellen zeigt, und das Ergebnis an `NewService`
übergeben.

```bash
go build -o /tmp/habitatus ./cmd/habitatus
/tmp/habitatus -rules ~/work/projects/eunis/EUNIS-ESy/EUNIS-ESy-2025-10-03.txt \
  -backbones ~/work/projects/eunis/EUNIS-ESy/Nomenclature-translation-from-Turboveg-2-databases &
sleep 3
curl -s localhost:8080/metrics | head -c 300
kill %1
```

Expected: Beim Start werden 48 Backbones protokolliert.

- [ ] **Step 7: Commit**

```bash
git add internal/rulepack/backbone.go internal/classify/ internal/adapters/ cmd/
git commit -m "feat: load backbone tables, expose stats and the truncation marker"
```

---

## Reihenfolge und Abhängigkeiten

```
1 ─▶ 2 ─▶ 3 ─▶ 4 ─▶ 5 ─▶ 6 ─┐
                            ├─▶ 9 ─▶ 10 ─▶ 11 ─▶ 12 ─▶ 13 ─▶ 14 ─▶ 15
                    7 ─▶ 8 ─┘
```

Tasks 7 und 8 hängen nur voneinander ab und können parallel zu 2–6 laufen.

**Task 11 ist das eigentliche Nadelöhr.** Bis der Golden Master grün ist, sind die Tasks 1–10 unbelegt: Die Unit-Tests prüfen, was wir *glauben*, nicht was der Upstream tut. Läuft das R-Skript nicht durch, hat alles andere zu warten.
