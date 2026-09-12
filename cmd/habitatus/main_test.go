package main

import (
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

func TestResolveConfigPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want config
	}{
		{
			name: "built-in defaults, nothing set",
			env:  nil,
			args: nil,
			want: config{addr: ":8080", rulesPath: "", backbonesPath: "", modeName: "repaired"},
		},
		{
			name: "environment overrides built-in defaults",
			env: map[string]string{
				"HABITATUS_ADDR":      ":9090",
				"HABITATUS_RULES":     "/env/rules.txt",
				"HABITATUS_BACKBONES": "/env/backbones",
				"HABITATUS_MODE":      "faithful",
			},
			args: nil,
			want: config{
				addr:          ":9090",
				rulesPath:     "/env/rules.txt",
				backbonesPath: "/env/backbones",
				modeName:      "faithful",
			},
		},
		{
			name: "flag overrides environment",
			env: map[string]string{
				"HABITATUS_ADDR":  ":9090",
				"HABITATUS_RULES": "/env/rules.txt",
				"HABITATUS_MODE":  "faithful",
			},
			args: []string{"-mode", "repaired", "-rules", "/flag/rules.txt"},
			want: config{
				addr:      ":9090", // untouched by a flag, environment still wins here
				rulesPath: "/flag/rules.txt",
				modeName:  "repaired",
			},
		},
		{
			name: "mcp flag has no environment equivalent, defaults false",
			env:  nil,
			args: []string{"-mcp"},
			want: config{addr: ":8080", modeName: "repaired", mcp: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveConfig(envMap(tc.env), tc.args)
			if err != nil {
				t.Fatalf("resolveConfig: %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveConfig(%v, %v) = %+v, want %+v", tc.env, tc.args, got, tc.want)
			}
		})
	}
}
