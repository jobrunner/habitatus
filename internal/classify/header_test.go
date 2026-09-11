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
