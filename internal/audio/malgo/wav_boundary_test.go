//go:build cgo

package malgo

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
)

func TestWAVRejectsUnsupportedOrInconsistentFrameLayout(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		channels, alignment, format uint16
	}{
		{"three channels", 3, 6, 1},
		{"zero block alignment", 1, 0, 1},
		{"short block alignment", 2, 2, 1},
		{"16-bit float", 1, 2, 3},
		{"unknown encoding", 1, 2, 99},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("Decode panicked instead of rejecting header: %v", p)
				}
			}()
			data := generateTestWAV()
			binary.LittleEndian.PutUint16(data[20:22], tc.format)
			binary.LittleEndian.PutUint16(data[22:24], tc.channels)
			binary.LittleEndian.PutUint16(data[32:34], tc.alignment)
			if _, err := NewWavDecoder().Decode(context.Background(), bytes.NewReader(data)); err == nil {
				t.Fatal("unsupported frame layout accepted")
			}
		})
	}
}
