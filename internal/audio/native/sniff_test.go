package native

import "testing"

func TestSniffFormat(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		data       []byte
		want       audioFormat
	}{
		{"WAV magic despite .mp3", "fake.mp3", []byte("RIFF\x24\x00\x00\x00WAVEfmt "), formatWAV},
		{"MP3 frame sync despite .wav", "fake.wav", []byte{0xFF, 0xFB, 0x90, 0x00}, formatMP3},
		{"MP3 ID3 tag", "x.bin", []byte("ID3\x04\x00\x00\x00\x00\x00\x00"), formatMP3},
		{"AIFF magic", "x.bin", []byte("FORM\x00\x00\x00\x10AIFFCOMM"), formatAIFF},
		{"AIFC magic", "x.bin", []byte("FORM\x00\x00\x00\x10AIFCFVER"), formatAIFF},
		{"RIFF but not WAVE", "x.bin", []byte("RIFF\x24\x00\x00\x00AVI LIST"), formatUnknown},
		{"WavPack is not WAV", "track.wvp", append([]byte("wvpk"), make([]byte, 28)...), formatUnknown},
		{"MPEG sync with reserved layer", "x.bin", []byte{0xFF, 0xE0, 0x00, 0x00}, formatUnknown},
		{"garbage falls back to .wav", "test.wav", []byte("not audio data"), formatWAV},
		{"garbage falls back to .WAVE", "TEST.WAVE", []byte("not audio data"), formatWAV},
		{"garbage falls back to .mp3", "x.mp3", []byte("not audio data"), formatMP3},
		{"garbage falls back to .aif", "x.aif", []byte("not audio data"), formatAIFF},
		{"garbage falls back to .aifc", "x.aifc", []byte("not audio data"), formatAIFF},
		{"empty falls back to extension", "x.aiff", nil, formatAIFF},
		{"garbage without extension", "x", []byte("not audio data"), formatUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffFormat(tc.data, tc.file); got != tc.want {
				t.Errorf("sniffFormat(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}
