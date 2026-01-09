package utils

import (
	"go.uber.org/zap/zapcore"
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
