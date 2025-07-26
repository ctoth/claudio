package sounds

import (
	"log/slog"
	"strings"

	"claudio/internal/hooks"
)

// SoundMapper maps hook events to sound file paths using a 5-level fallback system
type SoundMapper struct{}

// SoundMappingResult contains the mapping result and metadata
type SoundMappingResult struct {
	SelectedPath  string   // The first path in the fallback chain (to be used)
	FallbackLevel int      // Which level was selected (1-5, 1-based)
	TotalPaths    int      // Total number of paths generated
	AllPaths      []string // All paths in fallback order
}

// NewSoundMapper creates a new sound mapper
func NewSoundMapper() *SoundMapper {
	slog.Debug("creating new sound mapper")
	return &SoundMapper{}
}

// MapSound maps a hook event context to sound file paths using a 5-level fallback system:
// 1. Exact hint match: "category/hint.wav"
// 2. Tool-specific: "category/tool.wav" 
// 3. Operation-specific: "category/operation.wav"
// 4. Category-specific: "category/category.wav"
// 5. Default: "default.wav"
func (m *SoundMapper) MapSound(context *hooks.EventContext) *SoundMappingResult {
	if context == nil {
		slog.Warn("nil context provided to sound mapper")
		return &SoundMappingResult{
			SelectedPath:  "default.wav",
			FallbackLevel: 5,
			TotalPaths:    1,
			AllPaths:      []string{"default.wav"},
		}
	}

	slog.Debug("mapping sound for hook event",
		"category", context.Category.String(),
		"tool_name", context.ToolName,
		"sound_hint", context.SoundHint,
		"operation", context.Operation,
		"file_type", context.FileType,
		"is_success", context.IsSuccess,
		"has_error", context.HasError)

	var paths []string
	categoryStr := context.Category.String()

	// Level 1: Exact hint match
	if context.SoundHint != "" {
		hintPath := categoryStr + "/" + normalizeName(context.SoundHint) + ".wav"
		paths = append(paths, hintPath)
		slog.Debug("added level 1 path (exact hint)", "path", hintPath)
	}

	// Level 2: Tool-specific (only if tool name exists)
	if context.ToolName != "" {
		toolPath := categoryStr + "/" + normalizeName(context.ToolName) + ".wav"
		paths = append(paths, toolPath)
		slog.Debug("added level 2 path (tool-specific)", "path", toolPath)
	}

	// Level 3: Operation-specific (only if operation exists)
	if context.Operation != "" {
		opPath := categoryStr + "/" + normalizeName(context.Operation) + ".wav"
		paths = append(paths, opPath)
		slog.Debug("added level 3 path (operation-specific)", "path", opPath)
	}

	// Level 4: Category-specific
	if categoryStr != "" && categoryStr != "unknown" {
		categoryPath := categoryStr + "/" + categoryStr + ".wav"
		paths = append(paths, categoryPath)
		slog.Debug("added level 4 path (category-specific)", "path", categoryPath)
	}

	// Level 5: Default (always present)
	paths = append(paths, "default.wav")
	slog.Debug("added level 5 path (default)", "path", "default.wav")

	if len(paths) == 0 {
		// Fallback if somehow no paths were generated
		slog.Warn("no paths generated, using default fallback")
		paths = []string{"default.wav"}
	}

	result := &SoundMappingResult{
		SelectedPath:  paths[0],
		FallbackLevel: 1, // Will be the first level that was actually added
		TotalPaths:    len(paths),
		AllPaths:      paths,
	}

	// Calculate the actual fallback level based on what was included
	if context.SoundHint != "" {
		result.FallbackLevel = 1
	} else if context.ToolName != "" {
		result.FallbackLevel = 2
	} else if context.Operation != "" {
		result.FallbackLevel = 3
	} else if categoryStr != "" && categoryStr != "unknown" {
		result.FallbackLevel = 4
	} else {
		result.FallbackLevel = 5
	}

	slog.Info("sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"all_paths", result.AllPaths,
		"context_category", context.Category.String(),
		"context_tool", context.ToolName,
		"context_hint", context.SoundHint)

	return result
}

// normalizeName converts a name to lowercase and replaces invalid characters
func normalizeName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase
	normalized := strings.ToLower(name)
	
	// Replace spaces and underscores with hyphens
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	
	// Replace any non-alphanumeric characters with hyphens to preserve word boundaries
	var result strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if r != '-' { // Don't replace existing hyphens
			result.WriteRune('-')
		} else {
			result.WriteRune(r) // Keep existing hyphens
		}
	}
	
	normalized = result.String()
	
	// Clean up multiple consecutive hyphens (but don't remove them entirely)
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}
	
	// Remove leading/trailing hyphens
	normalized = strings.Trim(normalized, "-")
	
	slog.Debug("normalized sound name", "original", name, "normalized", normalized)
	return normalized
}