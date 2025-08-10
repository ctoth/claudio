package sounds

import (
	"log/slog"
	"strings"

	"claudio.click/internal/hooks"
)

// SoundMapper maps hook events to sound file paths using event-specific fallback chains
type SoundMapper struct{}

// Chain type constants for sound mapping strategy
const (
	ChainTypeEnhanced = "enhanced" // PreToolUse: 9-level with command-only sounds
	ChainTypePostTool = "posttool" // PostToolUse: 6-level, skip command-only sounds
	ChainTypeSimple   = "simple"   // Simple events: 4-level event-specific fallback
	ChainTypeLegacy   = "legacy"   // Legacy fallback for compatibility
)

// SoundMappingResult contains the mapping result and metadata
type SoundMappingResult struct {
	SelectedPath  string   // The first path in the fallback chain (to be used)
	FallbackLevel int      // Which level was selected (1-6, 1-based)
	TotalPaths    int      // Total number of paths generated
	AllPaths      []string // All paths in fallback order
	ChainType     string   // Type of fallback chain used: "enhanced", "posttool", "simple"
}

// NewSoundMapper creates a new sound mapper
func NewSoundMapper() *SoundMapper {
	slog.Debug("creating new sound mapper")
	return &SoundMapper{}
}

// MapSound maps a hook event context to sound file paths using event-specific fallback chains:
// - PreToolUse: 9-level enhanced fallback with command-only sounds
// - PostToolUse: 6-level fallback (skip command-only sounds for semantic accuracy)
// - Simple events: 4-level fallback (UserPromptSubmit, Notification, Stop, SubagentStop, PreCompact)
func (m *SoundMapper) MapSound(context *hooks.EventContext) *SoundMappingResult {
	if context == nil {
		slog.Warn("nil context provided to sound mapper")
		return &SoundMappingResult{
			SelectedPath:  "default.wav",
			FallbackLevel: 6,
			TotalPaths:    1,
			AllPaths:      []string{"default.wav"},
			ChainType:     ChainTypeSimple,
		}
	}

	// Determine chain type based on event context
	chainType := m.determineChainType(context)
	slog.Debug("determined fallback chain type", "chain_type", chainType, "category", context.Category.String())

	// Route to appropriate mapping method based on chain type
	switch chainType {
	case ChainTypeEnhanced:
		return m.mapEnhancedSound(context)
	case ChainTypePostTool:
		return m.mapPostToolSound(context)
	case ChainTypeSimple:
		return m.mapSimpleSound(context)
	default:
		slog.Warn("unknown chain type, falling back to legacy mapper", "chain_type", chainType)
		return m.mapLegacySound(context)
	}
}

// determineChainType analyzes event context to determine which fallback chain type to use
func (m *SoundMapper) determineChainType(context *hooks.EventContext) string {
	// Strategy pattern: determine chain type based on event characteristics
	if m.isEnhancedChainEvent(context) {
		return ChainTypeEnhanced
	}

	if m.isPostToolChainEvent(context) {
		return ChainTypePostTool
	}

	// Default to simple chain for all other events
	return ChainTypeSimple
}

// isEnhancedChainEvent determines if event should use enhanced 9-level fallback
func (m *SoundMapper) isEnhancedChainEvent(context *hooks.EventContext) bool {
	// PreToolUse events with tool names use enhanced fallback including command-only sounds
	return context.Category == hooks.Loading && context.ToolName != ""
}

// isPostToolChainEvent determines if event should use PostToolUse 6-level fallback
func (m *SoundMapper) isPostToolChainEvent(context *hooks.EventContext) bool {
	// PostToolUse events with tool names use 6-level fallback (skip command-only sounds for semantic accuracy)
	return (context.Category == hooks.Success || context.Category == hooks.Error) && context.ToolName != ""
}

