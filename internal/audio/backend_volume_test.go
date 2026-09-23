package audio

import (
	"math"
	"testing"
)

// Every backend rejects what volume.Validate rejects; NaN fails both range
// comparisons, so it needs the explicit check.
func TestBackendsRejectNonFiniteVolume(t *testing.T) {
	backends := map[string]AudioBackend{
		"system_command": NewSystemCommandBackend("true"),
	}
	for name, b := range backends {
		for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1)), -1, 2} {
			if err := b.SetVolume(v); err == nil {
				t.Errorf("%s: SetVolume(%v) = nil, want error", name, v)
			}
		}
	}
}
