package config

import (
	"errors"
	"fmt"
	"net/url"
)

// ForwardConfig sends each hook event somewhere else instead of playing it
// on this machine: to a `claudio listen`, or to a relay one reads from.
type ForwardConfig struct {
	URL   string `json:"url"`             // Address each event is POSTed to, used as given; empty = play here
	Token string `json:"token,omitempty"` // Bearer token sent with each event
}

// ForwardURL is the address events are sent to, or "" to play them here.
func (c *Config) ForwardURL() string {
	if c.Forward == nil {
		return ""
	}
	return c.Forward.URL
}

// validateForwardURL checks that raw is an http or https address with a
// host. Its path and query are the receiver's business.
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
	return nil
}

// forwardOf returns c's forward section, creating it when absent.
func forwardOf(c *Config) *ForwardConfig {
	if c.Forward == nil {
		c.Forward = &ForwardConfig{}
	}
	return c.Forward
}
