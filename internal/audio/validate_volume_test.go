package audio

import (
	"math"
	"testing"
)

func TestValidateVolume(t *testing.T) {
	for _, v := range []float64{0, 0.25, 1} {
		if err := ValidateVolume(v); err != nil {
			t.Errorf("ValidateVolume(%v) = %v, want nil", v, err)
		}
	}
	for _, v := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := ValidateVolume(v); err == nil {
			t.Errorf("ValidateVolume(%v) = nil, want error", v)
		}
	}
}

// Every backend rejects what ValidateVolume rejects; the fake used to let
// NaN through because NaN fails both range comparisons.
func TestBackendsRejectNonFiniteVolume(t *testing.T) {
	backends := map[string]AudioBackend{
		"fake":           NewFakeBackend(),
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
