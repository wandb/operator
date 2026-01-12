package logx

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

type logKey struct{}

func IntoContext(ctx context.Context, logName string) (context.Context, Logger) {
	log := NewLogger(ctrl.Log.WithName(logName))
	return context.WithValue(ctx, logKey{}, log), log
}

func FromContext(ctx context.Context) Logger {
	if log, ok := ctx.Value(logKey{}).(Logger); ok {
		return log
	}
	return NewLogger(ctrl.LoggerFrom(ctx))
}
