//go:build windows

package install

import "os"

// getHomeDirectory returns the user's home directory using Windows-specific
// environment fallbacks: USERPROFILE, then an MSYS-style HOME (/c/Users/x),
// then HOMEDRIVE+HOMEPATH.
func getHomeDirectory() string {
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return userProfile
	}
	if home := os.Getenv("HOME"); home != "" {
		return normalizeMSYSPath(home)
	}
	if homeDrive := os.Getenv("HOMEDRIVE"); homeDrive != "" {
		if homePath := os.Getenv("HOMEPATH"); homePath != "" {
			return homeDrive + homePath
		}
	}
	return ""
}
