package config

import "testing"

func TestEffectiveVolume(t *testing.T) {
	if got := (&Config{}).EffectiveVolume(); got != DefaultVolume {
		t.Errorf("unset volume: EffectiveVolume() = %v, want DefaultVolume %v", got, DefaultVolume)
	}
	v := 0.2
	if got := (&Config{Volume: &v}).EffectiveVolume(); got != 0.2 {
		t.Errorf("EffectiveVolume() = %v, want 0.2", got)
	}
	if got := *NewConfigManager().GetDefaultConfig().Volume; got != DefaultVolume {
		t.Errorf("default config volume = %v, want DefaultVolume %v", got, DefaultVolume)
	}
}
