package install

import (
	"log/slog"
	"maps"
	"sort"
)

// This file is claudio's one hook strip engine. Install (merge) and
// uninstall both remove claudio entries through stripClaudioEntries, and
// every "does this contain a claudio hook" question is answered by the
// same walk, so recognition and removal cannot drift apart.

// stripClaudioEntries returns the hook-array elements that remain after
// removing claudio commands, and how many commands it removed. An element
// is either a command object ({"command": ...}, the flat Copilot shape) or
// a matcher group ({"matcher": ..., "hooks": [commands]}). A group is
// dropped only when removal emptied its hooks; groups without claudio
// commands, including pre-existing empty ones, are kept verbatim. The
// input is never modified.
func stripClaudioEntries(entries []any) ([]any, int) {
	kept := make([]any, 0, len(entries))
	removed := 0
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if isClaudioCommandValue(entryMap["command"]) {
			removed++
			continue
		}
		hooks, ok := entryMap["hooks"].([]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		keptHooks := make([]any, 0, len(hooks))
		for _, hook := range hooks {
			if hookMap, ok := hook.(map[string]any); ok && isClaudioCommandValue(hookMap["command"]) {
				continue
			}
			keptHooks = append(keptHooks, hook)
		}
		groupRemoved := len(hooks) - len(keptHooks)
		if groupRemoved == 0 {
			kept = append(kept, entry)
			continue
		}
		removed += groupRemoved
		if len(keptHooks) == 0 {
			continue
		}
		group := make(map[string]any, len(entryMap))
		maps.Copy(group, entryMap)
		group["hooks"] = keptHooks
		kept = append(kept, group)
	}
	return kept, removed
}

// isClaudioCommandValue reports whether a JSON "command" value names the
// claudio executable.
func isClaudioCommandValue(v any) bool {
	s, ok := v.(string)
	return ok && IsClaudioCommandString(s)
}

// stripClaudioHookValue removes claudio commands from one hook event's
// value. It returns the remaining value, the number of commands removed,
// and whether the event should be kept at all (false when removal left it
// empty). Legacy string values are one command.
func stripClaudioHookValue(value any) (any, int, bool) {
	switch v := value.(type) {
	case string:
		if IsClaudioCommandString(v) {
			return nil, 1, false
		}
	case []any:
		kept, removed := stripClaudioEntries(v)
		if removed == 0 {
			return value, 0, true
		}
		return kept, removed, len(kept) > 0
	}
	return value, 0, true
}

// IsClaudioHook reports whether a hook event's value (legacy string or
// array of entries) contains any claudio command.
func IsClaudioHook(hookValue any) bool {
	_, removed, _ := stripClaudioHookValue(hookValue)
	return removed > 0
}

// hooksSection returns settings["hooks"] when it is a JSON object.
func hooksSection(settings *SettingsMap) (map[string]any, bool) {
	if settings == nil {
		return nil, false
	}
	hooks, ok := (*settings)["hooks"].(map[string]any)
	return hooks, ok
}

// ClaudioHookNames returns the sorted names of hook events that contain a
// claudio command.
func ClaudioHookNames(settings *SettingsMap) []string {
	hooks, ok := hooksSection(settings)
	if !ok {
		return nil
	}
	var names []string
	for name, value := range hooks {
		if IsClaudioHook(value) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// removeClaudioHooks strips every claudio command from settings in place,
// deleting events that become empty and the hooks section if it empties.
// It returns the sorted names of the events it changed.
func removeClaudioHooks(settings *SettingsMap) []string {
	hooks, ok := hooksSection(settings)
	if !ok {
		return nil
	}
	var names []string
	for name, value := range hooks {
		kept, removed, keep := stripClaudioHookValue(value)
		if removed == 0 {
			continue
		}
		names = append(names, name)
		if keep {
			hooks[name] = kept
		} else {
			delete(hooks, name)
		}
	}
	if len(names) > 0 && len(hooks) == 0 {
		delete(*settings, "hooks")
	}
	sort.Strings(names)
	slog.Debug("removed claudio hooks", "hooks", names, "remaining_hooks", len(hooks))
	return names
}
