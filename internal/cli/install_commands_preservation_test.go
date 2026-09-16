package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandArtifactInstallPreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("user-owned content"), 0600); err != nil {
		t.Fatal(err)
	}
	artifact := commandArtifact{Directory: dir, Path: path, Content: claudioSkillContent}
	if err := installCommandArtifact(artifact); err == nil {
		t.Fatal("expected refusal to overwrite an existing custom skill")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "user-owned content" {
		t.Fatalf("custom skill changed: %q, %v", contents, err)
	}
}

func TestCommandArtifactBatchChecksConflictsBeforeChanges(t *testing.T) {
	for _, uninstall := range []bool{false, true} {
		name := "install"
		if uninstall {
			name = "uninstall"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			artifacts, err := resolveCommandArtifacts(commandArtifactAgentAntigravity)
			if err != nil {
				t.Fatal(err)
			}
			if uninstall {
				if err := installCommandArtifact(artifacts[0]); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(artifacts[1].Directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(artifacts[1].Path, []byte("custom content"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			cmd.Flags().String("agent", "antigravity", "")
			if uninstall {
				err = runUninstallCommandsE(cmd, nil)
			} else {
				err = runInstallCommandsE(cmd, nil)
			}
			if err == nil {
				t.Fatal("expected conflict in second artifact")
			}
			data, err := os.ReadFile(artifacts[0].Path)
			if uninstall {
				if err != nil || string(data) != artifacts[0].Content {
					t.Fatalf("first artifact removed before detecting conflict: %v", err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("first artifact created before detecting conflict: %v", err)
			}
		})
	}
}

func TestCommandArtifactUninstallPreservesModifiedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	custom := claudioSkillContent + "\nUser customization.\n"
	if err := os.WriteFile(path, []byte(custom), 0600); err != nil {
		t.Fatal(err)
	}
	artifact := commandArtifact{Directory: dir, Path: path, Content: claudioSkillContent}
	removed, err := uninstallCommandArtifact(artifact)
	if err == nil || removed {
		t.Fatalf("expected refusal to remove a customized skill, got removed=%v error=%v", removed, err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != custom {
		t.Fatalf("custom skill changed: %q, %v", contents, err)
	}
}
