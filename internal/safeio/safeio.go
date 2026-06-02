// Package safeio provides size-limited reads of attacker-influenced input.
//
// All entry points to claudio that read bytes from untrusted sources (hook
// stdin, soundpack JSON on disk, audio files referenced by soundpacks)
// must use ReadAllCapped to defend against OOM amplification by a
// malicious or accidentally-large input.
package safeio

import (
	"fmt"
	"io"
)

// Recommended caps. Exported so call sites are self-documenting.
const (
	// MaxHookPayloadBytes caps the hook JSON read from Claude Code stdin
	// (or its on-disk spool). 8 MiB is ~50x the realistic legitimate
	// payload (Bash tool_response.stdout for a verbose command) while
	// remaining orders of magnitude below OOM territory.
	MaxHookPayloadBytes int64 = 8 * 1024 * 1024 // 8 MiB

	// MaxSoundpackJSONBytes caps soundpack JSON files installed from
	// untrusted sources. 10 MiB combined with the 10K mappings cap gives
	// defense in depth — the JSON parser bails before the mappings
	// counter has to.
	MaxSoundpackJSONBytes int64 = 10 * 1024 * 1024 // 10 MiB

	// MaxAudioFileBytes caps WAV/AIFF/MP3 bytes referenced by a soundpack
	// mapping. UI sound effects are sub-second; 100 MiB is ~10 minutes of
	// 16-bit/44.1kHz/stereo WAV. The cap exists to prevent OOM on a
	// pathological file, not to be tight.
	MaxAudioFileBytes int64 = 100 * 1024 * 1024 // 100 MiB
)

// ReadAllCapped reads up to max+1 bytes from r. If the input exceeds max,
// it returns a descriptive error naming the kind of input. kind is
// human-readable (e.g. "hook payload", "soundpack JSON", "audio file")
// and appears in the error message.
//
// The +1 trick (read max+1, error if > max) detects exactly-at-cap inputs
// cleanly: an input of exactly max bytes succeeds, max+1 bytes fails.
func ReadAllCapped(r io.Reader, max int64, kind string) ([]byte, error) {
	limited := io.LimitReader(r, max+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", kind, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%s exceeds %d byte limit", kind, max)
	}
	return data, nil
}
