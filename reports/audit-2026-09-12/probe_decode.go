//go:build ignore

// Decode every tracked audio asset without opening an audio device.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"claudio.click/internal/audio/malgo"
)

func main() {
	listed, err := exec.Command("git", "ls-files").Output()
	if err != nil {
		panic(err)
	}
	registry := malgo.NewDefaultRegistry()
	var results []map[string]any
	failed := false
	for _, path := range strings.Split(strings.TrimSpace(string(listed)), "\n") {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".wav" && ext != ".mp3" {
			continue
		}
		entry := map[string]any{"path": path}
		file, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		data, err := registry.DecodeFile(context.Background(), path, file)
		_ = file.Close()
		if err != nil {
			entry["error"] = err.Error()
			failed = true
		} else {
			entry["pcm_bytes"] = len(data.Samples)
			entry["channels"] = data.Channels
			entry["sample_rate"] = data.SampleRate
			entry["pcm_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data.Samples))
		}
		results = append(results, entry)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		panic(err)
	}
	if failed {
		os.Exit(1)
	}
}
