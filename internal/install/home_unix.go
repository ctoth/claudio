//go:build !windows

package install

import "os"

// getHomeDirectory returns the user's home directory on Unix-like platforms.
func getHomeDirectory() string {
	return os.Getenv("HOME")
}

func appendUserProfilePath(paths []string, homeDir, dirName, fileName string) []string {
	return paths
}
