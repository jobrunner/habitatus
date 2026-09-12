package classify

import (
	"os"
	"sort"
	"testing"
)

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

// TestCountryTableParsesExactlyFiftyTwoCountries guards against a silent
// truncation of the embedded table: a future edit that drops or merges a row
// must fail this count, not just "a country stopped being valid" discovered
// much later.
func TestCountryTableParsesExactlyFiftyTwoCountries(t *testing.T) {
	got, err := parseCountries(countryCSV)
	if err != nil {
		t.Fatalf("embedded table failed to parse: %v", err)
	}
	if len(got) != 52 {
		t.Errorf("want 52 countries, got %d: %v", len(got), got)
	}
}

// TestEmbeddedCountryTableMatchesSource guards against the embedded copy
// (forced by go:embed's package-directory restriction) drifting from
// data/esy-country-names.csv, the source of truth. Without this, the two
// silently diverge and the failure mode looks like "this country stopped
// being valid" long after the actual cause.
func TestEmbeddedCountryTableMatchesSource(t *testing.T) {
	source, err := os.ReadFile("../../data/esy-country-names.csv")
	if err != nil {
		t.Fatalf("could not read source table: %v", err)
	}
	if string(source) != countryCSV {
		t.Error("internal/classify/esy-country-names.csv has drifted from data/esy-country-names.csv; copy the source file over the embedded one")
	}
}

func TestParseCountriesRejectsTruncatedRow(t *testing.T) {
	_, err := parseCountries("iso_alpha2,esy_country,note\nDE,Germany\n")
	if err == nil {
		t.Error("a row with too few fields must be rejected, not silently skipped")
	}
}

func TestParseCountriesRejectsEmptyCode(t *testing.T) {
	_, err := parseCountries("iso_alpha2,esy_country,note\n,Germany,\n")
	if err == nil {
		t.Error("an empty ISO code must be rejected")
	}
}

func TestParseCountriesRejectsEmptyName(t *testing.T) {
	_, err := parseCountries("iso_alpha2,esy_country,note\nDE,,\n")
	if err == nil {
		t.Error("an empty country name must be rejected")
	}
}

func TestParseCountriesRejectsDuplicateCode(t *testing.T) {
	_, err := parseCountries("iso_alpha2,esy_country,note\nDE,Germany,\nDE,Austria,\n")
	if err == nil {
		t.Error("a duplicate ISO code must be rejected")
	}
}

func TestParseCountriesRejectsDuplicateName(t *testing.T) {
	_, err := parseCountries("iso_alpha2,esy_country,note\nDE,Germany,\nAT,Germany,\n")
	if err == nil {
		t.Error("a duplicate country name must be rejected")
	}
}

func TestCountryNamesIsSortedAndMatchesValidateHeader(t *testing.T) {
	names := CountryNames()
	if len(names) == 0 {
		t.Fatal("CountryNames must not be empty")
	}
	if !sort.StringsAreSorted(names) {
		t.Error("CountryNames must be sorted")
	}
	for _, n := range names {
		h := validHeader()
		h["Country"] = n
		if err := ValidateHeader(h); err != nil {
			t.Errorf("CountryNames returned %q, but ValidateHeader rejects it: %v", n, err)
		}
	}
}

func TestCountryNamesReturnsACopy(t *testing.T) {
	names := CountryNames()
	names[0] = "mutated"
	again := CountryNames()
	if again[0] == "mutated" {
		t.Error("CountryNames must return a fresh copy, not a view onto shared state")
	}
}

func TestCoastValuesMatchesValidateHeader(t *testing.T) {
	values := CoastValues()
	if len(values) != 6 {
		t.Fatalf("want 6 Coast_EEA values, got %d: %v", len(values), values)
	}
	for _, v := range values {
		h := validHeader()
		h["Coast_EEA"] = v
		if err := ValidateHeader(h); err != nil {
			t.Errorf("CoastValues returned %q, but ValidateHeader rejects it: %v", v, err)
		}
	}
}

func TestDuneValuesMatchesValidateHeader(t *testing.T) {
	values := DuneValues()
	if len(values) != 2 {
		t.Fatalf("want 2 Dunes_Bohn values, got %d: %v", len(values), values)
	}
	for _, v := range values {
		h := validHeader()
		h["Dunes_Bohn"] = v
		if err := ValidateHeader(h); err != nil {
			t.Errorf("DuneValues returned %q, but ValidateHeader rejects it: %v", v, err)
		}
	}
}
