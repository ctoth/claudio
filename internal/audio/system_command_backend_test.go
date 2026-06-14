package audio

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewSystemCommandBackendStoresFallbackChain(t *testing.T) {
	scb := NewSystemCommandBackend("paplay", "ffplay", "aplay")
	want := []string{"paplay", "ffplay", "aplay"}
	if !reflect.DeepEqual(scb.commands, want) {
		t.Fatalf("commands = %v, want %v", scb.commands, want)
	}
}

func TestCommandSupportsFormat(t *testing.T) {
	tests := []struct {
		name    string
		command string
		ext     string
		want    bool
	}{
		{name: "aplay supports wav", command: "aplay", ext: ".wav", want: true},
		{name: "aplay supports wav case-insensitive", command: "/usr/bin/aplay", ext: ".WAV", want: true},
		{name: "aplay rejects mp3", command: "aplay", ext: ".mp3", want: false},
		{name: "aplay rejects aiff", command: "aplay", ext: ".aiff", want: false},
		{name: "paplay accepts mp3", command: "paplay", ext: ".mp3", want: true},
		{name: "ffplay accepts aiff", command: "ffplay", ext: ".aiff", want: true},
		{name: "unknown accepts mp3", command: "custom-player", ext: ".mp3", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commandSupportsFormat(tt.command, tt.ext)
			if got != tt.want {
				t.Fatalf("commandSupportsFormat(%q, %q) = %v, want %v", tt.command, tt.ext, got, tt.want)
			}
		})
	}
}

// TestSetVolume_RejectsNaN verifies SetVolume rejects NaN. NaN evaluates as
// false for both bounds checks, so without an explicit guard it would slip
// past the [0.0, 1.0] range check and reach the subprocess argv.
func TestSetVolume_RejectsNaN(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	err := scb.SetVolume(float32(math.NaN()))
	if err == nil {
		t.Fatal("SetVolume(NaN) should error")
	}
	if !strings.Contains(err.Error(), "finite") {
		t.Errorf("expected 'finite' in error, got: %v", err)
	}
}

// TestSetVolume_RejectsPosInf verifies SetVolume rejects +Inf.
func TestSetVolume_RejectsPosInf(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	if err := scb.SetVolume(float32(math.Inf(+1))); err == nil {
		t.Fatal("SetVolume(+Inf) should error")
	}
}

// TestSetVolume_RejectsNegInf verifies SetVolume rejects -Inf.
func TestSetVolume_RejectsNegInf(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	if err := scb.SetVolume(float32(math.Inf(-1))); err == nil {
		t.Fatal("SetVolume(-Inf) should error")
	}
}

