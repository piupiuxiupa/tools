package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger with additional convenience methods
type Logger struct {
	zerolog.Logger
}

// New creates a new Logger with the specified level and format
func New(level string, format ...string) *Logger {
	outputFormat := "console"
	if len(format) > 0 {
		outputFormat = format[0]
	}

	var output io.Writer = os.Stderr

	// Pretty print for development
	if outputFormat == "console" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			FormatLevel: func(i interface{}) string {
				return strings.ToUpper(i.(string))
			},
		}
	}

	zl := zerolog.New(output).With().Timestamp().Logger()

	// Set log level
	lvl := parseLevel(level)
	zl = zl.Level(lvl)

	return &Logger{zl}
}

// parseLevel parses string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.Logger.Debug().Msg(msg)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.Logger.Debug().Msgf(format, v...)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.Logger.Info().Msg(msg)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, v ...interface{}) {
	l.Logger.Info().Msgf(format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.Logger.Warn().Msg(msg)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Logger.Warn().Msgf(format, v...)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.Logger.Error().Msg(msg)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Logger.Error().Msgf(format, v...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.Logger.Fatal().Msg(msg)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Logger.Fatal().Msgf(format, v...)
}

// WithField returns a logger with a field added to the context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.Logger.With().Interface(key, value).Logger()}
}

// WithFields returns a logger with multiple fields added to the context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.Logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{ctx.Logger()}
}

// WithError returns a logger with an error field added
func (l *Logger) WithError(err error) *Logger {
	return &Logger{l.Logger.With().Err(err).Logger()}
}

// SetLevel dynamically changes the log level
func (l *Logger) SetLevel(level string) {
	lvl := parseLevel(level)
	l.Logger = l.Logger.Level(lvl)
}
