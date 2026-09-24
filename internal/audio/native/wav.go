package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/wav"
)

const (
	wavFormatPCM        = 1
	wavFormatFloat      = 3
	wavFormatExtensible = 0xFFFE
)

// wavInfo is the part of a WAV header needed to validate and route it.
type wavInfo struct {
	format, channels, blockAlign, bits int
	rate                               int
	data                               []byte // the data chunk body
}

// parseWAV walks the RIFF chunks. It exists because beep's decoder trusts
// the header (a zero or short block alignment panics it) and does not read
// 32-bit integer or float samples.
func parseWAV(data []byte) (wavInfo, error) {
	var info wavInfo
	var haveFmt, haveData bool
	for off := 12; off+8 <= len(data) && !haveData; {
		id := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4:]))
		body := data[off+8:]
		if size > len(body) {
			return info, fmt.Errorf("%w: %q chunk claims %d bytes, %d remain", ErrInvalidData, id, size, len(body))
		}
		body = body[:size]
		switch id {
		case "fmt ":
			if size < 16 {
				return info, fmt.Errorf("%w: short fmt chunk", ErrInvalidData)
			}
			info.format = int(binary.LittleEndian.Uint16(body[0:]))
			info.channels = int(binary.LittleEndian.Uint16(body[2:]))
			info.rate = int(binary.LittleEndian.Uint32(body[4:]))
			info.blockAlign = int(binary.LittleEndian.Uint16(body[12:]))
			info.bits = int(binary.LittleEndian.Uint16(body[14:]))
			if info.format == wavFormatExtensible && size >= 26 {
				// The sub-format GUID starts with the plain format tag.
				info.format = int(binary.LittleEndian.Uint16(body[24:]))
			}
			haveFmt = true
		case "data":
			info.data = body
			haveData = true
		}
		off += 8 + size + size%2
	}
	if !haveFmt || !haveData {
		return info, fmt.Errorf("%w: missing fmt or data chunk", ErrInvalidData)
	}
	if info.channels < 1 || info.rate < 1 || info.bits%8 != 0 || info.blockAlign != info.channels*info.bits/8 {
		return info, fmt.Errorf("%w: inconsistent frame layout (%d channels, %d-bit, block %d, %d Hz)",
			ErrInvalidData, info.channels, info.bits, info.blockAlign, info.rate)
	}
	// Channel layouts beyond stereo are not folded for WAV; beep would
	// silently drop all but the first two.
	if info.channels > 2 {
		return info, fmt.Errorf("%w: %d-channel WAV", ErrUnsupportedFormat, info.channels)
	}
	return info, nil
}

// decodeWAV plays 8/16/24-bit PCM through beep's decoder. beep has no
// 32-bit integer or float support, so those two layouts use a minimal
// in-package reader over the data chunk.
func decodeWAV(data []byte) (sound, error) {
	info, err := parseWAV(data)
	if err != nil {
		return sound{}, err
	}
	frames := len(info.data) / info.blockAlign
	switch {
	case info.format == wavFormatPCM && info.bits <= 24:
		s, format, err := wav.Decode(bytes.NewReader(data))
		if err != nil {
			return sound{}, fmt.Errorf("%w: %w", ErrInvalidData, err)
		}
		return sound{Streamer: s, rate: format.SampleRate, frames: frames}, nil
	case info.format == wavFormatPCM && info.bits == 32:
		return wav32(info, frames, func(u uint32) float64 { return float64(int32(u)) / (1 << 31) }), nil
	case info.format == wavFormatFloat && info.bits == 32:
		return wav32(info, frames, func(u uint32) float64 { return clip(float64(math.Float32frombits(u))) }), nil
	}
	return sound{}, fmt.Errorf("%w: WAV format %d, %d-bit", ErrUnsupportedFormat, info.format, info.bits)
}

func wav32(info wavInfo, frames int, decode func(uint32) float64) sound {
	pcm := info.data
	return sound{
		Streamer: &interleaved{
			channels: info.channels,
			frames:   frames,
			sample:   func(i int) float64 { return decode(binary.LittleEndian.Uint32(pcm[i*4:])) },
		},
		rate:   beep.SampleRate(info.rate),
		frames: frames,
	}
}
