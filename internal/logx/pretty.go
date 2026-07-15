package logx

import (
	"log/slog"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

func BuildPrettyHandler(opts *Options) slog.Handler {
	return tint.NewHandler(opts.Output, &tint.Options{
		TimeFormat: time.TimeOnly + ".000000",
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if strings.EqualFold(a.Key, LoggerKey) {
				return slog.String(strings.ToUpper(a.Key), a.Value.String())
			}
			return a
		},
	})
}
