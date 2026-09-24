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
// closure the caller passes at construction. The CLI composes a
// tracking.LookupBuffer.Observer() and harvests its Lookups() for
// RecordEvent after MapSound returns.
type SoundMapper struct {
	resolver soundpack.SoundpackResolver // resolver for path existence checks (may be nil)
	observer soundpack.PathObserver      // optional per-candidate observer
}

// ChainType names the fallback chain a mapping used. The string values are
// recorded in the tracking database.
type ChainType string

const (
	ChainTypeEnhanced ChainType = "enhanced" // PreToolUse: 9-level with command-only sounds
	ChainTypePostTool ChainType = "posttool" // PostToolUse: 6-level, skip command-only sounds
	ChainTypeSimple   ChainType = "simple"   // Simple events: 4-level event-specific fallback
)

// SoundMappingResult contains the mapping result and metadata
type SoundMappingResult struct {
	SelectedPath  string    // The chosen candidate (first existing, else the last)
	FallbackLevel int       // 1-based index of SelectedPath in AllPaths
	TotalPaths    int       // Total number of paths generated
	AllPaths      []string  // All paths in fallback order
	ChainType     ChainType // Fallback chain used
}

// NewSoundMapper creates a new sound mapper with no resolver (path-existence
// checks return false; FallbackLevel defaults to 1). Retained for tests and
// for the rare caller that wants chain-construction without resolution.
func NewSoundMapper() *SoundMapper {
	return &SoundMapper{}
}

// NewSoundMapperWithResolver creates a new sound mapper that uses the given
// soundpack.SoundpackResolver for per-candidate existence checks. The
// observer fires once per candidate path the resolver walks during MapSound;
// a nil observer means "no observation". Use with tracking.LookupBuffer to
// record the resolved chain for later RecordEvent persistence.
func NewSoundMapperWithResolver(resolver soundpack.SoundpackResolver, observer soundpack.PathObserver) *SoundMapper {
	return &SoundMapper{
		resolver: resolver,
		observer: observer,
	}
}

// MapSound maps a hook event context to sound file paths using event-specific fallback chains:
// - PreToolUse: 9-level enhanced fallback with command-only sounds
// - PostToolUse: 6-level fallback (skip command-only sounds for semantic accuracy)
// - Simple events: 4-level fallback (UserPromptSubmit, Notification, Stop, SubagentStop, PreCompact)
//
// Levels whose inputs are empty are skipped, and duplicate candidates are
// collapsed. ctx is threaded into resolver operations; a nil ctx is treated
// as context.Background().
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

	chainType := chainTypeFor(eventCtx)
	in := newChainInput(eventCtx)
	var paths []string
	for _, level := range chains[chainType] {
		if p := level(in); p != "" {
			paths = append(paths, p)
		}
	}
	return m.finalizeResult(ctx, eventCtx, paths, chainType)
}

// chainTypeFor picks the chain: tool events use the enhanced (start) or
// posttool (success/error) chain, everything else the simple chain.
func chainTypeFor(eventCtx *hooks.EventContext) ChainType {
	if eventCtx.ToolName == "" {
		return ChainTypeSimple
	}
	switch eventCtx.Category {
	case hooks.Loading:
		return ChainTypeEnhanced
	case hooks.Success, hooks.Error:
		return ChainTypePostTool
	default:
		return ChainTypeSimple
	}
}

// chainInput is everything a chain level reads.
type chainInput struct {
	category     string
	hint         string
	command      string
	subcommand   string
	suffix       string
	originalTool string
	operation    string
}

func newChainInput(eventCtx *hooks.EventContext) chainInput {
	command, subcommand := commandOf(eventCtx)
	return chainInput{
		category:     eventCtx.Category.String(),
		hint:         eventCtx.SoundHint,
		command:      command,
		subcommand:   subcommand,
		suffix:       phaseOf(eventCtx),
		originalTool: eventCtx.OriginalTool,
		operation:    eventCtx.Operation,
	}
}

// chainLevel yields one candidate path, or "" to skip the level.
type chainLevel func(in chainInput) string

