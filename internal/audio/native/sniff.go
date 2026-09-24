package native

import (
	"bytes"
	"path/filepath"
	"strings"
)

// audioFormat is a container format the native backend can decode.
type audioFormat int

const (
	formatUnknown audioFormat = iota
	formatWAV
	formatMP3
	formatAIFF
)

func (f audioFormat) String() string {
	switch f {
	case formatWAV:
		return "WAV"
	case formatMP3:
		return "MP3"
	case formatAIFF:
		return "AIFF"
	default:
		return "unknown"
	}
}

// sniffFormat identifies data by its magic bytes and falls back to the file
// extension only when the content is not recognised. Content wins because
// soundpacks do ship mislabelled files.
func sniffFormat(data []byte, filename string) audioFormat {
	switch {
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return formatWAV
	case len(data) >= 12 && string(data[:4]) == "FORM" &&
		(string(data[8:12]) == "AIFF" || string(data[8:12]) == "AIFC"):
		return formatAIFF
	case bytes.HasPrefix(data, []byte("ID3")):
		return formatMP3
	case len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0 && data[1]&0x06 != 0:
		// MPEG audio frame sync with a defined layer.
		return formatMP3
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".wav", ".wave":
		return formatWAV
	case ".mp3", ".mpeg":
		return formatMP3
	case ".aif", ".aiff", ".aifc":
		return formatAIFF
	}
	return formatUnknown
}
