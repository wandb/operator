package utils

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

/////////////////////////////////////
// Names, Defaults

const (
	Webhook = "webhook"
	Worker  = "worker"
)

var overrides = map[string]zapcore.Level{
	Webhook: zapcore.ErrorLevel,
	Worker:  zapcore.DebugLevel,
}

func BuildNameFilterLogger(root zapcore.Level) logr.Logger {
	opts := ctrlzap.Options{
		Level: root,
		ZapOpts: []zap.Option{
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return &NameFilteredCore{Core: core, overrides: overrides}
			}),
		},
	}
	return ctrlzap.New(ctrlzap.UseFlagOptions(&opts))
}

/*
func main() {
	overrides := map[string]zapcore.Level{
		"webhook": zapcore.ErrorLevel,
		"worker":  zapcore.DebugLevel,
	}

	opts := ctrlzap.Options{
		ZapOpts: []zap.Option{
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return &NameFilteredCore{Core: core, overrides: overrides}
			}),
		},
	}
	ctrl.SetLogger(ctrlzap.New(ctrlzap.UseFlagOptions(&opts)))
}
*/
