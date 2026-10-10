package config

import (
	"errors"
	"fmt"
	"net/url"
)

// ForwardConfig sends each hook event to a `claudio listen` on another
// machine instead of playing it on this one.
type ForwardConfig struct {
	URL   string `json:"url"`             // Listener base URL (http://host:port); empty = play here
	Token string `json:"token,omitempty"` // Token the listener requires
}

// ForwardURL is the listener events are sent to, or "" to play them here.
func (c *Config) ForwardURL() string {
	if c.Forward == nil {
		return ""
	}
	return c.Forward.URL
}

// validateForwardURL checks that raw is a listener base URL: http or https,
// a host, no query or fragment.
func validateForwardURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("forward url is not a URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("forward url %q must start with http:// or https://", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("forward url %q has no host", raw)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("forward url %q must not have a query or fragment", raw)
	}
	return nil
}

// forwardOf returns c's forward section, creating it when absent.
func forwardOf(c *Config) *ForwardConfig {
	if c.Forward == nil {
		c.Forward = &ForwardConfig{}
	}
	return c.Forward
}