// mapEnhancedSound handles PreToolUse events with 9-level enhanced fallback chain
func (m *SoundMapper) mapEnhancedSound(context *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using enhanced 9-level fallback for PreToolUse",
		"category", context.Category.String(),
		"tool_name", context.ToolName,
		"original_tool", context.OriginalTool,
		"sound_hint", context.SoundHint,
		"operation", context.Operation)

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	paths := make([]string, 0, 9)
	categoryStr := context.Category.String()

	// Extract command and subcommand once for reuse
	command, subcommand := m.extractCommandFromHint(context.SoundHint, context.ToolName)
	suffix := m.extractSuffixFromOperation(context.Operation)

	// Level 1: Exact hint match
	if context.SoundHint != "" {
		hintPath := categoryStr + "/" + normalizeName(context.SoundHint) + ".wav"
		paths = append(paths, hintPath)
		slog.Debug("added level 1 path (exact hint)", "path", hintPath)
	}

	// Level 2: Command-subcommand without suffix (e.g., "git-commit.wav")
	if command != "" && subcommand != "" {
		cmdSubPath := categoryStr + "/" + normalizeName(command) + "-" + normalizeName(subcommand) + ".wav"
		paths = append(paths, cmdSubPath)
		slog.Debug("added level 2 path (command-subcommand)", "path", cmdSubPath)
	}

	// Level 3: Command with suffix (e.g., "git-start.wav")
	if command != "" && suffix != "" {
		cmdSuffixPath := categoryStr + "/" + normalizeName(command) + "-" + suffix + ".wav"
		paths = append(paths, cmdSuffixPath)
		slog.Debug("added level 3 path (command with suffix)", "path", cmdSuffixPath)
	}

	// Level 4: Command-only (e.g., "git.wav") - included for PreToolUse semantic appropriateness
	if command != "" {
		commandPath := categoryStr + "/" + normalizeName(command) + ".wav"
		paths = append(paths, commandPath)
		slog.Debug("added level 4 path (command-only)", "path", commandPath)
	}

	// Level 5: Original tool with suffix (e.g., "bash-start.wav")
	if context.OriginalTool != "" && suffix != "" {
		origSuffixPath := categoryStr + "/" + normalizeName(context.OriginalTool) + "-" + suffix + ".wav"
		paths = append(paths, origSuffixPath)
		slog.Debug("added level 5 path (original tool with suffix)", "path", origSuffixPath)
	}

	// Level 6: Original tool fallback (e.g., "bash.wav") - avoid duplicates
	if context.OriginalTool != "" && context.OriginalTool != command {
		originalPath := categoryStr + "/" + normalizeName(context.OriginalTool) + ".wav"
		paths = append(paths, originalPath)
		slog.Debug("added level 6 path (original tool)", "path", originalPath)
	}

	// Level 7: Operation-specific (e.g., "tool-start.wav")
	if context.Operation != "" {
		opPath := categoryStr + "/" + normalizeName(context.Operation) + ".wav"
		paths = append(paths, opPath)
		slog.Debug("added level 7 path (operation-specific)", "path", opPath)
	}

	// Level 8: Category-specific (e.g., "loading.wav")
	if categoryStr != "" && categoryStr != "unknown" {
		categoryPath := categoryStr + "/" + categoryStr + ".wav"
		paths = append(paths, categoryPath)
		slog.Debug("added level 8 path (category-specific)", "path", categoryPath)
	}

	// Level 9: Default fallback
	paths = append(paths, "default.wav")
	slog.Debug("added level 9 path (default)", "path", "default.wav")

	// Ensure we have at least the default path
	if len(paths) == 0 {
		slog.Warn("no paths generated in enhanced fallback, using default")
		paths = []string{"default.wav"}
	}

	result := &SoundMappingResult{
		SelectedPath:  paths[0],
		FallbackLevel: m.calculateFallbackLevel(context, paths),
		TotalPaths:    len(paths),
		AllPaths:      paths,
		ChainType:     ChainTypeEnhanced,
	}

	slog.Info("enhanced sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"chain_type", result.ChainType,
		"all_paths", result.AllPaths)

	return result
}

// buildPath creates a standardized sound path with proper normalization
func (m *SoundMapper) buildPath(category, name string) string {
	if category == "" {
		return "default.wav"
	}
	if name == "" {
		return category + "/" + category + ".wav"
	}
	return category + "/" + normalizeName(name) + ".wav"
}

