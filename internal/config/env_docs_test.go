package config

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var envVarToken = regexp.MustCompile("`(CLAUDIO_[A-Z_]+)`")

// documentedEnvVars returns the CLAUDIO_* names in the "## Environment
// Variables" section of docs/configuration.md (the reference table) plus any
// mentioned in docs/cli-reference.md. The test-only section is excluded.
func documentedEnvVars(t *testing.T) []string {
	t.Helper()
	cfgDoc, err := os.ReadFile("../../docs/configuration.md")
	if err != nil {
		t.Fatalf("read configuration.md: %v", err)
	}
	// Git may check docs out with CRLF (core.autocrlf on Windows CI).
	section := strings.ReplaceAll(string(cfgDoc), "\r\n", "\n")
	start := strings.Index(section, "\n## Environment Variables\n")
	if start < 0 {
		t.Fatal("configuration.md has no '## Environment Variables' section")
	}
	section = section[start+1:]
	if end := strings.Index(section[3:], "\n## "); end >= 0 {
		section = section[:end+3]
	}

	cliDoc, err := os.ReadFile("../../docs/cli-reference.md")
	if err != nil {
		t.Fatalf("read cli-reference.md: %v", err)
	}

	var names []string
	for _, text := range []string{section, string(cliDoc)} {
		for _, m := range envVarToken.FindAllStringSubmatch(text, -1) {
			if !slices.Contains(names, m[1]) {
				names = append(names, m[1])
			}
		}
	}
	return names
}

func TestEnvVarTable_MatchesDocs(t *testing.T) {
	documented := documentedEnvVars(t)
	table := EnvVarNames()

	if len(documented) == 0 {
		t.Fatal("found no documented CLAUDIO_* variables; doc layout changed?")
	}
	for _, name := range documented {
		if !slices.Contains(table, name) {
			t.Errorf("%s is documented but not in the env override table", name)
		}
	}
	for _, name := range table {
		if !slices.Contains(documented, name) {
			t.Errorf("%s is in the env override table but not documented in docs/configuration.md", name)
		}
	}
}
