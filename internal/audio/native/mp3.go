package native

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
)

// bytesReadSeekCloser lets go-mp3 seek the in-memory file, which it needs
// to compute the exact length up front.
type bytesReadSeekCloser struct{ *bytes.Reader }

func (bytesReadSeekCloser) Close() error { return nil }

func decodeMP3(data []byte) (sound, error) {
	s, format, err := mp3.Decode(bytesReadSeekCloser{bytes.NewReader(data)})
	if err != nil {
		return sound{}, fmt.Errorf("%w: %w", ErrInvalidData, err)
	}
	frames := s.Len()
	if int64(frames)*4 > MaxDecodedPCMBytes {
		return sound{}, fmt.Errorf("%w: decoded MP3 exceeds %d bytes", ErrReadFailure, MaxDecodedPCMBytes)
	}
	trim, ok := mp3InfoTrim(data)
	if !ok {
		return sound{Streamer: s, rate: format.SampleRate, frames: frames}, nil
	}
	if trim.start+trim.end >= frames {
		slog.Warn("ignoring MP3 info header that trims the whole stream",
			"frames", frames, "trim_start", trim.start, "trim_end", trim.end)
		return sound{Streamer: s, rate: format.SampleRate, frames: frames}, nil
	}
	slog.Debug("trimming MP3 info header frame and encoder delay",
		"frames", frames, "trim_start", trim.start, "trim_end", trim.end)
	frames -= trim.start + trim.end
	return sound{
		Streamer: &window{Streamer: s, skip: trim.start, remaining: frames},
		rate:     format.SampleRate,
		frames:   frames,
	}, nil
}

// mp3DecoderDelay is the delay of a Layer III decoder, in frames. Encoders
// declare only their own delay; the audio starts this much later still.
const mp3DecoderDelay = 529

// mp3Trim is what an MP3's info header says is not audio, in frames to drop
// from each end of the decoded stream.
type mp3Trim struct{ start, end int }

// mp3InfoTrim reads the Xing/Info header that encoders put in the first
// frame of a file. go-mp3 decodes that frame as a frame of silence, and
// leaves in the encoder delay and end padding that the header declares, so
// a sound starts late by all three. The frame is always dropped; the delay
// and padding are dropped when the header comes from LAME or libav, which
// fill those fields in.
func mp3InfoTrim(data []byte) (mp3Trim, bool) {
	header := mp3FirstFrame(data)
	if len(header) < 4 {
		return mp3Trim{}, false
	}
	mpeg1 := header[1]>>3&3 == 3
	mono := header[3]>>6 == 3
	sideInfo, frame := 17, 576
	switch {
	case mpeg1 && mono:
		sideInfo, frame = 17, 1152
	case mpeg1:
		sideInfo, frame = 32, 1152
	case mono:
		sideInfo = 9
	}
	tag := 4 + sideInfo
	if header[1]&1 == 0 { // CRC follows the header
		tag += 2
	}
	if len(header) < tag+8 {
		return mp3Trim{}, false
	}
	if name := string(header[tag : tag+4]); name != "Xing" && name != "Info" {
		return mp3Trim{}, false
	}
	trim := mp3Trim{start: frame}

	// Optional fields, each present when its flag is set: frame count,
	// byte count, seek table, quality.
	flags := binary.BigEndian.Uint32(header[tag+4:])
	ext := tag + 8
	for _, field := range []struct {
		flag uint32
		size int
	}{{1, 4}, {2, 4}, {4, 100}, {8, 4}} {
		if flags&field.flag != 0 {
			ext += field.size
		}
	}
	// The LAME extension: a 9-byte encoder name, then 12 bytes of other
	// fields, then the delay and padding packed as two 12-bit numbers.
	if len(header) < ext+24 {
		return trim, true
	}
	switch string(header[ext : ext+4]) {
	case "LAME", "Lavc", "Lavf":
	default:
		return trim, true
	}
	packed := header[ext+21 : ext+24]
	delay := int(packed[0])<<4 | int(packed[1])>>4
	padding := int(packed[1]&0x0F)<<8 | int(packed[2])
	if delay == 0 && padding == 0 {
		return trim, true
	}
	trim.start += delay + mp3DecoderDelay
	// The padding is counted from the encoder's side, so the decoder delay
	// has already eaten into it.
	trim.end = max(padding-mp3DecoderDelay, 0)
	return trim, true
}

// mp3FirstFrame returns data from its first Layer III frame header on,
// skipping ID3v2 tags and anything else in front of it.
func mp3FirstFrame(data []byte) []byte {
	for len(data) >= 10 && string(data[:3]) == "ID3" {
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 | int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		size += 10
		if data[5]&0x10 != 0 { // footer
			size += 10
		}
		if size > len(data) {
			return nil
		}
		data = data[size:]
	}
	for i := 0; i+4 <= len(data); i++ {
		h := data[i:]
		if h[0] == 0xFF && h[1]&0xE0 == 0xE0 && // sync
			h[1]>>3&3 != 1 && // version is not reserved
			h[1]>>1&3 == 1 && // Layer III
			h[2]>>4 != 0x0F && // bitrate is valid
			h[2]>>2&3 != 3 { // sample rate is valid
			return h
		}
	}
	return nil
}

// window streams frames [skip, skip+remaining) of a streamer. The skipped
// frames are decoded and thrown away, so what follows is exactly what a
// straight decode gives.
type window struct {
	beep.Streamer
	skip, remaining int
}

func (w *window) Stream(dst [][2]float64) (int, bool) {
	var scratch [512][2]float64
	for w.skip > 0 {
		n, ok := w.Streamer.Stream(scratch[:min(len(scratch), w.skip)])
		if !ok {
			return 0, false
		}
		w.skip -= n
	}
	if w.remaining == 0 {
		return 0, false
	}
	n, ok := w.Streamer.Stream(dst[:min(len(dst), w.remaining)])
	w.remaining -= n
	return n, ok
}
