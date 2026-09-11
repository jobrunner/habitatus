// Package classify is the application layer: validate, resolve, evaluate.
package classify

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// esy-country-names.csv is a copy of the repository's data/esy-country-names.csv
// — go:embed cannot reach outside this package directory. Keep the two files
// in step: this copy is what ships, data/esy-country-names.csv is the source
// of truth.
//
//go:embed esy-country-names.csv
var countryCSV string

var (
	coastValues = map[string]bool{
		"ARC_COAST": true, "ATL_COAST": true, "BAL_COAST": true,
		"BLA_COAST": true, "MED_COAST": true, "N_COAST": true,
	}
	duneValues = map[string]bool{"Y_DUNES": true, "N_DUNES": true}
	countries  = mustLoadCountries(countryCSV)
)

// CountryNames returns the ESy country vocabulary — the exact English
// names ValidateHeader accepts for the "Country" header field — sorted and
// as a fresh copy on every call, so a caller cannot mutate this package's
// state through it. It is the single source of truth other adapters (e.g.
// mcpapi's tool schema) should build their own enums from, rather than
// keeping a second copy that can drift from ValidateHeader.
func CountryNames() []string { return sortedKeys(countries) }

// CoastValues returns the six permitted "Coast_EEA" header values, sorted
// and as a fresh copy on every call. See CountryNames for why callers
// should use this instead of hardcoding the list.
func CoastValues() []string { return sortedKeys(coastValues) }

// DuneValues returns the two permitted "Dunes_Bohn" header values, sorted
// and as a fresh copy on every call. See CountryNames for why callers
// should use this instead of hardcoding the list.
func DuneValues() []string { return sortedKeys(duneValues) }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// mustLoadCountries wraps parseCountries for package initialisation. The
// country table is a build-time asset with no sensible runtime recovery, so
// any defect fails loudly here rather than silently shrinking the
// valid-country set — a shrunk set is the worst failure mode, because it
// would make a legitimate country quietly unusable, with no error anywhere.
func mustLoadCountries(csvText string) map[string]bool {
	out, err := parseCountries(csvText)
	if err != nil {
		panic("esy-country-names.csv is malformed: " + err.Error())
	}
	return out
}

// parseCountries parses the country table strictly: exactly three fields per
// row (a mismatched row, e.g. a truncated one, is an error, not a silent
// skip), a non-empty ISO code and country name on every row, and no
// duplicate code or name.
func parseCountries(csvText string) (map[string]bool, error) {
	r := csv.NewReader(strings.NewReader(csvText))
	// Default settings: the field count is fixed from the header row (3) and
	// any row with a different count is a hard error, as is any unescaped
	// quote.
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	out := map[string]bool{}
	byCode := map[string]bool{}
	for i, row := range rows {
		if i == 0 {
			continue // header
		}
		line := i + 1 // 1-based, matching the file
		if len(row) != 3 {
			return nil, fmt.Errorf("line %d: want 3 fields, got %d", line, len(row))
		}
		code, name := strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
		if code == "" {
			return nil, fmt.Errorf("line %d: empty ISO code", line)
		}
		if name == "" {
			return nil, fmt.Errorf("line %d: empty country name", line)
		}
		if byCode[code] {
			return nil, fmt.Errorf("line %d: duplicate ISO code %q", line, code)
		}
		if out[name] {
			return nil, fmt.Errorf("line %d: duplicate country name %q", line, name)
		}
		byCode[code] = true
		out[name] = true
	}
	return out, nil
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
