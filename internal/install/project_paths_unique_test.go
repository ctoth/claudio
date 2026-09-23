package install

import "testing"

// TestProjectScopePathsListedOnce pins that project-scope candidates are
// unique: filepath.Join(".", x) == x, so listing both forms duplicated
// every candidate.
func TestProjectScopePathsListedOnce(t *testing.T) {
	for _, agent := range ConcreteAgents() {
		t.Run(string(agent), func(t *testing.T) {
			paths, err := agent.ConfigPaths(ScopeProject)
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
