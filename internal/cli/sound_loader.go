package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"claudio/internal/audio"
)

// FileNotFoundError represents a sound file not found error
type FileNotFoundError struct {
	SoundPath string
	Paths     []string
}

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("sound file not found: %s (searched in: %s)", e.SoundPath, strings.Join(e.Paths, ", "))
}

// IsFileNotFoundError checks if an error is a FileNotFoundError
func IsFileNotFoundError(err error) bool {
	_, ok := err.(*FileNotFoundError)
	return ok
}

// SoundLoader handles loading sound files from soundpack directories
type SoundLoader struct {
	soundpackPaths []string
	registry       *audio.DecoderRegistry
}

// NewSoundLoader creates a new sound loader with the given soundpack paths
func NewSoundLoader(soundpackPaths []string) *SoundLoader {
	slog.Debug("creating new sound loader", "soundpack_paths", soundpackPaths)
	
	loader := &SoundLoader{
		soundpackPaths: soundpackPaths,
		registry:       audio.NewDefaultRegistry(), // Use default registry with WAV/MP3 support
	}
	
	slog.Info("sound loader created", "total_soundpack_paths", len(soundpackPaths))
	return loader
}

// LoadSound loads a sound file from the configured soundpack paths
func (sl *SoundLoader) LoadSound(soundPath string) (*audio.AudioData, error) {
	if soundPath == "" {
		err := fmt.Errorf("sound path cannot be empty")
		slog.Error("load sound failed", "error", err)
		return nil, err
	}

	slog.Debug("attempting to load sound", "sound_path", soundPath, "search_paths", len(sl.soundpackPaths))

	var searchedPaths []string
	
	// Try each soundpack path in order
	for i, basePath := range sl.soundpackPaths {
		fullPath := filepath.Join(basePath, soundPath)
		searchedPaths = append(searchedPaths, fullPath)
		
		slog.Debug("checking sound file", "attempt", i+1, "full_path", fullPath)
		
		// Check if file exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			slog.Debug("sound file not found", "path", fullPath)
			continue
		} else if err != nil {
			slog.Error("error checking sound file", "path", fullPath, "error", err)
			continue
		}
		
		// File exists, try to load it
		slog.Debug("sound file found, attempting to decode", "path", fullPath)
		
		audioData, err := sl.loadAndDecodeFile(fullPath)
		if err != nil {
			slog.Error("failed to decode sound file", "path", fullPath, "error", err)
			return nil, fmt.Errorf("failed to decode sound file %s: %w", fullPath, err)
		}
		
		slog.Info("sound loaded successfully", 
			"sound_path", soundPath,
			"full_path", fullPath,
			"channels", audioData.Channels,
			"sample_rate", audioData.SampleRate,
			"samples", len(audioData.Samples))
		
		return audioData, nil
	}
	
	// File not found in any path
	err := &FileNotFoundError{
		SoundPath: soundPath,
		Paths:     searchedPaths,
	}
	
	slog.Warn("sound file not found in any soundpack path", 
		"sound_path", soundPath,
		"searched_paths", searchedPaths)
	
	return nil, err
}

// loadAndDecodeFile loads and decodes a specific audio file
func (sl *SoundLoader) loadAndDecodeFile(filePath string) (*audio.AudioData, error) {
	slog.Debug("opening sound file for decoding", "file_path", filePath)
	
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("failed to open sound file", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get appropriate decoder
	decoder := sl.registry.DetectFormat(filePath)
	if decoder == nil {
		err := fmt.Errorf("no decoder found for file format")
		slog.Error("no decoder found for sound file", "file_path", filePath, "error", err)
		return nil, err
	}
	
	slog.Debug("decoding sound file", "file_path", filePath, "decoder", decoder.FormatName())
	
	// Decode the audio data
	audioData, err := decoder.Decode(file)
	if err != nil {
		slog.Error("decoding failed", "file_path", filePath, "decoder", decoder.FormatName(), "error", err)
		return nil, fmt.Errorf("decoding failed: %w", err)
	}
	
	slog.Debug("sound file decoded successfully", 
		"file_path", filePath,
		"format", decoder.FormatName(),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"data_size", len(audioData.Samples))
	
	return audioData, nil
}

// GetSoundpackPaths returns the configured soundpack paths
func (sl *SoundLoader) GetSoundpackPaths() []string {
	return sl.soundpackPaths
}

// AddSoundpackPath adds a new soundpack path to the search list
func (sl *SoundLoader) AddSoundpackPath(path string) {
	slog.Debug("adding soundpack path", "path", path)
	sl.soundpackPaths = append(sl.soundpackPaths, path)
	slog.Info("soundpack path added", "path", path, "total_paths", len(sl.soundpackPaths))
}

// ResolveSoundPath resolves a sound path to its full file path without loading the audio data
func (sl *SoundLoader) ResolveSoundPath(soundPath string) (string, error) {
	if soundPath == "" {
		err := fmt.Errorf("sound path cannot be empty")
		slog.Error("resolve sound path failed", "error", err)
		return "", err
	}

	slog.Debug("attempting to resolve sound path", "sound_path", soundPath, "search_paths", len(sl.soundpackPaths))

	var searchedPaths []string
	
	// Try each soundpack path in order
	for i, basePath := range sl.soundpackPaths {
		fullPath := filepath.Join(basePath, soundPath)
		searchedPaths = append(searchedPaths, fullPath)
		
		slog.Debug("checking sound file", "attempt", i+1, "full_path", fullPath)
		
		// Check if file exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			slog.Debug("sound file not found", "path", fullPath)
			continue
		} else if err != nil {
			slog.Error("error checking sound file", "path", fullPath, "error", err)
			continue
		}
		
		// File exists
		slog.Info("sound path resolved successfully", 
			"sound_path", soundPath,
			"full_path", fullPath)
		
		return fullPath, nil
	}
	
	// File not found in any path
	err := &FileNotFoundError{
		SoundPath: soundPath,
		Paths:     searchedPaths,
	}
	
	slog.Warn("sound file not found in any soundpack path", 
		"sound_path", soundPath,
		"searched_paths", searchedPaths)
	
	return "", err
}