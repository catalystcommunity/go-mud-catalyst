package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// LogLevel represents different logging levels
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
}

// DefaultLogger returns a logger with default configuration
func DefaultLogger() *Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewLogger creates a new logger with the specified level and output
func NewLogger(level LogLevel, output io.Writer) *Logger {
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: slogLevel,
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewJSONLogger creates a new logger with JSON output
func NewJSONLogger(level LogLevel, output io.Writer) *Logger {
	var slogLevel slog.Level
	switch level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: slogLevel,
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithContext returns a logger with the given context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: l.Logger.With(),
	}
}

// WithComponent returns a logger with a component field
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
	}
}

// WithClient returns a logger with client-specific fields
func (l *Logger) WithClient(clientID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("client_id", clientID),
	}
}

// WithConnection returns a logger with connection-specific fields
func (l *Logger) WithConnection(connType, remoteAddr string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			"connection_type", connType,
			"remote_addr", remoteAddr,
		),
	}
}

// WithMessage returns a logger with message-specific fields
func (l *Logger) WithMessage(messageType, messageID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			"message_type", messageType,
			"message_id", messageID,
		),
	}
}

// Global logger instance
var defaultLogger = DefaultLogger()

// GetDefaultLogger returns the default logger instance
func GetDefaultLogger() *Logger {
	return defaultLogger
}

// SetDefaultLogger sets the default logger instance
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// Convenience functions for default logger
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// Fatal logs an error message and exits the program
func Fatal(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}
