package install

import (
	"errors"
	"path/filepath"
	"strings"
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
	return []string{filepath.Join(homeDir, dirName, fileName)}, nil
}

// normalizeMSYSPath converts MSYS/Git Bash-style paths (e.g. /c/Users/Q) to
// native Windows paths (e.g. C:\Users\Q). Returns the path unchanged if it
// doesn't match the MSYS pattern.
func normalizeMSYSPath(path string) string {
	// MSYS pattern: /X/... where X is a single drive letter
	if len(path) >= 3 && path[0] == '/' && path[2] == '/' &&
		((path[1] >= 'a' && path[1] <= 'z') || (path[1] >= 'A' && path[1] <= 'Z')) {
		drive := strings.ToUpper(string(path[1]))
		return drive + ":" + filepath.FromSlash(path[2:])
	}
	return path
}
