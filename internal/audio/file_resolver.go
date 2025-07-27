package audio

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// FileResolver handles resolution of sound files with multiple extension support
type FileResolver struct {
	supportedExtensions []string
}

// NewFileResolver creates a new FileResolver with the specified supported extensions
func NewFileResolver(extensions []string) *FileResolver {
	slog.Debug("creating file resolver", 
		"extensions", extensions,
		"extension_count", len(extensions))
	
	return &FileResolver{
		supportedExtensions: extensions,
	}
}

// ResolveWithExtensions attempts to find an existing file by trying each supported extension
// in priority order. Returns the path to the first existing file found.
func (f *FileResolver) ResolveWithExtensions(basePath string) (string, error) {
	if basePath == "" {
		err := fmt.Errorf("base path cannot be empty")
		slog.Error("file resolution failed", "error", err)
		return "", err
	}

	slog.Debug("resolving file with extensions", 
		"base_path", basePath,
		"extensions", f.supportedExtensions,
		"extension_count", len(f.supportedExtensions))

	// Try each extension in priority order
	for i, ext := range f.supportedExtensions {
		// Ensure extension starts with dot
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		
		candidate := basePath + ext
		
		slog.Debug("checking file candidate", 
			"index", i,
			"extension", ext,
			"candidate", candidate)
		
		if _, err := os.Stat(candidate); err == nil {
			slog.Info("file resolved successfully",
				"base_path", basePath,
				"resolved_path", candidate,
				"extension", ext,
				"extension_index", i)
			
			return candidate, nil
		} else {
			slog.Debug("candidate not found", 
				"candidate", candidate, 
				"error", err)
		}
	}

	// No files found with any extension
	err := fmt.Errorf("no file found for base path %s with extensions %v", 
		basePath, f.supportedExtensions)
	
	slog.Warn("file resolution failed", 
		"base_path", basePath,
		"extensions_tried", f.supportedExtensions,
		"error", err)

	return "", err
}

// GetSupportedExtensions returns the list of supported extensions in priority order
func (f *FileResolver) GetSupportedExtensions() []string {
	return f.supportedExtensions
}