package logger

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog.Logger with additional functionality
type Logger struct {
	logger zerolog.Logger
}

// Config represents logger configuration
type Config struct {
	Level      string `yaml:"level" mapstructure:"level"`             // debug, info, warn, error
	Format     string `yaml:"format" mapstructure:"format"`           // json, console
	Output     string `yaml:"output" mapstructure:"output"`           // stdout, stderr, file path
	Timestamp  bool   `yaml:"timestamp" mapstructure:"timestamp"`     // include timestamp
	Caller     bool   `yaml:"caller" mapstructure:"caller"`           // include caller info
	PrettyMode bool   `yaml:"pretty_mode" mapstructure:"pretty_mode"` // enable pretty console output
}

// DefaultConfig returns default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "console",
		Output:     "stdout",
		Timestamp:  true,
		Caller:     false,
		PrettyMode: true,
	}
}

// globalLogger holds the global logger instance
var globalLogger *Logger

// Initialize sets up the global logger with the provided configuration
func Initialize(config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	// Set global log level
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output writer
	var output io.Writer
	switch strings.ToLower(config.Output) {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// File output
		if err := os.MkdirAll(filepath.Dir(config.Output), 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return err
		}
		output = file
	}

	// Create base logger
	var logger zerolog.Logger

	switch {
	case config.Format == "console" && config.PrettyMode:
		// Pretty console output with colors
		consoleWriter := zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}

		// Custom level formatting with emojis and colors
		consoleWriter.FormatLevel = func(i interface{}) string {
			var l string
			if ll, ok := i.(string); ok {
				switch ll {
				case "trace":
					l = "🔍 TRACE"
				case "debug":
					l = "🐛 DEBUG"
				case "info":
					l = "ℹ️  INFO"
				case "warn":
					l = "⚠️  WARN"
				case "error":
					l = "❌ ERROR"
				case "fatal":
					l = "💀 FATAL"
				case "panic":
					l = "🔥 PANIC"
				default:
					l = strings.ToUpper(ll)
				}
			}
			return l
		}

		// Custom message formatting
		consoleWriter.FormatMessage = func(i interface{}) string {
			if msg, ok := i.(string); ok {
				return msg
			}
			return ""
		}

		logger = zerolog.New(consoleWriter)
	case config.Format == "console":
		// Simple console output without colors
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    true,
		})
	default:
		// JSON output
		logger = zerolog.New(output)
	}

	// Add timestamp if enabled
	if config.Timestamp {
		logger = logger.With().Timestamp().Logger()
	}

	// Add caller info if enabled
	if config.Caller {
		logger = logger.With().Caller().Logger()
	}

	// Create wrapper
	globalLogger = &Logger{logger: logger}

	// Set as global zerolog logger
	log.Logger = logger

	return nil
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Initialize with defaults if not already initialized
		_ = Initialize(nil)
	}
	return globalLogger
}

// WithContext returns a logger with context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{logger: l.logger.With().Ctx(ctx).Logger()}
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{logger: l.logger.With().Interface(key, value).Logger()}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	logger := l.logger.With()
	for k, v := range fields {
		logger = logger.Interface(k, v)
	}
	return &Logger{logger: logger.Logger()}
}

// WithComponent adds a component field to the logger
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{logger: l.logger.With().Str("component", component).Logger()}
}

// WithError adds an error field to the logger
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{logger: l.logger.With().Err(err).Logger()}
}

// Debug logs a debug message
func (l *Logger) Debug() *zerolog.Event {
	return l.logger.Debug()
}

// Info logs an info message
func (l *Logger) Info() *zerolog.Event {
	return l.logger.Info()
}

// Warn logs a warning message
func (l *Logger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

// Error logs an error message
func (l *Logger) Error() *zerolog.Event {
	return l.logger.Error()
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal() *zerolog.Event {
	return l.logger.Fatal()
}

// Panic logs a panic message and panics
func (l *Logger) Panic() *zerolog.Event {
	return l.logger.Panic()
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() zerolog.Level {
	return l.logger.GetLevel()
}

// Global logging functions for convenience

// Debug logs a debug message using the global logger
func Debug() *zerolog.Event {
	return Get().Debug()
}

// Info logs an info message using the global logger
func Info() *zerolog.Event {
	return Get().Info()
}

// Warn logs a warning message using the global logger
func Warn() *zerolog.Event {
	return Get().Warn()
}

// Error logs an error message using the global logger
func Error() *zerolog.Event {
	return Get().Error()
}

// Fatal logs a fatal message and exits using the global logger
func Fatal() *zerolog.Event {
	return Get().Fatal()
}

// Panic logs a panic message and panics using the global logger
func Panic() *zerolog.Event {
	return Get().Panic()
}

// WithComponent returns a logger with a component field using the global logger
func WithComponent(component string) *Logger {
	return Get().WithComponent(component)
}

// WithError returns a logger with an error field using the global logger
func WithError(err error) *Logger {
	return Get().WithError(err)
}

// WithField returns a logger with a field using the global logger
func WithField(key string, value interface{}) *Logger {
	return Get().WithField(key, value)
}

// WithFields returns a logger with multiple fields using the global logger
func WithFields(fields map[string]interface{}) *Logger {
	return Get().WithFields(fields)
}
