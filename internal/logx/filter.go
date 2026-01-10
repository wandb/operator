package logx

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type NameFilteredCore struct {
	zapcore.Core
	overrides map[string]zapcore.Level
}

func (c *NameFilteredCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if minLevel, ok := c.overrides[ent.LoggerName]; ok {
		if ent.Level >= minLevel {
			return c.Core.Check(ent, ce)
		}
		return ce
	}
	return c.Core.Check(ent, ce)
}

func WithFilter(opts ctrlzap.Options) logr.Logger {
	opts.ZapOpts = append(
		opts.ZapOpts,
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return &NameFilteredCore{Core: core, overrides: overrides}
		}),
	)
	return ctrlzap.New(ctrlzap.UseFlagOptions(&opts))
}
