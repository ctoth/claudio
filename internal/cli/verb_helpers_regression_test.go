package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"claudio.click/internal/config"
	"github.com/adrg/xdg"
	"github.com/spf13/afero"
)

func TestSoundpackMutationPreservesEffectiveConfig(t *testing.T) {
	for _, command := range []string{"install", "use"} {
		for _, target := range []string{"implicit missing", "implicit existing", "explicit missing", "explicit user path"} {
			t.Run(command+"/"+target, func(t *testing.T) {
				dataDir, configDir, cleanup := setupInstallTestEnv(t)
				defer cleanup()
				t.Setenv("CLAUDE_CONFIG_DIR", "")
				systemDir := t.TempDir()
				t.Setenv("XDG_CONFIG_DIRS", systemDir)
				xdg.Reload()
				manager := config.NewConfigManager()
				system := manager.GetDefaultConfig()
				volume := 0.23
				system.Volume = &volume
				system.LogLevel = "debug"
				system.FileLogging.MaxBackups = 17
				system.SoundpackPaths = []string{filepath.Join(systemDir, "shared-packs")}
				systemPath := filepath.Join(systemDir, "claudio", "config.json")
				if err := config.WriteConfigFile(afero.NewOsFs(), systemPath, system); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(systemPath)
				if err != nil {
					t.Fatal(err)
				}
				userPath := filepath.Join(configDir, "claudio", "config.json")
				outputPath := userPath
				want := system
				args := []string{"claudio"}
				switch target {
				case "implicit existing":
					want = manager.GetDefaultConfig()
					want.LogLevel = "error"
					if err := config.WriteConfigFile(afero.NewOsFs(), userPath, want); err != nil {
						t.Fatal(err)
					}
				case "explicit missing", "explicit user path":
					want = manager.GetDefaultConfig()
					if target == "explicit missing" {
						outputPath = filepath.Join(t.TempDir(), "explicit.json")
					}
					args = append(args, "--config", outputPath)
				}
				if command == "install" {
					source := filepath.Join(t.TempDir(), "new-pack")
					createDummyWAV(t, filepath.Join(source, "default.wav"))
					args = append(args, "soundpack", "install", source)
					want.SoundpackPaths = append(want.SoundpackPaths, filepath.Join(dataDir, "claudio", "soundpacks", "new-pack"))
				} else {
					args = append(args, "soundpack", "use", "windows")
					want.DefaultSoundpack = "windows"
				}
				var stdout, stderr bytes.Buffer
				if code := NewCLI().Run(args, nil, &stdout, &stderr); code != 0 {
					t.Fatalf("command failed: code=%d stderr=%s", code, stderr.String())
				}
				got, err := manager.LoadFromFile(outputPath)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("persisted config lost effective settings: got %+v, want %+v", got, want)
				}
				after, err := os.ReadFile(systemPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Error("system config was modified")
				}
			})
		}
	}
}
