package logger

import (
	"context"

	"github.com/rs/zerolog"
)

// contextKey is the key used to store logger in context
type contextKey struct{}

var loggerContextKey = contextKey{}

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext extracts a logger from the context
// If no logger is found, returns the global logger
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*Logger); ok {
		return logger
	}
	return Get()
}

// Ctx returns a zerolog context logger from the context
// This is useful for direct zerolog usage
func Ctx(ctx context.Context) *zerolog.Logger {
	logger := FromContext(ctx)
	return &logger.logger
}

// DebugCtx logs a debug message using context logger
func DebugCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Debug()
}

// InfoCtx logs an info message using context logger
func InfoCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Info()
}

// WarnCtx logs a warning message using context logger
func WarnCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Warn()
}

// ErrorCtx logs an error message using context logger
func ErrorCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Error()
}

// FatalCtx logs a fatal message and exits using context logger
func FatalCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Fatal()
}

// PanicCtx logs a panic message and panics using context logger
func PanicCtx(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Panic()
}