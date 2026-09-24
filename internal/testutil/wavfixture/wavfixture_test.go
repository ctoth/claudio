package wavfixture

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWAVHeaderDescribesThePCM(t *testing.T) {
	frames := SineFrames(10, 2, 0.5)
	data := WAV(TagPCM, 24, 48000, frames)
	if len(data) != 44+10*2*3 {
		t.Fatalf("len = %d, want %d", len(data), 44+10*2*3)
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) || string(data[8:16]) != "WAVEfmt " {
		t.Fatalf("bad RIFF/WAVE header: %q", data[:16])
	}
	le := binary.LittleEndian
	if got := le.Uint32(data[4:]); got != uint32(len(data)-8) {
		t.Errorf("RIFF size = %d, want %d", got, len(data)-8)
	}
	if ch, rate, align, depth := le.Uint16(data[22:]), le.Uint32(data[24:]), le.Uint16(data[32:]), le.Uint16(data[34:]); ch != 2 || rate != 48000 || align != 6 || depth != 24 {
		t.Errorf("fmt = ch %d rate %d align %d depth %d", ch, rate, align, depth)
	}
}

func TestAIFFAndAIFCForms(t *testing.T) {
	for _, form := range []string{"AIFF", "AIFC"} {
		data := AIFF(form, 16, 44100, SineFrames(4, 1, 0.5))
		if !bytes.HasPrefix(data, []byte("FORM")) || string(data[8:12]) != form {
			t.Errorf("%s: bad header %q", form, data[:12])
		}
		if got := binary.BigEndian.Uint32(data[4:]); got != uint32(len(data)-8) {
			t.Errorf("%s: FORM size = %d, want %d", form, got, len(data)-8)
		}
	}
}

func TestQuantizeClampsToDepth(t *testing.T) {
	if got := Quantize(2, 16); got != 32767 {
		t.Errorf("Quantize(2,16) = %d", got)
	}
	if got := Quantize(-2, 8); got != -128 {
		t.Errorf("Quantize(-2,8) = %d", got)
	}
}

func TestToneFramesLength(t *testing.T) {
	frames := ToneFrames(8000, 3, 250, 0.1)
	if len(frames) != 2000 || len(frames[0]) != 3 {
		t.Fatalf("got %d frames x %d channels", len(frames), len(frames[0]))
	}
}

func TestWriteCreatesParentDirsAndAValidWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "sound.wav")
	Write(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, Minimal()) {
		t.Error("Write did not write Minimal()")
	}
	if !bytes.HasPrefix(data, []byte("RIFF")) {
		t.Error("not a RIFF file")
	}
}
