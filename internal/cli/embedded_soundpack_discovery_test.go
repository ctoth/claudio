package cli

import "testing"

func TestDiscoverEmbeddedSoundpacksIncludesLinux(t *testing.T) {
	packs, err := discoverEmbeddedSoundpacks()
	if err != nil {
		t.Fatalf("discoverEmbeddedSoundpacks returned error: %v", err)
	}

	found := map[string]bool{}
	for _, pack := range packs {
		if pack.Type == "embedded" {
			found[pack.Name] = true
		}
	}

	for _, name := range []string{"windows", "wsl", "darwin", "linux"} {
		if !found[name] {
			t.Errorf("expected embedded soundpack discovery to include %q", name)
		}
	}
}
