// Package logger provides a single structured logger for the process,
// built on the standard library's log/slog so there is no extra
// dependency for something this low-stakes.
package logger

import (
	"log/slog"
	"os"
)

// New builds a JSON structured logger writing to stdout. Deployments that
// want a different sink (files, a log shipper) can redirect stdout at the
// process level; there is no need for configurability inside the app.
func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}
