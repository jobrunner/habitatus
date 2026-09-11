// Package classify is the application layer: validate, resolve, evaluate.
package classify

import (
	_ "embed"
	"encoding/csv"
	"fmt"
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
	countries  = loadCountries()
)

func loadCountries() map[string]bool {
	out := map[string]bool{}
	r := csv.NewReader(strings.NewReader(countryCSV))
	// The note column has unquoted bare quotes in a few rows (e.g. the
	// Czech Republic and United Kingdom entries); only the first two
	// columns matter here, so tolerate that rather than reject the file.
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
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
