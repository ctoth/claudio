package sounds

import (
	"context"
	"log/slog"
	"strings"

	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
)

// SoundMapper maps hook events to sound file paths using event-specific
// fallback chains. The mapper owns the chain construction; the resolver
// (passed in at construction) owns the per-candidate existence check.
//
// The optional PathObserver is the inversion seam for tracking. The mapper
// itself knows nothing about tracking — observation data flows to whatever
// closure the caller wires in via WithObserver. The CLI composes a
// tracking.LookupBuffer.Observer() and harvests its Lookups() for
// RecordEvent after MapSound returns.
type SoundMapper struct {
	resolver soundpack.SoundpackResolver // resolver for path existence checks (may be nil)
	observer soundpack.PathObserver      // optional per-candidate observer
}

// MapperOption configures a SoundMapper at construction.
type MapperOption func(*SoundMapper)

// WithObserver injects a soundpack.PathObserver that fires once per
// candidate path the resolver walks during MapSound. A nil observer means
// "no observation". Use with tracking.LookupBuffer to record the resolved
// chain for later RecordEvent persistence.
func WithObserver(obs soundpack.PathObserver) MapperOption {
	return func(m *SoundMapper) {
		m.observer = obs
	}
}

// Chain type constants for sound mapping strategy
const (
	ChainTypeEnhanced = "enhanced" // PreToolUse: 9-level with command-only sounds
	ChainTypePostTool = "posttool" // PostToolUse: 6-level, skip command-only sounds
	ChainTypeSimple   = "simple"   // Simple events: 4-level event-specific fallback
)

// SoundMappingResult contains the mapping result and metadata
type SoundMappingResult struct {
	SelectedPath  string   // The first path in the fallback chain (to be used)
	FallbackLevel int      // Which level was selected (1-6, 1-based)
	TotalPaths    int      // Total number of paths generated
	AllPaths      []string // All paths in fallback order
	ChainType     string   // Type of fallback chain used: "enhanced", "posttool", "simple"
}

// NewSoundMapper creates a new sound mapper with no resolver (path-existence
// checks return false; FallbackLevel defaults to 1). Retained for tests and
// for the rare caller that wants chain-construction without resolution.
func NewSoundMapper() *SoundMapper {
	slog.Debug("creating new sound mapper without resolver")
	return &SoundMapper{}
}