// TestBuildPlayerArgv_Paplay verifies paplay's --volume=N mapping (0..65536).
func TestBuildPlayerArgv_Paplay(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string // exercises filepath.Base
		volume float64
		want   []string
	}{
		{"paplay relative half", "paplay", 0.5,
			[]string{"--volume=32768", "/tmp/s.wav"}},
		{"paplay absolute full", "/usr/bin/paplay", 1.0,
			[]string{"--volume=65536", "/tmp/s.wav"}},
		{"paplay zero", "paplay", 0.0,
			[]string{"--volume=0", "/tmp/s.wav"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scb := NewSystemCommandBackend(tc.cmd)
			got := scb.buildPlayerArgv("/tmp/s.wav", tc.volume)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildPlayerArgv_PaplayRoundsCorrectly guards against truncation drift:
// round(0.07 * 65536) = 4588, not 4587 (which truncation would give).
func TestBuildPlayerArgv_PaplayRoundsCorrectly(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	argv := scb.buildPlayerArgv("/tmp/s.wav", 0.07)
	want := []string{"--volume=4588", "/tmp/s.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("got %v, want %v", argv, want)
	}
}

// TestBuildPlayerArgv_FfplayIncludesNodispAutoexit verifies ffplay's
// -volume N integer mapping plus -nodisp -autoexit flags.
func TestBuildPlayerArgv_FfplayIncludesNodispAutoexit(t *testing.T) {
	scb := NewSystemCommandBackend("ffplay")
	argv := scb.buildPlayerArgv("/tmp/s.wav", 0.7)
	want := []string{"-nodisp", "-autoexit", "-volume", "70", "/tmp/s.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("got %v, want %v", argv, want)
	}
	s := strings.Join(argv, " ")
	if !strings.Contains(s, "-nodisp") || !strings.Contains(s, "-autoexit") {
		t.Errorf("ffplay argv must include -nodisp -autoexit, got %v", argv)
	}
}

// TestBuildPlayerArgv_AfplayIdentityNotScaled is load-bearing: review #4
// claimed 0..255 scaling; scout verified afplay accepts a float where
// 1.0 = 100%. The mapping [0,1] -> afplay -v is identity.
func TestBuildPlayerArgv_AfplayIdentityNotScaled(t *testing.T) {
	scb := NewSystemCommandBackend("afplay")
	argv := scb.buildPlayerArgv("/tmp/sound.aiff", 0.5)
	want := []string{"-v", "0.50", "/tmp/sound.aiff"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("afplay@0.5 should produce identity '-v 0.50' (not '-v 127' or '-v 255*0.5'), got %v", argv)
	}
}

func TestBuildPlayerArgv_AfplayMax(t *testing.T) {
	scb := NewSystemCommandBackend("afplay")
	argv := scb.buildPlayerArgv("/tmp/x.aiff", 1.0)
	want := []string{"-v", "1.00", "/tmp/x.aiff"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("afplay@1.0 should produce '-v 1.00', got %v", argv)
	}
}

// TestBuildPlayerArgv_AplayDropsVolume verifies aplay's no-volume-flag behavior:
// argv is filePath only, repeated calls remain idempotent. We don't directly
// assert the sync.Once-guarded WARN because that requires injecting a slog
// handler; the sync.Once mechanic is standard library and trusted.
func TestBuildPlayerArgv_AplayDropsVolume(t *testing.T) {
	scb := NewSystemCommandBackend("aplay")
	// First call at v != 1.0 -- should produce just the file path, no volume flag.
	argv := scb.buildPlayerArgv("/tmp/x.wav", 0.3)
	want := []string{"/tmp/x.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("aplay argv should be [filePath] only, got %v", argv)
	}
	// Second call -- still just the file path; idempotent.
	argv2 := scb.buildPlayerArgv("/tmp/y.wav", 0.7)
	want2 := []string{"/tmp/y.wav"}
	if !reflect.DeepEqual(argv2, want2) {
		t.Errorf("aplay argv on second call should still be [filePath], got %v", argv2)
	}
}

// TestBuildPlayerArgv_AplayFullVolumeNoWarn verifies that the warn path does
// not trigger when volume is exactly 1.0 (the default), and argv shape is
// unchanged.
func TestBuildPlayerArgv_AplayFullVolumeNoWarn(t *testing.T) {
	scb := NewSystemCommandBackend("aplay")
	argv := scb.buildPlayerArgv("/tmp/x.wav", 1.0)
	want := []string{"/tmp/x.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("aplay@1.0 argv should be [filePath], got %v", argv)
	}
}

// TestBuildPlayerArgv_UnknownCommandFallsBack confirms the default branch
// passes only filePath. TestSystemCommandBackend_Play uses a harmless
// platform command, but echo still exercises the default argv branch here.
func TestBuildPlayerArgv_UnknownCommandFallsBack(t *testing.T) {
	scb := NewSystemCommandBackend("echo")
	argv := scb.buildPlayerArgv("/tmp/x.wav", 0.5)
	want := []string{"/tmp/x.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("default branch should pass only filePath, got %v", argv)
	}
}

// TestBuildPlayerArgv_DefaultBackendVolumeIsOne sanity-checks that a newly
// constructed backend's loadVolume returns 1.0, so the default playback path
// produces full-volume argv even if SetVolume is never called.
func TestBuildPlayerArgv_DefaultBackendVolumeIsOne(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	v := scb.loadVolume()
	if v != 1.0 {
		t.Errorf("default volume should be 1.0, got %f", v)
	}
}

// TestBuildPlayerArgv_VolumeReachesArgv is an integration-level test that
// asserts the SetVolume -> loadVolume -> buildPlayerArgv pipeline -- the
// exact regression that review #4 caught (volume was stored but never
// reached the subprocess).
func TestBuildPlayerArgv_VolumeReachesArgv(t *testing.T) {
	scb := NewSystemCommandBackend("paplay")
	if err := scb.SetVolume(0.25); err != nil {
		t.Fatalf("SetVolume(0.25) failed: %v", err)
	}
	v := scb.loadVolume()
	argv := scb.buildPlayerArgv("/tmp/s.wav", float64(v))
	want := []string{"--volume=16384", "/tmp/s.wav"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("volume set via SetVolume should reach argv as --volume=16384; got %v", argv)
	}
}

func TestPlayFileWithFallbackChain(t *testing.T) {
	tmpDir := t.TempDir()
	wavFile := filepath.Join(tmpDir, "sound.wav")
	if err := os.WriteFile(wavFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("write wav fixture: %v", err)
	}

	t.Run("primary command fails then fallback succeeds", func(t *testing.T) {
		scb := NewSystemCommandBackend("nonexistent-command-claudio", successfulNoopCommand())
		if err := scb.playFile(context.Background(), wavFile); err != nil {
			t.Fatalf("playFile should succeed via fallback: %v", err)
		}
	})

	t.Run("all commands fail", func(t *testing.T) {
		scb := NewSystemCommandBackend("nonexistent-command-one", "nonexistent-command-two")
		if err := scb.playFile(context.Background(), wavFile); err == nil {
			t.Fatal("playFile should fail when every command fails")
		}
	})

	t.Run("skips format-incompatible commands", func(t *testing.T) {
		mp3File := filepath.Join(tmpDir, "sound.mp3")
		if err := os.WriteFile(mp3File, []byte("fake mp3"), 0644); err != nil {
			t.Fatalf("write mp3 fixture: %v", err)
		}

		scb := NewSystemCommandBackend("aplay", successfulNoopCommand())
		if err := scb.playFile(context.Background(), mp3File); err != nil {
			t.Fatalf("playFile should skip aplay for mp3 and use fallback: %v", err)
		}
	})

	t.Run("all commands skipped for unsupported format", func(t *testing.T) {
		mp3File := filepath.Join(tmpDir, "unsupported.mp3")
		if err := os.WriteFile(mp3File, []byte("fake mp3"), 0644); err != nil {
			t.Fatalf("write mp3 fixture: %v", err)
		}

		scb := NewSystemCommandBackend("aplay")
		if err := scb.playFile(context.Background(), mp3File); err == nil {
			t.Fatal("playFile should fail when every command is format-incompatible")
		}
	})
}
