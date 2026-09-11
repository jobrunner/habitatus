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
// becomes "germansl-1.4". Two files deriving the same id is an error, not a
// silent overwrite.
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
