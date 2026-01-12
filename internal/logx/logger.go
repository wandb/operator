package logx

import (
	"github.com/go-logr/logr"
)

const (
	DebugLevel = -1
	InfoLevel  = 0
	WarnLevel  = 1
	ErrorLevel = 2
)

// Logger wraps logr.Logger to provide explicit level methods.
type Logger struct {
	logr.Logger
}

// NewLogger creates a new Logger wrapping the given logr.Logger.
func NewLogger(l logr.Logger) Logger {
	return Logger{Logger: l}
}

// Debug logs a debug message (V(-1) level).
func (l Logger) Debug(msg string, keysAndValues ...any) {
	l.V(DebugLevel).Logger.Info(msg, keysAndValues...)
}

// Info logs an informational message (V(0) level).
func (l Logger) Info(msg string, keysAndValues ...any) {
	l.V(InfoLevel).Logger.Info(msg, keysAndValues...)
}

// Warn logs a warning message (Info level, logr doesn't distinguish warn).
func (l Logger) Warn(msg string, keysAndValues ...any) {
	l.V(WarnLevel).Logger.Info(msg, keysAndValues...)
}

// Error logs an error message.
func (l Logger) Error(err error, msg string, keysAndValues ...any) {
	l.V(ErrorLevel).Logger.Error(err, msg, keysAndValues...)
}

// WithValues returns a new Logger with additional key-value pairs.
func (l Logger) WithValues(keysAndValues ...any) Logger {
	return Logger{Logger: l.Logger.WithValues(keysAndValues...)}
}

// WithName returns a new Logger with the given name appended.
func (l Logger) WithName(name string) Logger {
	return Logger{Logger: l.Logger.WithName(name)}
}

// V returns a new Logger at the specified verbosity level.
func (l Logger) V(level int) Logger {
	return Logger{Logger: l.Logger.V(level)}
}
