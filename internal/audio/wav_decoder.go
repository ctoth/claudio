package audio

import (
	"io"
	"log/slog"
	"strings"

	"github.com/gen2brain/malgo"
	"github.com/youpy/go-wav"
)

// WavDecoder handles WAV audio format decoding
type WavDecoder struct{}

// NewWavDecoder creates a new WAV decoder instance
func NewWavDecoder() *WavDecoder {
	slog.Debug("creating new WAV decoder instance")
	return &WavDecoder{}
}

// Decode reads WAV audio data from reader and returns decoded PCM data
func (d *WavDecoder) Decode(reader io.Reader) (*AudioData, error) {
	slog.Debug("starting WAV decode operation")
	
	// youpy/go-wav needs a ReadSeeker, so we need to read all data first
	data, err := io.ReadAll(reader)
	if err != nil {
		slog.Error("failed to read WAV data", "error", err)
		return nil, ErrReadFailure
	}
	
	if len(data) == 0 {
		slog.Error("empty WAV data")
		return nil, ErrInvalidData
	}
	
	// Create a ReadSeeker from the data
	seekReader := strings.NewReader(string(data))
	wavReader := wav.NewReader(seekReader)
	
	format, err := wavReader.Format()
	if err != nil {
		slog.Error("failed to read WAV format", "error", err)
		return nil, ErrInvalidData
	}
	
	slog.Debug("WAV format detected",
		"sample_rate", format.SampleRate,
		"channels", format.NumChannels,
		"bits_per_sample", format.BitsPerSample)
	
	// Validate format parameters
	if format.NumChannels == 0 || format.SampleRate == 0 {
		slog.Error("invalid WAV format parameters",
			"channels", format.NumChannels,
			"sample_rate", format.SampleRate)
		return nil, ErrInvalidData
	}
	
	// Convert bit depth to malgo format
	var malgoFormat malgo.FormatType
	switch format.BitsPerSample {
	case 16:
		malgoFormat = malgo.FormatS16
		slog.Debug("using 16-bit signed format")
	case 24:
		malgoFormat = malgo.FormatS24
		slog.Debug("using 24-bit signed format")
	case 32:
		malgoFormat = malgo.FormatS32
		slog.Debug("using 32-bit signed format")
	default:
		slog.Error("unsupported bit depth", "bits", format.BitsPerSample)
		return nil, ErrUnsupportedFormat
	}
	
	// Read all audio samples into memory
	slog.Debug("reading WAV audio samples")
	var allSamples []wav.Sample
	
	for {
		samples, err := wavReader.ReadSamples()
		if err != nil {
			if err == io.EOF {
				slog.Debug("reached end of WAV file", "total_samples", len(allSamples))
				break
			}
			slog.Error("failed to read WAV samples", "error", err)
			return nil, ErrReadFailure
		}
		
		if len(samples) == 0 {
			break
		}
		
		allSamples = append(allSamples, samples...)
		
		if len(allSamples)%16384 == 0 { // Log every 16K samples
			slog.Debug("reading WAV data", "samples_read", len(allSamples))
		}
	}
	
	if len(allSamples) == 0 {
		slog.Error("no audio data found in WAV file")
		return nil, ErrInvalidData
	}
	
	// Convert samples to raw bytes based on bit depth
	var rawBytes []byte
	
	for _, sample := range allSamples {
		// Process all channels in the sample (interleaved)
		for ch := 0; ch < int(format.NumChannels); ch++ {
			var val int
			if ch < len(sample.Values) {
				val = sample.Values[ch]
			} else {
				// If channel data is missing, use silence
				val = 0
			}
			
			switch format.BitsPerSample {
			case 16:
				rawBytes = append(rawBytes, byte(val), byte(val>>8))
			case 24:
				rawBytes = append(rawBytes, byte(val), byte(val>>8), byte(val>>16))
			case 32:
				rawBytes = append(rawBytes, byte(val), byte(val>>8), byte(val>>16), byte(val>>24))
			}
		}
	}
	
	audioData := &AudioData{
		Samples:    rawBytes,
		Channels:   uint32(format.NumChannels),
		SampleRate: uint32(format.SampleRate),
		Format:     malgoFormat,
	}
	
	slog.Info("WAV decode completed successfully",
		"total_bytes", len(rawBytes),
		"total_samples", len(allSamples),
		"channels", audioData.Channels,
		"sample_rate", audioData.SampleRate,
		"format", malgoFormat,
		"duration_estimate_ms", (len(allSamples)*1000)/int(audioData.SampleRate))
	
	return audioData, nil
}

// CanDecode checks if this decoder can handle the given filename
func (d *WavDecoder) CanDecode(filename string) bool {
	lower := strings.ToLower(filename)
	canDecode := strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".wave")
	
	slog.Debug("WAV decoder file check",
		"filename", filename,
		"can_decode", canDecode)
	
	return canDecode
}

// FormatName returns the name of the format this decoder handles
func (d *WavDecoder) FormatName() string {
	return "WAV"
}