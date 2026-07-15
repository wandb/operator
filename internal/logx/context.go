package logx

import (
	"context"
	"log/slog"
)

type logKey struct{}

func WithSlog(ctx context.Context, logName string) (context.Context, *slog.Logger) {
	if log, ok := ctx.Value(logKey{}).(*slog.Logger); ok {
		if log.Handler().(*Handler).loggerName == logName {
			return ctx, log
		}
	}
	log := NewSlogLogger(logName)
	return context.WithValue(ctx, logKey{}, log), log
}

func GetSlog(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(logKey{}).(*slog.Logger); ok {
		return log
	}
	return NewSlogLogger("unknown")
}
