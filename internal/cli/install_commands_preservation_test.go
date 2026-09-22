package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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

// Shipped content must never be edited in place: files written by older
// releases are only recognized as claudio-owned if their content is kept in a
// retired list. If this test fails, append the previous string to the matching
// retired list, then update the hash here.
func TestCommandArtifactShippedContentIsPinned(t *testing.T) {
	pinned := map[string]struct{ content, hash string }{
		"claudioCommandContent": {claudioCommandContent, "060204a40fe2765db1e395dd8d6a0ecda76b07a0d48079d18b6011c4c1f539ed"},
		"claudioSkillContent":   {claudioSkillContent, "400b29ef4b1727cba4f79f6cacb63b33b7cf7c0dfa19ac20a37727ff5962932a"},
	}
	for name, p := range pinned {
		sum := sha256.Sum256([]byte(p.content))
		if got := hex.EncodeToString(sum[:]); got != p.hash {
			t.Errorf("%s changed (sha256 %s); move the old text to its retired list and update the pin", name, got)
		}
	}
}

const retiredSample = "old claudio skill\n"

func TestCommandArtifactInstallReplacesRetiredContent(t *testing.T) {
	for _, retired := range []string{retiredSample, strings.ReplaceAll(retiredSample, "\n", "\r\n")} {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, []byte(retired), 0600); err != nil {
			t.Fatal(err)
		}
		artifact := commandArtifact{Directory: dir, Path: path, Content: claudioSkillContent, Retired: []string{retiredSample}}
		if err := preflightCommandArtifacts([]commandArtifact{artifact}); err != nil {
			t.Fatalf("preflight rejected retired content %q: %v", retired, err)
		}
		if err := installCommandArtifact(artifact); err != nil {
			t.Fatalf("install rejected retired content %q: %v", retired, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil || string(contents) != claudioSkillContent {
			t.Fatalf("retired content not replaced: %q, %v", contents, err)
		}
	}
}

func TestCommandArtifactUninstallRemovesRetiredContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(retiredSample), 0600); err != nil {
		t.Fatal(err)
	}
	artifact := commandArtifact{Directory: dir, Path: path, Content: claudioSkillContent, Retired: []string{retiredSample}}
	removed, err := uninstallCommandArtifact(artifact)
	if err != nil || !removed {
		t.Fatalf("expected retired content removed, got removed=%v error=%v", removed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("retired artifact still present: %v", err)
	}
}

func TestCommandArtifactCRLFCurrentContentIsOwned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	crlf := strings.ReplaceAll(claudioSkillContent, "\n", "\r\n")
	if err := os.WriteFile(path, []byte(crlf), 0600); err != nil {
		t.Fatal(err)
	}
	artifact := commandArtifact{Directory: dir, Path: path, Content: claudioSkillContent}
	if err := installCommandArtifact(artifact); err != nil {
		t.Fatalf("install rejected CRLF copy of current content: %v", err)
	}
	removed, err := uninstallCommandArtifact(artifact)
	if err != nil || !removed {
		t.Fatalf("uninstall rejected CRLF copy of current content: removed=%v error=%v", removed, err)
	}
}

func TestCommandArtifactsWireRetiredContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	oldCommand, oldSkill := retiredClaudioCommandContents, retiredClaudioSkillContents
	t.Cleanup(func() { retiredClaudioCommandContents, retiredClaudioSkillContents = oldCommand, oldSkill })
	retiredClaudioCommandContents = []string{"retired command"}
	retiredClaudioSkillContents = []string{"retired skill"}
	for _, agent := range []commandArtifactAgent{commandArtifactAgentClaude, commandArtifactAgentCodex, commandArtifactAgentAntigravity} {
		artifacts, err := resolveCommandArtifacts(agent)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range artifacts {
			want := "retired skill"
			if a.Content == claudioCommandContent {
				want = "retired command"
			}
			if len(a.Retired) != 1 || a.Retired[0] != want {
				t.Errorf("%s %s: retired list %q, want [%q]", agent, a.Kind, a.Retired, want)
			}
		}
	}
}
