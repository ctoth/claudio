//go:build !cgo

package audio

import (
	"context"
	"errors"
	"io"
)

var errCGORequired = errors.New(`Claudio requires CGO support for audio functionality.

This error occurs when trying to use claudio built without CGO enabled.

To fix this issue:
1. Ensure CGO_ENABLED=1 (this is the default for native builds)
2. Install a C compiler:
   - Linux: sudo apt-get install build-essential
   - macOS: xcode-select --install
   - Windows: Install MinGW or Visual Studio Build Tools
3. Then run: go install claudio.click/cmd/claudio

For more information, see: https://pkg.go.dev/cmd/cgo`)

// BackendFactory creates AudioBackend instances based on configuration
type BackendFactory interface {
	CreateBackend(backendType string) (AudioBackend, error)
	GetAvailableBackends() []string
	GetPreferredBackend() string
	ValidateBackend(backendType string) error
}

// Stub backend factory
type stubBackendFactory struct{}

func NewBackendFactory() BackendFactory {
	return &stubBackendFactory{}
}

func (f *stubBackendFactory) CreateBackend(backendType string) (AudioBackend, error) {
	return nil, errCGORequired
}

func (f *stubBackendFactory) GetAvailableBackends() []string {
	return []string{}
}

func (f *stubBackendFactory) GetPreferredBackend() string {
	return ""
}

func (f *stubBackendFactory) ValidateBackend(backendType string) error {
	return errCGORequired
}

// Stub audio player
type AudioPlayer struct{}

func NewAudioPlayer() *AudioPlayer {
	return &AudioPlayer{}
}

func (p *AudioPlayer) IsPlaying() bool {
	return false
}

func (p *AudioPlayer) GetVolume() float32 {
	return 0
}

func (p *AudioPlayer) SetVolume(volume float32) error {
	return errCGORequired
}

func (p *AudioPlayer) IsSoundLoaded(soundID string) bool {
	return false
}

func (p *AudioPlayer) PreloadSound(soundID string, audioData *AudioData) error {
	return errCGORequired
}

func (p *AudioPlayer) UnloadSound(soundID string) error {
	return errCGORequired
}

func (p *AudioPlayer) PlaySound(soundID string) error {
	return errCGORequired
}

func (p *AudioPlayer) PlaySoundWithContext(ctx context.Context, soundID string) error {
	return errCGORequired
}

func (p *AudioPlayer) Stop() error {
	return errCGORequired
}

func (p *AudioPlayer) StopAll() error {
	return errCGORequired
}

func (p *AudioPlayer) Close() error {
	return errCGORequired
}

// Stub decoder registry
func NewDecoderRegistry() *DecoderRegistry {
	return &DecoderRegistry{}
}

func NewDefaultRegistry() *DecoderRegistry {
	return NewDecoderRegistry()
}

type DecoderRegistry struct{}

func (r *DecoderRegistry) RegisterDecoder(fileType string, decoder Decoder) {
	// No-op
}

func (r *DecoderRegistry) GetDecoder(fileType string) (Decoder, error) {
	return nil, errCGORequired
}

func (r *DecoderRegistry) GetSupportedTypes() []string {
	return []string{}
}

func (r *DecoderRegistry) DecodeAudio(reader io.Reader, filename string) (*AudioData, error) {
	return nil, errCGORequired
}

func (r *DecoderRegistry) DetectFormat(filename string) Decoder {
	return nil
}

func (r *DecoderRegistry) DetectFormatWithContent(filename string, reader io.Reader) Decoder {
	return nil
}

// Stub types
type AudioData struct {
	Samples    []byte
	SampleRate int
	Channels   int
	Format     int
}

type Decoder interface {
	CanDecode(filename string) bool
	Decode(reader io.Reader) (*AudioData, error)
	FormatName() string
}

// Stub decoder implementation
type stubDecoder struct{}

func (d *stubDecoder) CanDecode(filename string) bool {
	return false
}

func (d *stubDecoder) Decode(reader io.Reader) (*AudioData, error) {
	return nil, errCGORequired
}

func (d *stubDecoder) FormatName() string {
	return ""
}