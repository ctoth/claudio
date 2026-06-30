package main

import "testing"

func TestParseCoverage(t *testing.T) {
	got, err := parseCoverage("ok  example.test/pkg  0.1s  coverage: 94.1% of statements\n")
	if err != nil {
		t.Fatalf("parseCoverage returned error: %v", err)
	}
	if got != 94.1 {
		t.Fatalf("coverage = %.1f, want 94.1", got)
	}
}

func TestParseCoverageRejectsMissingCoverageLine(t *testing.T) {
	if _, err := parseCoverage("ok  example.test/pkg  0.1s\n"); err == nil {
		t.Fatal("expected missing coverage line error")
	}
}
