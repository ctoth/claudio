package fs

import (
	"testing"

	"github.com/spf13/afero"
)

func TestDefaultFactory(t *testing.T) {
	factory := NewDefaultFactory()
	
	if factory == nil {
		t.Fatal("Expected factory to be created")
	}
	
	// Test production filesystem
	prodFS := factory.Production()
	if prodFS == nil {
		t.Fatal("Expected production filesystem")
	}
	
	// Should be OsFs type
	if _, ok := prodFS.(*afero.OsFs); !ok {
		t.Error("Expected production filesystem to be *afero.OsFs")
	}
	
	// Test memory filesystem
	memFS := factory.Memory()
	if memFS == nil {
		t.Fatal("Expected memory filesystem")
	}
	
	// Should be MemMapFs type
	if _, ok := memFS.(*afero.MemMapFs); !ok {
		t.Error("Expected memory filesystem to be *afero.MemMapFs")
	}
}

func TestMemoryFilesystemIsolation(t *testing.T) {
	factory := NewDefaultFactory()
	memFS1 := factory.Memory()
	memFS2 := factory.Memory()
	
	// Write to first memory filesystem
	err := afero.WriteFile(memFS1, "/test1.txt", []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to write to memFS1: %v", err)
	}
	
	// Write to second memory filesystem
	err = afero.WriteFile(memFS2, "/test2.txt", []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to write to memFS2: %v", err)
	}
	
	// Verify files are isolated
	exists1, _ := afero.Exists(memFS1, "/test2.txt")
	if exists1 {
		t.Error("Expected file from memFS2 not to exist in memFS1 (isolation broken)")
	}
	
	exists2, _ := afero.Exists(memFS2, "/test1.txt")
	if exists2 {
		t.Error("Expected file from memFS1 not to exist in memFS2 (isolation broken)")
	}
	
	// Verify each filesystem has its own file
	exists1Own, _ := afero.Exists(memFS1, "/test1.txt")
	if !exists1Own {
		t.Error("Expected memFS1 to have its own file")
	}
	
	exists2Own, _ := afero.Exists(memFS2, "/test2.txt")
	if !exists2Own {
		t.Error("Expected memFS2 to have its own file")
	}
}

func TestExecutablePathFunction(t *testing.T) {
	path, err := ExecutablePath()
	if err != nil {
		t.Errorf("ExecutablePath failed: %v", err)
	}
	
	if path == "" {
		t.Error("Expected non-empty executable path")
	}
}

func TestMockExecutablePath(t *testing.T) {
	// Set up mock
	originalMock := MockExecutablePath
	defer func() { MockExecutablePath = originalMock }()
	
	MockExecutablePath = func() (string, error) {
		return "/mock/claudio", nil
	}
	
	// Test mock is used
	path, err := TestExecutablePath()
	if err != nil {
		t.Errorf("TestExecutablePath with mock failed: %v", err)
	}
	
	if path != "/mock/claudio" {
		t.Errorf("Expected mock path '/mock/claudio', got '%s'", path)
	}
	
	// Clear mock and test fallback
	MockExecutablePath = nil
	path2, err := TestExecutablePath()
	if err != nil {
		t.Errorf("TestExecutablePath without mock failed: %v", err)
	}
	
	if path2 == "/mock/claudio" {
		t.Error("Expected real path when mock is cleared, but got mock path")
	}
}