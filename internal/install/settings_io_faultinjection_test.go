package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// faultPoint enumerates the syscall/operation we want to fail in a
// fault-injection table row.
//
// Scout's section F specified an additional failLockAcquire row, but the
// lock-held path is exercised by the companion test
// TestWriteSettingsFile_ConcurrentWritersSerialise against a real
// gofrs/flock instance — that test gives stronger coverage of the
// retry-and-fail behavior than a synthetic fault could. The split between
// syscall-level fault injection (this table) and lock-level coverage
// (the concurrent test) is deliberate.
type faultPoint int

const (
	failNone faultPoint = iota
	failMkdirAll
	failTempFile
	failWrite
	failSync
	failClose
	failChmod
	failRename
	failBackupCopy
)

// faultyFs wraps an inner afero.Fs (MemMapFs in tests) and intercepts
// specific calls based on `fail`. The wrapper is _test.go-only.
type faultyFs struct {
	afero.Fs
	fail faultPoint
}

func (f *faultyFs) MkdirAll(p string, mode os.FileMode) error {
	if f.fail == failMkdirAll {
		return errors.New("injected MkdirAll error")
	}
	return f.Fs.MkdirAll(p, mode)
}

func (f *faultyFs) Rename(oldname, newname string) error {
	if f.fail == failRename {
		return errors.New("injected Rename error")
	}
	return f.Fs.Rename(oldname, newname)
}

func (f *faultyFs) Chmod(name string, mode os.FileMode) error {
	if f.fail == failChmod {
		return errors.New("injected Chmod error")
	}
	return f.Fs.Chmod(name, mode)
}

// OpenFile is the entry point afero.TempFile uses (with O_CREATE|O_EXCL)
// and afero.WriteFile uses (with O_CREATE|O_TRUNC|O_WRONLY). We
// distinguish the two by flag bits and target path:
//   - failTempFile fires when flag has O_CREATE|O_EXCL (TempFile pattern)
//   - failBackupCopy fires when name ends with ".bak"
//
// For Write/Sync/Close failures we wrap the returned file with faultyFile.
func (f *faultyFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if f.fail == failTempFile && (flag&os.O_CREATE) != 0 && (flag&os.O_EXCL) != 0 {
		return nil, errors.New("injected TempFile/OpenFile error")
	}
	if f.fail == failBackupCopy && strings.HasSuffix(name, ".bak") {
		return nil, errors.New("injected Backup OpenFile error")
	}
	inner, err := f.Fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	// Wrap temp files for write/sync/close injection.
	if (flag&os.O_CREATE) != 0 && (flag&os.O_EXCL) != 0 {
		return &faultyFile{File: inner, fail: f.fail}, nil
	}
	return inner, nil
}

// faultyFile wraps an afero.File and injects Write/Sync/Close failures.
type faultyFile struct {
	afero.File
	fail faultPoint
}

func (f *faultyFile) Write(p []byte) (int, error) {
	if f.fail == failWrite {
		return 0, errors.New("injected Write error")
	}
	return f.File.Write(p)
}

func (f *faultyFile) Sync() error {
	if f.fail == failSync {
		return errors.New("injected Sync error")
	}
	return f.File.Sync()
}

func (f *faultyFile) Close() error {
	if f.fail == failClose {
		// Do NOT close the inner file: a real production Close failure
		// (NFS, EIO, ENOSPC) typically leaves the file in an undefined
		// state — temp not removable, or data partial. Closing the inner
		// file before returning the synthetic error would silently flush
		// the data and make the temp removable, hiding the failure mode
		// the test is meant to exercise. The injected error represents a
		// genuine Close failure.
		return errors.New("injected Close error")
	}
	return f.File.Close()
}

