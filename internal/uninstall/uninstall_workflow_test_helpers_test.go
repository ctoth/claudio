package uninstall

import (
	"testing"

	"claudio.click/internal/install"
)

// swapAgentResolver replaces the package-level agentResolver for the duration
// of the test, restoring the previous value via t.Cleanup. This lets tests
// inject a TempDir-scoped path without touching the workflow's public
// signature or constructing a stub agent.
func swapAgentResolver(t *testing.T, fn func(install.Agent, string) (string, error)) {
	t.Helper()
	prev := agentResolver
	agentResolver = fn
	t.Cleanup(func() { agentResolver = prev })
}

// fixedPathResolver returns a resolver that ignores the agent/scope and
// always returns the given path. Useful for routing the workflow at a
// t.TempDir() file.
func fixedPathResolver(path string) func(install.Agent, string) (string, error) {
	return func(install.Agent, string) (string, error) { return path, nil }
}
