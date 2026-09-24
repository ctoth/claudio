package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
)

func TestRunStoresSanitizedUniquePrivateFilesInUserCache(t *testing.T) {
	cacheRoot := isolateUserCache(t)
	payload := `{"hook_event_name":"../../Session Start\n","session_id":"test"}`

	for range 2 {
		var stderr bytes.Buffer
		if code := run(strings.NewReader(payload), &stderr); code != 0 {
			t.Fatalf("run exit code = %d, stderr = %s", code, stderr.String())
		}
	}

	logsDir := filepath.Join(cacheRoot, "claudio", "hook-logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("read log directory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("log file count = %d, want 2", len(entries))
	}
	if entries[0].Name() == entries[1].Name() {
		t.Fatalf("log filename collision: %q", entries[0].Name())
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), "..") || !strings.Contains(entry.Name(), "Session_Start") {
			t.Errorf("unsafe or unsanitized filename %q", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(logsDir, entry.Name()))
		if err != nil {
			t.Fatalf("read log file: %v", err)
		}
		if string(data) != payload {
			t.Errorf("saved payload = %q, want %q", data, payload)
		}
	}

	if runtime.GOOS != "windows" {
		assertPermissions(t, logsDir, 0o700)
		for _, entry := range entries {
			assertPermissions(t, filepath.Join(logsDir, entry.Name()), 0o600)
		}
	}
}

func TestRunReturnsAfterCompleteJSONWithoutEOF(t *testing.T) {
	isolateUserCache(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	result := make(chan int, 1)
	go func() {
		_, _ = io.WriteString(writer, `{"hook_event_name":"Stop"}`)
	}()
	go func() {
		result <- run(reader, io.Discard)
	}()

	select {
	case code := <-result:
		if code != 0 {
			t.Fatalf("run exit code = %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("run waited for EOF after receiving complete JSON")
	}
}

func TestRunDoesNotEchoOrSaveInvalidPayload(t *testing.T) {
	cacheRoot := isolateUserCache(t)
	const payload = `{"secret-token":`
	var stderr bytes.Buffer

	if code := run(strings.NewReader(payload), &stderr); code == 0 {
		t.Fatal("run accepted invalid JSON")
	}
	if strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("stderr exposed invalid payload: %s", stderr.String())
	}

	logsDir := filepath.Join(cacheRoot, "claudio", "hook-logs")
	if entries, err := os.ReadDir(logsDir); err == nil && len(entries) != 0 {
		t.Fatalf("invalid payload created %d log files", len(entries))
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read log directory: %v", err)
	}
}

// TestIsolateUserCacheRestoresXDGAfterTest pins issue #92: once a test using
// isolateUserCache ends, xdg.CacheHome must not point at the deleted sandbox.
func TestIsolateUserCacheRestoresXDGAfterTest(t *testing.T) {
	before := xdg.CacheHome
	t.Run("isolated", func(t *testing.T) {
		isolateUserCache(t)
	})
	if xdg.CacheHome != before {
		t.Errorf("xdg.CacheHome = %q after the isolated test, want %q", xdg.CacheHome, before)
	}
}

func isolateUserCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Registered before t.Setenv so it runs after the env is restored (#92).
	t.Cleanup(xdg.Reload)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("XDG_CACHE_HOME", root)
	xdg.Reload()

	// Hook logs share claudio's XDG cache root (not os.UserCacheDir,
	// which differs from it on Windows and macOS).
	cacheRoot := xdg.CacheHome
	if filepath.Clean(cacheRoot) != filepath.Clean(root) {
		t.Fatalf("XDG cache home %q, want %q", cacheRoot, root)
	}
	return cacheRoot
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s permissions = %#o, want %#o", path, got, want)
	}
}
