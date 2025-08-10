package util

import "claudio.click/internal/install"

// GetSettingsKeys returns a list of top-level keys in settings for logging
func GetSettingsKeys(settings *install.SettingsMap) []string {
	if settings == nil {
		return []string{}
	}

	keys := make([]string, 0, len(*settings))
	for key := range *settings {
		keys = append(keys, key)
	}
	return keys
}