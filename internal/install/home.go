package install

import (
	"errors"
	"path/filepath"
)

// errNoHomeDirectory is returned for global scope when no home directory
// can be determined. Falling back to a literal "~" path would create a
// "~" directory under the current working directory.
var errNoHomeDirectory = errors.New("cannot determine home directory: set HOME (or USERPROFILE on Windows)")

// homeScopedPaths returns the global-scope candidates <home>/dirName/fileName.
func homeScopedPaths(dirName, fileName string) ([]string, error) {
	homeDir := getHomeDirectory()
	if homeDir == "" {
		return nil, errNoHomeDirectory
	}
	paths := []string{filepath.Join(homeDir, dirName, fileName)}
	return appendUserProfilePath(paths, homeDir, dirName, fileName), nil
}
