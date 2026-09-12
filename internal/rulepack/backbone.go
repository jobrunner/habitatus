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
//
// The second return value carries each table's parse Issues (duplicate
// source names, chains — UnknownGroups is always empty here, since a
// backbone file has no rules to reference a group). These tables are not
// merely as defective as the main rule pack; measured against the real
// files, GermanSL 1.4 alone carries 26 duplicate source names and 51
// chains. Load keeps and logs this class of defect for the main rule pack,
// so LoadBackbones does the same instead of discarding it.
func LoadBackbones(dir string) (map[string]map[string]string, map[string]Issues, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]map[string]string{}
	issues := map[string]Issues{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), backboneSuffix) {
			continue
		}
		id := backboneID(strings.TrimSuffix(e.Name(), backboneSuffix))
		if _, dup := out[id]; dup {
			return nil, nil, fmt.Errorf("backbone id %q is claimed by two files", id)
		}
		//nolint:gosec // dir is the operator-supplied backbone directory and e.Name() comes from reading it
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, nil, err
		}
		secs, err := SplitSections(f)
		_ = f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, ok := secs[1]; !ok {
			return nil, nil, fmt.Errorf("%s: no section 1 (nomenclature translation table)", e.Name())
		}
		tbl, iss := ParseAggregation(secs[1])
		out[id] = tbl
		issues[id] = iss
	}
	return out, issues, nil
}

func backboneID(stem string) string {
	s := strings.ToLower(strings.TrimSpace(stem))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
