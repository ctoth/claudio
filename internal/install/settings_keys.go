package install

// SettingsKeys returns a list of top-level keys in settings for logging.
// Returns an empty slice if settings is nil. Used by install/uninstall
// workflows to surface which top-level settings keys were present in the
// file at read time.
func SettingsKeys(settings *SettingsMap) []string {
	if settings == nil {
		return []string{}
	}

	keys := make([]string, 0, len(*settings))
	for key := range *settings {
		keys = append(keys, key)
	}
	return keys
}