// getEventSpecificPath maps operations to event-specific sound paths for simple events
func (m *SoundMapper) getEventSpecificPath(category, operation string) string {
	if category == "" || operation == "" {
		return ""
	}

	// Map operation to event-specific sound name
	var eventName string
	switch operation {
	case "prompt":
		eventName = "prompt-submit"
	case "notification":
		eventName = "notification"
	case "stop":
		eventName = "stop"
	case "subagent-stop":
		eventName = "subagent-stop"
	case "compact":
		eventName = "pre-compact"
	default:
		// For unknown operations, use the operation name directly
		eventName = operation
	}

	return m.buildPath(category, eventName)
}

// determineCategorySuffix determines the appropriate suffix based on event category and context
func (m *SoundMapper) determineCategorySuffix(category hooks.EventCategory, operation string) string {
	switch category {
	case hooks.Success:
		return "success"
	case hooks.Error:
		return "error"
	case hooks.Loading:
		return "start"
	case hooks.Interactive:
		return "submit"
	case hooks.Completion:
		return "complete"
	case hooks.System:
		return "" // System events typically don't have suffixes
	default:
		// Fallback to operation-based suffix
		return m.extractSuffixFromOperation(operation)
	}
}

// extractCommandFromHint extracts command and subcommand from sound hint and tool name
func (m *SoundMapper) extractCommandFromHint(hint, toolName string) (command, subcommand string) {
	if hint == "" || toolName == "" {
		return toolName, ""
	}

	// Parse hint like "git-commit-start" to extract "git" and "commit"
	parts := strings.Split(hint, "-")
	if len(parts) >= 2 {
		// First part should match the tool name
		if strings.EqualFold(parts[0], toolName) {
			command = parts[0]
			// Check if second part is a known suffix, if not it's likely a subcommand
			suffixes := []string{"start", "thinking", "success", "error", "complete"}
			secondPart := parts[1]

			// If second part is not a suffix, it's a subcommand
			isSuffix := false
			for _, suffix := range suffixes {
				if strings.EqualFold(secondPart, suffix) {
					isSuffix = true
					break
				}
			}

			if !isSuffix && len(parts) >= 3 {
				// Pattern: git-commit-start -> command="git", subcommand="commit"
				subcommand = secondPart
			}
		}
	}

	// Fallback: use toolName as command if extraction failed
	if command == "" {
		command = toolName
	}

	slog.Debug("extracted command from hint",
		"hint", hint,
		"tool_name", toolName,
		"command", command,
		"subcommand", subcommand)

	return command, subcommand
}

// extractSuffixFromOperation extracts the appropriate suffix from operation context
func (m *SoundMapper) extractSuffixFromOperation(operation string) string {
	if operation == "" {
		return ""
	}

	// Map operation to appropriate suffix
	switch operation {
	case "tool-start":
		return "start"
	case "tool-complete":
		return "complete"
	case "prompt":
		return "submit"
	case "notification":
		return "" // No suffix for notifications
	case "stop":
		return "complete"
	case "subagent-stop":
		return "complete"
	case "compact":
		return "" // No suffix for compact operations
	default:
		// For unknown operations, try to extract meaningful suffix
		if strings.HasSuffix(operation, "-start") {
			return "start"
		}
		if strings.HasSuffix(operation, "-complete") {
			return "complete"
		}
		return operation // Use as-is if no known pattern
	}
}

// calculateFallbackLevel determines which level in the fallback chain would be selected
func (m *SoundMapper) calculateFallbackLevel(context *hooks.EventContext, paths []string) int {
	// For now, always return 1 since we always put the most specific path first
	// In a real implementation, this would check file existence and return the level of the first existing file
	return 1
}

