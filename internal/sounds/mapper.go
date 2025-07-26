package sounds

import (
	"log/slog"
	"strings"

	"claudio/internal/hooks"
)

// SoundMapper maps hook events to sound file paths using a 6-level fallback system
type SoundMapper struct{}

// NewSoundMapper creates a new sound mapper
func NewSoundMapper() *SoundMapper {
	slog.Debug("creating new sound mapper")
	return &SoundMapper{}
}

// GetSoundPaths returns a prioritized list of sound file paths for the given context
// using a 6-level fallback system:
// 1. Exact hint match: "category/hint.wav"
// 2. Tool-specific: "category/tool.wav" 
// 3. Operation-specific: "category/operation.wav"
// 4. Category-specific: "category/category.wav"
// 5. Default: "default.wav"
// 6. Silent: ""
func (m *SoundMapper) GetSoundPaths(context *hooks.EventContext) []string {
	if context == nil {
		slog.Warn("nil context provided to sound mapper")
		return []string{"default.wav", ""}
	}

	slog.Debug("mapping sound paths",
		"category", context.Category.String(),
		"tool_name", context.ToolName,
		"sound_hint", context.SoundHint,
		"operation", context.Operation,
		"file_type", context.FileType)

	var paths []string
	categoryStr := context.Category.String()

	// Level 1: Exact hint match
	if context.SoundHint != "" {
		hintPath := categoryStr + "/" + normalizeName(context.SoundHint) + ".wav"
		paths = append(paths, hintPath)
		slog.Debug("added hint path", "path", hintPath)
	}

	// Level 2: Tool-specific (only if tool name exists)
	if context.ToolName != "" {
		toolPath := categoryStr + "/" + normalizeName(context.ToolName) + ".wav"
		paths = append(paths, toolPath)
		slog.Debug("added tool path", "path", toolPath)
	}

	// Level 3: Operation-specific (only if operation exists)
	if context.Operation != "" {
		opPath := categoryStr + "/" + normalizeName(context.Operation) + ".wav"
		paths = append(paths, opPath)
		slog.Debug("added operation path", "path", opPath)
	}

	// Level 4: Category-specific
	categoryPath := categoryStr + "/" + categoryStr + ".wav"
	paths = append(paths, categoryPath)
	slog.Debug("added category path", "path", categoryPath)

	// Level 5: Default
	paths = append(paths, "default.wav")
	slog.Debug("added default path")

	// Level 6: Silent
	paths = append(paths, "")
	slog.Debug("added silent fallback")

	slog.Info("sound paths generated",
		"total_paths", len(paths),
		"primary_path", paths[0],
		"context_category", context.Category.String())

	return paths
}

// normalizeName converts a name to lowercase and replaces invalid characters
func normalizeName(name string) string {
	// Convert to lowercase
	normalized := strings.ToLower(name)
	
	// Replace spaces and other characters with hyphens
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	
	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}