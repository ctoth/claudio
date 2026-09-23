package install

import "testing"

// TestProjectScopePathsListedOnce pins that project-scope candidates are
// unique: filepath.Join(".", x) == x, so listing both forms duplicated
// every candidate.
func TestProjectScopePathsListedOnce(t *testing.T) {
	finders := map[string]func(string) ([]string, error){
		"claude":  FindClaudeSettingsPaths,
		"codex":   FindCodexHooksPaths,
		"gemini":  FindGeminiSettingsPaths,
		"qwen":    FindQwenSettingsPaths,
		"copilot": FindCopilotSettingsPaths,
	}
	for name, find := range finders {
		t.Run(name, func(t *testing.T) {
			paths, err := find(ScopeProject)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, p := range paths {
				if seen[p] {
					t.Errorf("path %q listed more than once in %v", p, paths)
				}
				seen[p] = true
			}
		})
	}
}