// mapPostToolSound handles PostToolUse events with 6-level fallback (skip command-only sounds)
func (m *SoundMapper) mapPostToolSound(context *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using PostToolUse 6-level fallback (skip command-only)",
		"category", context.Category.String(),
		"tool_name", context.ToolName,
		"original_tool", context.OriginalTool,
		"sound_hint", context.SoundHint,
		"operation", context.Operation,
		"is_success", context.IsSuccess,
		"has_error", context.HasError)

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	paths := make([]string, 0, 6)
	categoryStr := context.Category.String()

	// Extract command once for reuse (skip subcommand since we don't use command-subcommand level)
	command, _ := m.extractCommandFromHint(context.SoundHint, context.ToolName)

	// Determine suffix based on category (success/error context)
	suffix := m.determineCategorySuffix(context.Category, context.Operation)

	// Level 1: Exact hint match
	if context.SoundHint != "" {
		hintPath := m.buildPath(categoryStr, context.SoundHint)
		paths = append(paths, hintPath)
		slog.Debug("added level 1 path (exact hint)", "path", hintPath)
	}

	// Level 2: Command with suffix (e.g., "git-success.wav") - skip command-only for semantic accuracy
	if command != "" && suffix != "" {
		cmdSuffixPath := m.buildPath(categoryStr, command+"-"+suffix)
		paths = append(paths, cmdSuffixPath)
		slog.Debug("added level 2 path (command with suffix, skip command-only)", "path", cmdSuffixPath)
	}

	// Level 3: Original tool with suffix (e.g., "bash-success.wav")
	if context.OriginalTool != "" && suffix != "" {
		origSuffixPath := m.buildPath(categoryStr, context.OriginalTool+"-"+suffix)
		paths = append(paths, origSuffixPath)
		slog.Debug("added level 3 path (original tool with suffix)", "path", origSuffixPath)
	}

	// Level 4: Operation-specific (e.g., "tool-complete.wav")
	if context.Operation != "" {
		opPath := m.buildPath(categoryStr, context.Operation)
		paths = append(paths, opPath)
		slog.Debug("added level 4 path (operation-specific)", "path", opPath)
	}

	// Level 5: Category-specific (e.g., "success.wav", "error.wav")
	if categoryStr != "" && categoryStr != "unknown" {
		categoryPath := m.buildPath(categoryStr, "")
		paths = append(paths, categoryPath)
		slog.Debug("added level 5 path (category-specific)", "path", categoryPath)
	}

	// Level 6: Default fallback
	paths = append(paths, "default.wav")
	slog.Debug("added level 6 path (default)", "path", "default.wav")

	// Ensure we have at least the default path
	if len(paths) == 0 {
		slog.Warn("no paths generated in PostToolUse fallback, using default")
		paths = []string{"default.wav"}
	}

	result := &SoundMappingResult{
		SelectedPath:  paths[0],
		FallbackLevel: m.calculateFallbackLevel(context, paths),
		TotalPaths:    len(paths),
		AllPaths:      paths,
		ChainType:     ChainTypePostTool,
	}

	slog.Info("PostToolUse sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"chain_type", result.ChainType,
		"all_paths", result.AllPaths)

	return result
}

// mapSimpleSound handles simple events with 4-level fallback chain
func (m *SoundMapper) mapSimpleSound(context *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using simple 4-level fallback for simple events",
		"category", context.Category.String(),
		"sound_hint", context.SoundHint,
		"operation", context.Operation)

	// Pre-allocate slice with exact capacity for 4-level fallback
	paths := make([]string, 0, 4)
	categoryStr := context.Category.String()

	// Level 1: Specific hint match
	if context.SoundHint != "" {
		hintPath := m.buildPath(categoryStr, context.SoundHint)
		paths = append(paths, hintPath)
		slog.Debug("added level 1 path (specific hint)", "path", hintPath)
	}

	// Level 2: Event-specific based on operation (not tool-based)
	if context.Operation != "" {
		eventPath := m.getEventSpecificPath(categoryStr, context.Operation)
		if eventPath != "" {
			paths = append(paths, eventPath)
			slog.Debug("added level 2 path (event-specific)", "path", eventPath)
		}
	}

	// Level 3: Category-specific
	if categoryStr != "" && categoryStr != "unknown" {
		categoryPath := m.buildPath(categoryStr, "")
		paths = append(paths, categoryPath)
		slog.Debug("added level 3 path (category-specific)", "path", categoryPath)
	}

	// Level 4: Default fallback
	paths = append(paths, "default.wav")
	slog.Debug("added level 4 path (default)", "path", "default.wav")

	// Ensure we have at least the default path
	if len(paths) == 0 {
		slog.Warn("no paths generated in simple fallback, using default")
		paths = []string{"default.wav"}
	}

	result := &SoundMappingResult{
		SelectedPath:  paths[0],
		FallbackLevel: m.calculateFallbackLevel(context, paths),
		TotalPaths:    len(paths),
		AllPaths:      paths,
		ChainType:     ChainTypeSimple,
	}

	slog.Info("simple sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"chain_type", result.ChainType,
		"all_paths", result.AllPaths)

	return result
}

