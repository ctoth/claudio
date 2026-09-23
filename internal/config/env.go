package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

// envVar binds one environment variable to the field of T it overrides.
// apply parses the (non-empty) raw value and writes it into target; an
// error means the value was rejected and target is left untouched.
type envVar[T any] struct {
	name  string
	apply func(target *T, raw string) error
}

// applyEnvVars applies every set, non-empty variable in table to target.
// Rejected values are logged and ignored, so a bad env var never stops a hook.
func applyEnvVars[T any](target *T, table []envVar[T]) {
	for _, v := range table {
		raw := os.Getenv(v.name)
		if raw == "" {
			continue
		}
		if err := v.apply(target, raw); err != nil {
			slog.Warn("invalid environment variable; ignoring it", "name", v.name, "value", raw, "error", err)
			continue
		}
		slog.Debug("applied environment override", "name", v.name, "value", raw)
	}
}

// stringVar sets the string field chosen by field.
func stringVar[T any](name string, field func(*T) *string) envVar[T] {
	return envVar[T]{name: name, apply: func(t *T, raw string) error {
		*field(t) = raw
		return nil
	}}
}

// boolVar sets the bool field chosen by field; the value follows
// strconv.ParseBool ("1"/"0", "true"/"false", ...).
func boolVar[T any](name string, field func(*T) *bool) envVar[T] {
	return envVar[T]{name: name, apply: func(t *T, raw string) error {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		*field(t) = b
		return nil
	}}
}

// configEnvVars are the top-level Config overrides. Sound tracking has its
// own table (soundTrackingEnvVars) because it is also applied standalone.
var configEnvVars = []envVar[Config]{
	{name: "CLAUDIO_VOLUME", apply: func(c *Config, raw string) error {
		vol, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		c.Volume = &vol
		return nil
	}},
	stringVar("CLAUDIO_SOUNDPACK", func(c *Config) *string { return &c.DefaultSoundpack }),
	boolVar("CLAUDIO_ENABLED", func(c *Config) *bool { return &c.Enabled }),
	stringVar("CLAUDIO_LOG_LEVEL", func(c *Config) *string { return &c.LogLevel }),
	{name: "CLAUDIO_AUDIO_BACKEND", apply: func(c *Config, raw string) error {
		backend := migrateLegacyAudioBackend(raw, "CLAUDIO_AUDIO_BACKEND")
		if !isSupportedAudioBackend(backend) {
			return fmt.Errorf("unsupported audio backend %q", backend)
		}
		c.AudioBackend = backend
		return nil
	}},
	// Opt-out switch so test environments can disable the lumberjack file
	// handle that would otherwise block t.TempDir() cleanup on Windows.
	boolVar("CLAUDIO_FILE_LOGGING", func(c *Config) *bool {
		if c.FileLogging == nil {
			c.FileLogging = &FileLoggingConfig{}
		}
		return &c.FileLogging.Enabled
	}),
}

// soundTrackingEnvVars are the SoundTrackingConfig overrides.
var soundTrackingEnvVars = []envVar[SoundTrackingConfig]{
	boolVar("CLAUDIO_SOUND_TRACKING", func(s *SoundTrackingConfig) *bool { return &s.Enabled }),
	stringVar("CLAUDIO_SOUND_TRACKING_DB", func(s *SoundTrackingConfig) *string { return &s.DatabasePath }),
}

// EnvVarNames lists every production environment variable claudio reads
// as a config override, in table order.
func EnvVarNames() []string {
	names := make([]string, 0, len(configEnvVars)+len(soundTrackingEnvVars))
	for _, v := range configEnvVars {
		names = append(names, v.name)
	}
	for _, v := range soundTrackingEnvVars {
		names = append(names, v.name)
	}
	return names
}
