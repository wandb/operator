package logx

import (
	"context"
	"log/slog"
)

const LoggerKey = "LOGGER"

func NewHandler(opts *Options, loggerName string) *Handler {
	opts = withDefaults(opts)

	var baseHandler slog.Handler
	switch opts.Format {
	case JsonFormat:
		baseHandler = slog.NewJSONHandler(opts.Output, opts.HandlerOptions)
	case PrettyFormat:
		baseHandler = BuildPrettyHandler(opts)
	default:
		baseHandler = slog.NewTextHandler(opts.Output, opts.HandlerOptions)
	}

	defaultLevel := slog.LevelInfo
	if opts.HandlerOptions.Level != nil {
		defaultLevel = opts.HandlerOptions.Level.Level()
	}

	return &Handler{
		handler:      baseHandler,
		overrides:    opts.Overrides,
		loggerName:   loggerName,
		defaultLevel: defaultLevel,
	}
}

// Handler wraps a slog.Handler and filters log records by logger name.
// It tracks the logger name through WithGroup() calls and applies
// per-logger-name level overrides.
type Handler struct {
	handler      slog.Handler
	overrides    map[string]slog.Level
	loggerName   string
	defaultLevel slog.Level
}

// Enabled reports whether the handler is enabled for the given level.
// It checks if the current logger name has an override level, and if so,
// compares against that. Otherwise, it uses the default level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.loggerName != "" {
		if minLevel, ok := h.overrides[h.loggerName]; ok {
			return level >= minLevel
		}
	}
	return level >= h.defaultLevel
}

// Handle passes the record through to the wrapped handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		handler:      h.handler.WithAttrs(attrs),
		overrides:    h.overrides,
		loggerName:   h.loggerName,
		defaultLevel: h.defaultLevel,
	}
}

// WithGroup returns a new handler with the given group name added to the logger name.
func (h *Handler) WithGroup(name string) slog.Handler {
	return h.handler.WithGroup(name)
}