// mapLegacySound implements the original 6-level fallback system for backwards compatibility
func (m *SoundMapper) mapLegacySound(context *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using legacy 6-level fallback",
		"category", context.Category.String(),
		"tool_name", context.ToolName,
		"original_tool", context.OriginalTool,
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

	// Level 2: Tool-specific (extracted tool)
	if context.ToolName != "" {
		toolPath := categoryStr + "/" + normalizeName(context.ToolName) + ".wav"
		paths = append(paths, toolPath)
		slog.Debug("added level 2 path (tool-specific)", "path", toolPath)
	}

	// Level 3: Original tool fallback (if different from current tool)
	if context.OriginalTool != "" && context.OriginalTool != context.ToolName {
		originalPath := categoryStr + "/" + normalizeName(context.OriginalTool) + ".wav"
		paths = append(paths, originalPath)
		slog.Debug("added level 3 path (original tool fallback)", "path", originalPath)
	}

	// Level 4: Operation-specific
	if context.Operation != "" {
		opPath := categoryStr + "/" + normalizeName(context.Operation) + ".wav"
		paths = append(paths, opPath)
		slog.Debug("added level 4 path (operation-specific)", "path", opPath)
	}

	// Level 5: Category-specific
	if categoryStr != "" && categoryStr != "unknown" {
		categoryPath := categoryStr + "/" + categoryStr + ".wav"
		paths = append(paths, categoryPath)
		slog.Debug("added level 5 path (category-specific)", "path", categoryPath)
	}

	// Level 6: Default
	paths = append(paths, "default.wav")
	slog.Debug("added level 6 path (default)", "path", "default.wav")

	if len(paths) == 0 {
		slog.Warn("no paths generated, using default fallback")
		paths = []string{"default.wav"}
	}

	// Determine chain type for result metadata using same logic as main mapper
	chainType := ChainTypeLegacy
	if m.isEnhancedChainEvent(context) {
		chainType = ChainTypeEnhanced // Will be properly implemented in Phase 2.2
	} else if m.isPostToolChainEvent(context) {
		chainType = ChainTypePostTool // Will be properly implemented in Phase 2.3
	} else {
		chainType = ChainTypeSimple // Will be properly implemented in Phase 2.4
	}

	result := &SoundMappingResult{
		SelectedPath:  paths[0],
		FallbackLevel: 1,
		TotalPaths:    len(paths),
		AllPaths:      paths,
		ChainType:     chainType,
	}

	// Calculate actual fallback level
	if context.SoundHint != "" {
		result.FallbackLevel = 1
	} else if context.ToolName != "" {
		result.FallbackLevel = 2
	} else if context.OriginalTool != "" && context.OriginalTool != context.ToolName {
		result.FallbackLevel = 3
	} else if context.Operation != "" {
		result.FallbackLevel = 4
	} else if categoryStr != "" && categoryStr != "unknown" {
		result.FallbackLevel = 5
	} else {
		result.FallbackLevel = 6
	}

	slog.Info("legacy sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"chain_type", result.ChainType,
		"all_paths", result.AllPaths,
		"context_category", context.Category.String(),
		"context_tool", context.ToolName,
		"context_original_tool", context.OriginalTool,
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
