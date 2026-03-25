package main

import (
	"log/slog"
	"os"

	"github.com/leshchenko/pdf-extract/internal/config"
	"github.com/leshchenko/pdf-extract/internal/httpserver"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Error("mkdir uploads", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Error("mkdir outputs", "err", err)
		os.Exit(1)
	}

	if err := httpserver.ListenAndServe(cfg, log); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
