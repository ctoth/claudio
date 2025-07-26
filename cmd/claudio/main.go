package main

import (
	"log/slog"
	"os"

	"claudio/internal/audio"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("claudio starting", "version", "0.1.0")

	// Test audio context initialization
	ctx, err := audio.NewContext()
	if err != nil {
		slog.Error("failed to initialize audio", "error", err)
		os.Exit(1)
	}
	defer ctx.Close()

	slog.Info("claudio initialized successfully", "audio_context_valid", ctx.IsValid())
}