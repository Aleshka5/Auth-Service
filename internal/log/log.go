package applog

import (
	"log/slog"
	"os"
)

func New() *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
