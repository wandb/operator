package logx

import (
	"io"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
)

type LogFormat string

const (
	TextFormat   LogFormat = "text"
	JsonFormat   LogFormat = "json"
	PrettyFormat LogFormat = "pretty"
)

// Options provides configuration for creating a logr or slog logger.
type Options struct {
	HandlerOptions *slog.HandlerOptions
	Overrides      map[string]slog.Level
	Output         io.Writer
	Format         LogFormat
}

var options *Options

func withDefaults(opts *Options) *Options {
	if opts == nil {
		opts = &Options{}
	}
	if opts.HandlerOptions == nil {
		opts.HandlerOptions = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}

	if opts.Overrides == nil {
		opts.Overrides = overrides
	}

	if opts.Output == nil {
		opts.Output = os.Stderr
	}

	if opts.Format == "" {
		opts.Format = "text"
	}

	return opts
}

// SetOptions sets the default options used by NewLogrLogger and NewSlogLogger.
func SetOptions(opts *Options) {
	options = withDefaults(opts)
}

// NewLogrLogger creates a logr.Logger backed by slog with per-name filtering.
func NewLogrLogger() logr.Logger {
	return logr.FromSlogHandler(NewHandler(options, ""))
}

// NewSlogLogger creates a slog.Logger backed by slog with per-name filtering.
func NewSlogLogger(loggerName string) *slog.Logger {
	return slog.New(NewHandler(options, loggerName).
		WithAttrs([]slog.Attr{slog.Any(LoggerKey, loggerName)}))
}
