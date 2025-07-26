package main

import (
	"log/slog"
	"os"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("claudio starting", "version", "0.1.0")
	slog.Info("claudio initialized successfully")
}