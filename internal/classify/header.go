// Package classify is the application layer: validate, resolve, evaluate.
package classify

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"math"
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

// The sentinels for "this plot is not on a coast" and "not on a dune". ortus
// delivers them for every inland point, so they are the common case.
const (
	valueNoCoast = "N_COAST"
	valueNoDunes = "N_DUNES"
)

var (
	coastValues = map[string]bool{
		"ARC_COAST": true, "ATL_COAST": true, "BAL_COAST": true,
		"BLA_COAST": true, "MED_COAST": true, valueNoCoast: true,
	}
	duneValues = map[string]bool{"Y_DUNES": true, valueNoDunes: true}
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

// Header field names. They are the ESy column headers verbatim — the rule
// file compares against these spellings, so they are wire format, not labels.
const (
	fieldCountry  = "Country"
	fieldCoast    = "Coast_EEA"
	fieldDunes    = "Dunes_Bohn"
	fieldEcoreg   = "Ecoreg"
	fieldAltitude = "Altitude (m)"
	fieldLat      = "DEG_LAT"
	fieldLon      = "DEG_LON"
)

// ValidateHeader checks the plot header against the vocabularies the rules
// compare against. Values are rejected rather than treated as unknown: a
// misspelled country would otherwise silently suppress every rule that tests it,
// with no error anywhere.
func ValidateHeader(h map[string]string) error {
	for _, f := range []string{fieldCountry, fieldCoast, fieldDunes, fieldEcoreg,
		fieldAltitude, fieldLat, fieldLon} {
		if strings.TrimSpace(h[f]) == "" {
			return invalidf("header field %q is required", f)
		}
	}
	if err := validateEnums(h); err != nil {
		return err
	}
	return validateNumbers(h)
}

// validateEnums checks the four fields whose values come from a closed set.
func validateEnums(h map[string]string) error {
	if !countries[h[fieldCountry]] {
		return invalidf("Country %q is not an ESy country name; see data/esy-country-names.csv", h[fieldCountry])
	}
	if !coastValues[h[fieldCoast]] {
		return invalidf("Coast_EEA %q is not one of ARC_/ATL_/BAL_/BLA_/MED_/N_COAST", h[fieldCoast])
	}
	if !duneValues[h[fieldDunes]] {
		return invalidf("Dunes_Bohn %q is not Y_DUNES or N_DUNES", h[fieldDunes])
	}
	if _, err := strconv.Atoi(h[fieldEcoreg]); err != nil {
		return invalidf("Ecoreg %q is not an integer ECO_ID", h[fieldEcoreg])
	}
	return nil
}

// validateNumbers parses the three numeric fields and range-checks them.
// Non-finite values are rejected explicitly: NaN compares false against every
// bound, so a range check alone would let it through and it would then poison
// every group total it reaches.
func validateNumbers(h map[string]string) error {
	// Three locals, not a map: this runs on every classification request, and
	// a map here allocates on the heap for no gain.
	var alt, lat, lon float64
	for _, f := range []string{fieldAltitude, fieldLat, fieldLon} {
		v, err := strconv.ParseFloat(h[f], 64)
		if err != nil {
			return invalidf("%s %q is not a number", f, h[f])
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return invalidf("%s %q must be a finite number", f, h[f])
		}
		switch f {
		case fieldAltitude:
			alt = v
		case fieldLat:
			lat = v
		case fieldLon:
			lon = v
		}
	}
	if lat < -90 || lat > 90 {
		return invalidf("DEG_LAT %v is out of range", lat)
	}
	if lon < -180 || lon > 180 {
		return invalidf("DEG_LON %v is out of range", lon)
	}
	// -500 to 9000 metres is generous — the rule file's own thresholds top
	// out at 1500 — but still catches a transposed or garbage value.
	if alt < -500 || alt > 9000 {
		return invalidf("Altitude (m) %v is out of range", alt)
	}
	return nil
}