// NewSoundMapperWithResolver creates a new sound mapper that uses the given
// soundpack.SoundpackResolver for per-candidate existence checks. Optional
// MapperOptions (e.g. WithObserver) configure observation.
func NewSoundMapperWithResolver(resolver soundpack.SoundpackResolver, opts ...MapperOption) *SoundMapper {
	slog.Debug("creating new sound mapper with resolver")
	m := &SoundMapper{
		resolver: resolver,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// MapSound maps a hook event context to sound file paths using event-specific fallback chains:
// - PreToolUse: 9-level enhanced fallback with command-only sounds
// - PostToolUse: 6-level fallback (skip command-only sounds for semantic accuracy)
// - Simple events: 4-level fallback (UserPromptSubmit, Notification, Stop, SubagentStop, PreCompact)
//
// ctx is threaded into resolver operations. A nil ctx is treated as
// context.Background() so existing call sites that don't have a context
// handy still work.
func (m *SoundMapper) MapSound(ctx context.Context, eventCtx *hooks.EventContext) *SoundMappingResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if eventCtx == nil {
		slog.Warn("nil context provided to sound mapper")
		return &SoundMappingResult{
			SelectedPath:  "default.wav",
			FallbackLevel: 6,
			TotalPaths:    1,
			AllPaths:      []string{"default.wav"},
			ChainType:     ChainTypeSimple,
		}
	}
	if eventCtx.Category == hooks.Silent {
		slog.Debug("silent context provided to sound mapper")
		return nil
	}

	// Determine chain type based on event context
	chainType := m.determineChainType(eventCtx)
	slog.Debug("determined fallback chain type", "chain_type", chainType, "category", eventCtx.Category.String())

	// Route to appropriate mapping method based on chain type
	switch chainType {
	case ChainTypeEnhanced:
		return m.mapEnhancedSound(ctx, eventCtx)
	case ChainTypePostTool:
		return m.mapPostToolSound(ctx, eventCtx)
	case ChainTypeSimple:
		return m.mapSimpleSound(ctx, eventCtx)
	}
	// Unreachable: determineChainType returns only Enhanced, PostTool, or Simple.
	return m.mapSimpleSound(ctx, eventCtx)
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
func (m *SoundMapper) mapEnhancedSound(ctx context.Context, eventCtx *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using enhanced 9-level fallback for PreToolUse",
		"category", eventCtx.Category.String(),
		"tool_name", eventCtx.ToolName,
		"original_tool", eventCtx.OriginalTool,
		"sound_hint", eventCtx.SoundHint,
		"operation", eventCtx.Operation)

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	paths := make([]string, 0, 9)
	categoryStr := eventCtx.Category.String()

	// Extract command and subcommand once for reuse
	command, subcommand := m.extractCommandFromHint(eventCtx.SoundHint, eventCtx.ToolName)
	suffix := m.extractSuffixFromOperation(eventCtx.Operation)

	// Level 1: Exact hint match
	if eventCtx.SoundHint != "" {
		hintPath := categoryStr + "/" + normalizeName(eventCtx.SoundHint) + ".wav"
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
	if eventCtx.OriginalTool != "" && suffix != "" {
		origSuffixPath := categoryStr + "/" + normalizeName(eventCtx.OriginalTool) + "-" + suffix + ".wav"
		paths = append(paths, origSuffixPath)
		slog.Debug("added level 5 path (original tool with suffix)", "path", origSuffixPath)
	}

	// Level 6: Original tool fallback (e.g., "bash.wav") - avoid duplicates
	if eventCtx.OriginalTool != "" && eventCtx.OriginalTool != command {
		originalPath := categoryStr + "/" + normalizeName(eventCtx.OriginalTool) + ".wav"
		paths = append(paths, originalPath)
		slog.Debug("added level 6 path (original tool)", "path", originalPath)
	}

	// Level 7: Operation-specific (e.g., "tool-start.wav")
	if eventCtx.Operation != "" {
		opPath := categoryStr + "/" + normalizeName(eventCtx.Operation) + ".wav"
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

	return m.finalizeResult(ctx, paths, ChainTypeEnhanced)
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

// dedupPreserveOrder collapses duplicate paths keeping first occurrence.
// Chains can emit duplicate paths (e.g. PostTool's hint and command-suffix
// levels collapse when SoundHint == ToolName+"-"+suffix). Dedup keeps the
// "first existing wins" intent and matches the UNIQUE(event_id, path)
// constraint downstream telemetry relies on.
func dedupPreserveOrder(paths []string) []string {
	if len(paths) <= 1 {
		return paths
	}
	seen := make(map[string]struct{}, len(paths))
	deduped := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		deduped = append(deduped, p)
	}
	return deduped
}

// finalizeResult dedups the chain, resolves the winning candidate via
// soundpack.ResolveSoundWithFallback (wiring the observer through), and
// returns the SoundMappingResult with FallbackLevel set to the 1-based
// winner index (or len(paths) when no candidate existed).
//
// If the mapper has no resolver, returns level 1 unchecked — callers without
// a resolver get the first path back as the selection. No observation fires
// in that path (no candidate was actually inspected).
func (m *SoundMapper) finalizeResult(ctx context.Context, paths []string, chainType string) *SoundMappingResult {
	_ = ctx // reserved for future cancellable resolution
	paths = dedupPreserveOrder(paths)

	fallbackLevel := 1
	selectedPath := paths[0]

	if m.resolver != nil {
		// Record per-candidate hits via a local tracker so the mapper can
		// compute FallbackLevel from the observer stream — the same source
		// of truth telemetry uses. Wrap the caller's observer if any.
		caller := m.observer
		winnerIdx := -1
		tracker := func(path string, sequence int, exists bool) {
			if exists && winnerIdx == -1 {
				winnerIdx = sequence
			}
			if caller != nil {
				caller(path, sequence, exists)
			}
		}

		resolved, err := m.resolver.ResolveSoundWithFallback(
			paths,
			soundpack.WithObserver(tracker),
		)
		if err == nil {
			// Prefer the observer's logical chain index when present, so the
			// SelectedPath we surface to the caller and the chain index used
			// for tracking always agree. Fall back to the resolver's physical
			// path only when no observer winner was captured. Chunk 14 F3:
			// the previous shape unconditionally assigned `selectedPath =
			// resolved` and then overwrote it whenever winnerIdx > 0 — dead
			// store in the common branch.
			if winnerIdx > 0 {
				fallbackLevel = winnerIdx
				selectedPath = paths[winnerIdx-1]
			} else {
				selectedPath = resolved
			}
		} else {
			// Nothing existed; fall back to the last chain entry (default).
			fallbackLevel = len(paths)
			selectedPath = paths[len(paths)-1]
		}
	}

	result := &SoundMappingResult{
		SelectedPath:  selectedPath,
		FallbackLevel: fallbackLevel,
		TotalPaths:    len(paths),
		AllPaths:      paths,
		ChainType:     chainType,
	}

	slog.Debug("sound mapping completed",
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"chain_type", result.ChainType,
		"all_paths", result.AllPaths)

	return result
}

// mapPostToolSound handles PostToolUse events with 6-level fallback (skip command-only sounds)
func (m *SoundMapper) mapPostToolSound(ctx context.Context, eventCtx *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using PostToolUse 6-level fallback (skip command-only)",
		"category", eventCtx.Category.String(),
		"tool_name", eventCtx.ToolName,
		"original_tool", eventCtx.OriginalTool,
		"sound_hint", eventCtx.SoundHint,
		"operation", eventCtx.Operation,
		"is_success", eventCtx.IsSuccess,
		"has_error", eventCtx.HasError)

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	paths := make([]string, 0, 6)
	categoryStr := eventCtx.Category.String()

	// Extract command once for reuse (skip subcommand since we don't use command-subcommand level)
	command, _ := m.extractCommandFromHint(eventCtx.SoundHint, eventCtx.ToolName)

	// Determine suffix based on category (success/error context)
	suffix := m.determineCategorySuffix(eventCtx.Category, eventCtx.Operation)

	// Level 1: Exact hint match
	if eventCtx.SoundHint != "" {
		hintPath := m.buildPath(categoryStr, eventCtx.SoundHint)
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
	if eventCtx.OriginalTool != "" && suffix != "" {
		origSuffixPath := m.buildPath(categoryStr, eventCtx.OriginalTool+"-"+suffix)
		paths = append(paths, origSuffixPath)
		slog.Debug("added level 3 path (original tool with suffix)", "path", origSuffixPath)
	}

	// Level 4: Operation-specific (e.g., "tool-complete.wav")
	if eventCtx.Operation != "" {
		opPath := m.buildPath(categoryStr, eventCtx.Operation)
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

	return m.finalizeResult(ctx, paths, ChainTypePostTool)
}

// mapSimpleSound handles simple events with 4-level fallback chain
func (m *SoundMapper) mapSimpleSound(ctx context.Context, eventCtx *hooks.EventContext) *SoundMappingResult {
	slog.Debug("mapping sound using simple 4-level fallback for simple events",
		"category", eventCtx.Category.String(),
		"sound_hint", eventCtx.SoundHint,
		"operation", eventCtx.Operation)

	// Pre-allocate slice with exact capacity for 4-level fallback
	paths := make([]string, 0, 4)
	categoryStr := eventCtx.Category.String()

	// Level 1: Specific hint match
	if eventCtx.SoundHint != "" {
		hintPath := m.buildPath(categoryStr, eventCtx.SoundHint)
		paths = append(paths, hintPath)
		slog.Debug("added level 1 path (specific hint)", "path", hintPath)
	}

	// Level 2: Event-specific based on operation (not tool-based)
	if eventCtx.Operation != "" {
		eventPath := m.getEventSpecificPath(categoryStr, eventCtx.Operation)
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

	return m.finalizeResult(ctx, paths, ChainTypeSimple)
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
