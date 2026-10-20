package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestForwardSectionLoadsFromTheConfigFile(t *testing.T) {
	path := writeConfig(t, `{"forward":{"url":"http://laptop.lan:19190","token":"s3cret"}}`)
	cfg, err := NewConfigManager().LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Forward == nil || cfg.Forward.URL != "http://laptop.lan:19190" || cfg.Forward.Token != "s3cret" {
		t.Errorf("Forward = %+v", cfg.Forward)
	}
	if got := cfg.ForwardURL(); got != "http://laptop.lan:19190" {
		t.Errorf("ForwardURL() = %q", got)
	}
}

// Forwarding is off unless a URL is given; that is the default.
func TestForwardIsOffByDefault(t *testing.T) {
	cfg := NewConfigManager().GetDefaultConfig()
	if cfg.ForwardURL() != "" {
		t.Errorf("default ForwardURL() = %q, want none", cfg.ForwardURL())
	}
	if (&Config{Forward: &ForwardConfig{Token: "t"}}).ForwardURL() != "" {
		t.Error("a token without a URL turned forwarding on")
	}
}

func TestForwardURLMustBeAnHTTPAddress(t *testing.T) {
	for _, url := range []string{"laptop.lan:19190", "ftp://laptop.lan", "http://", "://"} {
		path := writeConfig(t, `{"forward":{"url":"`+url+`"}}`)
		_, err := NewConfigManager().LoadFromFile(path)
		if err == nil {
			t.Errorf("url %q: loaded without error", url)
		} else if !strings.Contains(err.Error(), "forward") {
			t.Errorf("url %q: error %q does not name the forward url", url, err)
		}
	}
	for _, url := range []string{"http://127.0.0.1:19190/events", "https://relay.example.com/topics/my-sounds?priority=low"} {
		path := writeConfig(t, `{"forward":{"url":"`+url+`"}}`)
		if _, err := NewConfigManager().LoadFromFile(path); err != nil {
			t.Errorf("url %q: %v", url, err)
		}
	}
}

func TestForwardEnvironmentOverrides(t *testing.T) {
	t.Setenv("CLAUDIO_FORWARD_URL", "http://laptop.lan:19190")
	t.Setenv("CLAUDIO_FORWARD_TOKEN", "s3cret")
	manager := NewConfigManager()

	fromNothing := manager.ApplyEnvironmentOverrides(manager.GetDefaultConfig())
	if fromNothing.Forward == nil || fromNothing.Forward.URL != "http://laptop.lan:19190" || fromNothing.Forward.Token != "s3cret" {
		t.Errorf("Forward = %+v", fromNothing.Forward)
	}

	original := manager.GetDefaultConfig()
	original.Forward = &ForwardConfig{URL: "http://old:1", Token: "old"}
	result := manager.ApplyEnvironmentOverrides(original)
	if result.Forward.URL != "http://laptop.lan:19190" || result.Forward.Token != "s3cret" {
		t.Errorf("override not applied: %+v", result.Forward)
	}
	if original.Forward.URL != "http://old:1" || original.Forward.Token != "old" {
		t.Errorf("caller's Forward was mutated: %+v", original.Forward)
	}
}

// A bad environment value is ignored, like every other override: it must
// not stop a hook or turn forwarding on.
func TestBadForwardURLInTheEnvironmentIsIgnored(t *testing.T) {
	t.Setenv("CLAUDIO_FORWARD_URL", "not a url")
	manager := NewConfigManager()
	result := manager.ApplyEnvironmentOverrides(manager.GetDefaultConfig())
	if result.ForwardURL() != "" {
		t.Errorf("ForwardURL() = %q, want none", result.ForwardURL())
	}
	if err := manager.ValidateConfig(result); err != nil {
		t.Errorf("config no longer validates: %v", err)
	}
}

// The override log names the variable; a secret's value stays out of it.
func TestForwardTokenIsNotLogged(t *testing.T) {
	t.Setenv("CLAUDIO_FORWARD_URL", "http://laptop.lan:19190")
	t.Setenv("CLAUDIO_FORWARD_TOKEN", "hunter2-TOKEN")
	var log bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	manager := NewConfigManager()
	manager.ApplyEnvironmentOverrides(manager.GetDefaultConfig())

	if strings.Contains(log.String(), "hunter2-TOKEN") {
		t.Errorf("token in log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "CLAUDIO_FORWARD_TOKEN") {
		t.Errorf("log does not name the override:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "http://laptop.lan:19190") {
		t.Errorf("a non-secret override's value is missing from the log:\n%s", log.String())
	}
}
