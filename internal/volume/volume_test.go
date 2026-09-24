package volume

import (
	"math"
	"testing"
)

func TestValidate(t *testing.T) {
	for _, v := range []float64{0, 0.25, 1} {
		if err := Validate(v); err != nil {
			t.Errorf("Validate(%v) = %v, want nil", v, err)
		}
	}
	for _, v := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := Validate(v); err == nil {
			t.Errorf("Validate(%v) = nil, want error", v)
		}
	}
}
