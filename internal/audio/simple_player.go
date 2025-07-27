package audio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// SimplePlayer provides a simple interface for playing audio files
type SimplePlayer struct {
	streamingPlayer *StreamingPlayer
	volume          float32
}

// NewSimplePlayer creates a new simple audio player
func NewSimplePlayer() *SimplePlayer {
	return &SimplePlayer{
		streamingPlayer: NewStreamingPlayer(),
		volume:          1.0,
	}
}

// PlayFile plays an audio file with the given volume
func (p *SimplePlayer) PlayFile(filePath string, volume float32) error {
	slog.Debug("playing audio file", "path", filePath, "volume", volume)
	
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}
	
	// Set volume
	p.streamingPlayer.SetVolume(volume)
	
	// Determine decoder based on file extension
	var audioData *StreamingAudioData
	lower := strings.ToLower(filePath)
	
	if strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".wave") {
		decoder := NewStreamingWavDecoder()
		audioData, err = decoder.DecodeStreaming(data)
	} else if strings.HasSuffix(lower, ".mp3") {
		decoder := NewStreamingMp3Decoder()
		audioData, err = decoder.DecodeStreaming(data)
	} else {
		return fmt.Errorf("unsupported audio format: %s", filePath)
	}
	
	if err != nil {
		return fmt.Errorf("failed to decode audio: %w", err)
	}
	
	// Calculate approximate duration
	// For WAV: data size / (channels * bytesPerSample * sampleRate)
	// This is approximate since we don't know the exact audio data size
	bytesPerSample := 2 // Assuming 16-bit
	approxSamples := len(data) / (int(audioData.Channels) * bytesPerSample)
	duration := time.Duration(approxSamples) * time.Second / time.Duration(audioData.SampleRate)
	
	// Add some buffer time
	duration = duration + 500*time.Millisecond
	
	slog.Debug("estimated playback duration", "duration_ms", duration.Milliseconds())
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	// Play the sound
	return p.streamingPlayer.PlayStreamingSound(ctx, audioData)
}

// Close closes the player
func (p *SimplePlayer) Close() error {
	return p.streamingPlayer.Close()
}