// chains lists each chain's levels in fallback order. The examples are for
// "git commit" run through Bash.
var chains = map[ChainType][]chainLevel{
	ChainTypeEnhanced: {
		hintLevel,               // loading/git-commit-start.wav
		commandSubcommandLevel,  // loading/git-commit.wav
		commandSuffixLevel,      // loading/git-start.wav
		commandLevel,            // loading/git.wav
		originalToolSuffixLevel, // loading/bash-start.wav
		originalToolLevel,       // loading/bash.wav
		operationLevel,          // loading/tool-start.wav
		categoryLevel,           // loading/loading.wav
		defaultLevel,            // default.wav
	},
	// PostTool skips the subcommand and command-only levels: a bare
	// "git.wav" is a start sound, not a result.
	ChainTypePostTool: {
		hintLevel,               // success/git-commit-success.wav
		commandSuffixLevel,      // success/git-success.wav
		originalToolSuffixLevel, // success/bash-success.wav
		operationLevel,          // success/tool-complete.wav
		categoryLevel,           // success/success.wav
		defaultLevel,            // default.wav
	},
	ChainTypeSimple: {
		hintLevel,     // completion/agent-complete.wav
		eventLevel,    // completion/stop.wav
		categoryLevel, // completion/completion.wav
		defaultLevel,  // default.wav
	},
}

func hintLevel(in chainInput) string {
	return candidate(in.category, in.hint)
}

func commandSubcommandLevel(in chainInput) string {
	return candidate(in.category, in.command, in.subcommand)
}

func commandSuffixLevel(in chainInput) string {
	return candidate(in.category, in.command, in.suffix)
}

func commandLevel(in chainInput) string {
	return candidate(in.category, in.command)
}

func originalToolSuffixLevel(in chainInput) string {
	return candidate(in.category, in.originalTool, in.suffix)
}

// originalToolLevel skips the original tool when it is the command itself.
func originalToolLevel(in chainInput) string {
	if in.originalTool == in.command {
		return ""
	}
	return candidate(in.category, in.originalTool)
}

func operationLevel(in chainInput) string {
	return candidate(in.category, in.operation)
}

// eventSoundNames renames the simple-event operations whose sound name
// differs from the operation; other operations are used as-is.
var eventSoundNames = map[string]string{
	"prompt":  "prompt-submit",
	"compact": "pre-compact",
}

func eventLevel(in chainInput) string {
	if name, ok := eventSoundNames[in.operation]; ok {
		return candidate(in.category, name)
	}
	return candidate(in.category, in.operation)
}

func categoryLevel(in chainInput) string {
	if in.category == "unknown" {
		return ""
	}
	return candidate(in.category, in.category)
}

func defaultLevel(chainInput) string {
	return "default.wav"
}

// candidate joins the normalized parts into "<category>/<a>-<b>.wav". A
// part that is empty or normalizes to nothing skips the level.
func candidate(category string, parts ...string) string {
	names := make([]string, len(parts))
	for i, part := range parts {
		names[i] = normalizeName(part)
		if names[i] == "" {
			return ""
		}
	}
	return category + "/" + strings.Join(names, "-") + ".wav"
}

// commandOf returns the command and subcommand the command levels are
// built from. The parser fills EventContext.Command for every tool event;
// hand-built contexts without it fall back to ToolName with no subcommand.
func commandOf(eventCtx *hooks.EventContext) (command, subcommand string) {
	if eventCtx.Command != "" {
		return eventCtx.Command, eventCtx.Subcommand
	}
	return eventCtx.ToolName, ""
}

// categoryPhases is the phase a tool event's category implies, for
// hand-built contexts that do not set Phase.
var categoryPhases = map[hooks.EventCategory]string{
	hooks.Loading: "start",
	hooks.Success: "success",
	hooks.Error:   "error",
}

// phaseOf returns the suffix for the command and original-tool levels.
func phaseOf(eventCtx *hooks.EventContext) string {
	if eventCtx.Phase != "" {
		return eventCtx.Phase
	}
	return categoryPhases[eventCtx.Category]
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
func (m *SoundMapper) finalizeResult(ctx context.Context, eventCtx *hooks.EventContext, paths []string, chainType ChainType) *SoundMappingResult {
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
			// path only when no observer winner was captured.
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
		"category", eventCtx.Category.String(),
		"tool_name", eventCtx.ToolName,
		"sound_hint", eventCtx.SoundHint,
		"chain_type", result.ChainType,
		"selected_path", result.SelectedPath,
		"fallback_level", result.FallbackLevel,
		"all_paths", result.AllPaths)

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
		} else {
			result.WriteRune('-') // existing hyphens stay hyphens
		}
	}

	normalized = result.String()

	// Clean up multiple consecutive hyphens (but don't remove them entirely)
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	// Remove leading/trailing hyphens
	return strings.Trim(normalized, "-")
}
