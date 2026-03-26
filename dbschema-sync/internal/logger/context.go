package logger

import "context"

type contextKey string

const loggerKey contextKey = "logger"

// WithLogger adds logger to context
func WithLogger(ctx context.Context, log *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// GetLogger retrieves logger from context
func GetLogger(ctx context.Context) *Logger {
	if log, ok := ctx.Value(loggerKey).(*Logger); ok {
		return log
	}
	return New("info")
}
