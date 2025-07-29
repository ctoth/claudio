package uninstall

import (
	"log/slog"

	"github.com/ctoth/claudio/internal/install"
)

// removeSimpleClaudioHooks removes simple string claudio hooks from settings
func removeSimpleClaudioHooks(settings *install.SettingsMap, hookNames []string) {
	slog.Debug("removing simple claudio hooks", "hook_names", hookNames)
	
	if settings == nil {
		slog.Debug("settings is nil, nothing to remove")
		return
	}
	
	hooksInterface, exists := (*settings)["hooks"]
	if !exists {
		slog.Debug("no hooks section found in settings")
		return
	}
	
	hooksMap, ok := hooksInterface.(map[string]interface{})
	if !ok {
		slog.Warn("hooks section is not a map", "type", typeof(hooksInterface))
		return
	}
	
	removedCount := 0
	for _, hookName := range hookNames {
		slog.Debug("checking hook for removal", "name", hookName)
		
		if hookValue, exists := hooksMap[hookName]; exists {
			// Only remove if the value is exactly "claudio"
			if stringValue, ok := hookValue.(string); ok && stringValue == "claudio" {
				slog.Debug("removing simple claudio hook", "name", hookName)
				delete(hooksMap, hookName)
				removedCount++
			} else {
				slog.Debug("hook exists but is not simple claudio hook", 
					"name", hookName, "value", hookValue)
			}
		} else {
			slog.Debug("hook does not exist", "name", hookName)
		}
	}
	
	// If hooks map is now empty, remove the entire hooks section
	if len(hooksMap) == 0 {
		slog.Debug("hooks map is empty, removing hooks section")
		delete(*settings, "hooks")
	}
	
	slog.Info("simple claudio hook removal completed", 
		"removed_count", removedCount, 
		"remaining_hooks", len(hooksMap))
}