// Package volume holds the single volume rule shared by config, the CLI and
// the audio backends. It is a leaf package so config does not have to import
// the audio stack to validate a volume.
package volume

import (
	"fmt"
	"math"
)

// Validate reports whether v is a usable volume: finite and within
// [0.0, 1.0]. NaN fails both range comparisons, so it is checked first.
func Validate(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("volume must be a finite number between 0.0 and 1.0, got %v", v)
	}
	if v < 0.0 || v > 1.0 {
		return fmt.Errorf("volume must be between 0.0 and 1.0, got %v", v)
	}
	return nil
}
