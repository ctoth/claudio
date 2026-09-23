//go:build !windows

package install

import "os"

// getHomeDirectory returns the user's home directory on Unix-like platforms.
func getHomeDirectory() string {
	return os.Getenv("HOME")
}
