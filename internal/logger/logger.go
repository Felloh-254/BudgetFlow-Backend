package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func Init() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	Logger = slog.New(handler)
}

// WithComponent returns a logger with a component name attached.
//
// Example:
//
//	log := logger.WithComponent("service.summary")
//
// Every log produced by `log` will contain:
//
//	component=service.summary
func WithComponent(component string) *slog.Logger {
	return Logger.With("component", component)
}
