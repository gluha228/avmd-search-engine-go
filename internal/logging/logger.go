package logging

import (
	"avmd-search-engine-go/internal/config"
	"log/slog"
	"os"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LoggingLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		AddSource: cfg.LoggingAddSource,
		Level:     level,
	}
	if cfg.UseJsonLogs {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
