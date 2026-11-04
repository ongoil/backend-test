// Package logger creates the application's structured logger.
package logger

import (
	"log/slog"
	"os"
)

// New returns a logger writing to stdout in "json" or "text" format,
// and sets it as the default slog logger.
func New(format string) *slog.Logger {
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
