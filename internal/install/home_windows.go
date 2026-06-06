//go:build windows

package install

import (
	"os"
	"path/filepath"
)

// getHomeDirectory returns the user's home directory using Windows-specific
// environment fallbacks.
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

func appendUserProfilePath(paths []string, homeDir, dirName, fileName string) []string {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" && userProfile != homeDir {
		return append(paths, filepath.Join(userProfile, dirName, fileName))
	}
	return paths
}