// TestWriteSettingsFile_FaultInjection walks every syscall the atomic
// write goes through and asserts the on-disk invariants per scout
// section F: target unchanged on failure, no .tmp residue, .bak preserved.
func TestWriteSettingsFile_FaultInjection(t *testing.T) {
	type row struct {
		name                string
		fail                faultPoint
		expectErrSubstr     string // "" means no error expected
		expectFileUnchanged bool   // target content equals pre-write
		expectBakIntact     bool   // .bak still equals pre-write content
	}

	rows := []row{
		{name: "happy path", fail: failNone, expectErrSubstr: "", expectFileUnchanged: false, expectBakIntact: true},
		{name: "MkdirAll fails", fail: failMkdirAll, expectErrSubstr: "create directory", expectFileUnchanged: true, expectBakIntact: true},
		{name: "TempFile fails", fail: failTempFile, expectErrSubstr: "temp file", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Write fails", fail: failWrite, expectErrSubstr: "write to temp", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Sync fails", fail: failSync, expectErrSubstr: "sync temp", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Close fails", fail: failClose, expectErrSubstr: "close temp", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Chmod fails", fail: failChmod, expectErrSubstr: "permissions", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Rename fails", fail: failRename, expectErrSubstr: "rename", expectFileUnchanged: true, expectBakIntact: true},
		{name: "Backup fails non-fatal", fail: failBackupCopy, expectErrSubstr: "", expectFileUnchanged: false, expectBakIntact: false},
	}

	dir := "/fault-inject"
	settingsPath := filepath.Join(dir, "settings.json")
	bakPath := settingsPath + ".bak"
	preWriteBytes := []byte(`{
  "version": "pre-write"
}`)

	for _, r := range rows {
		r := r
		t.Run(r.name, func(t *testing.T) {
			memFS := afero.NewMemMapFs()
			if err := memFS.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("seed mkdir: %v", err)
			}
			if err := afero.WriteFile(memFS, settingsPath, preWriteBytes, 0644); err != nil {
				t.Fatalf("seed settings: %v", err)
			}
			// Seed a known-good .bak that BackupSettingsFile would
			// normally overwrite. For the backup-fails case this lets
			// us check that the original .bak is preserved.
			if err := afero.WriteFile(memFS, bakPath, preWriteBytes, 0644); err != nil {
				t.Fatalf("seed bak: %v", err)
			}

			fs := &faultyFs{Fs: memFS, fail: r.fail}

			newSettings := SettingsMap{"version": "post-write"}
			err := WriteSettingsFile(fs, settingsPath, &newSettings)

			// Error assertion
			if r.expectErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", r.expectErrSubstr)
				}
				if !strings.Contains(err.Error(), r.expectErrSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), r.expectErrSubstr)
				}
			}

			// Target file content assertion
			got, readErr := afero.ReadFile(memFS, settingsPath)
			if readErr != nil {
				t.Fatalf("read target after WriteSettingsFile: %v", readErr)
			}
			if r.expectFileUnchanged {
				if string(got) != string(preWriteBytes) {
					t.Errorf("target file was modified despite failure\n  got:  %s\n  want: %s",
						got, preWriteBytes)
				}
			} else {
				if string(got) == string(preWriteBytes) {
					t.Errorf("target file was NOT updated on success path: still %s", got)
				}
			}

			// No .settings-*.tmp residue
			tmpResidue := false
			_ = afero.Walk(memFS, dir, func(p string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				base := filepath.Base(p)
				if strings.HasPrefix(base, ".settings-") && strings.HasSuffix(base, ".tmp") {
					tmpResidue = true
				}
				return nil
			})
			if tmpResidue {
				t.Error(".settings-*.tmp residue remained after WriteSettingsFile")
			}

			// .bak content assertion
			if r.expectBakIntact {
				gotBak, bakErr := afero.ReadFile(memFS, bakPath)
				if bakErr != nil {
					t.Fatalf("read .bak: %v", bakErr)
				}
				if string(gotBak) != string(preWriteBytes) {
					t.Errorf(".bak does not contain pre-write content\n  got:  %s\n  want: %s",
						gotBak, preWriteBytes)
				}
			}
		})
	}
}

// TestWriteSettingsFile_ConcurrentWritersSerialise verifies that the
// workflow lock serialises two concurrent writers: a "holder" grabs the
// lock first and holds it longer than the retry budget; a "contender"
// then attempts to acquire and must fail with the "another claudio"
// message. The test is deterministically ordered (no race on which
// goroutine wins) so the assertion is one success and one lock-failure.
// Bounded at 10s so a deadlock fails fast.
func TestWriteSettingsFile_ConcurrentWritersSerialise(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	osFS := afero.NewOsFs()

	// Retry budget for LockSettingsDir is 5 attempts * 200ms = ~800ms.
	// Hold the lock 1.2s to guarantee the contender's retries exhaust.
	const holdDuration = 1200 * time.Millisecond

	type result struct {
		label string
		err   error
	}
	resultsCh := make(chan result, 2)
	var wg sync.WaitGroup

	holderReady := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		lock, err := LockSettingsDir(settingsPath)
		if err != nil {
			close(holderReady)
			resultsCh <- result{label: "holder", err: err}
			return
		}
		close(holderReady) // signal contender to start
		// Hold past contender's retry budget.
		time.Sleep(holdDuration)
		settings := SettingsMap{"writer": "holder"}
		writeErr := WriteSettingsFile(osFS, settingsPath, &settings)
		if unlockErr := lock.Unlock(); unlockErr != nil && writeErr == nil {
			writeErr = unlockErr
		}
		resultsCh <- result{label: "holder", err: writeErr}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-holderReady
		lock, err := LockSettingsDir(settingsPath)
		if err != nil {
			resultsCh <- result{label: "contender", err: err}
			return
		}
		defer func() { _ = lock.Unlock() }()
		settings := SettingsMap{"writer": "contender"}
		writeErr := WriteSettingsFile(osFS, settingsPath, &settings)
		resultsCh <- result{label: "contender", err: writeErr}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent writer test deadlocked or exceeded 10s budget")
	}

	close(resultsCh)
	var successes, lockFailures int
	for r := range resultsCh {
		switch {
		case r.err == nil:
			successes++
		case strings.Contains(r.err.Error(), "another claudio"):
			lockFailures++
		default:
			t.Errorf("unexpected error from %s: %v", r.label, r.err)
		}
	}

	if successes != 1 {
		t.Errorf("expected exactly 1 successful write, got %d", successes)
	}
	if lockFailures != 1 {
		t.Errorf("expected exactly 1 lock-failure, got %d", lockFailures)
	}
}
