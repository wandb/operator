package logx

import (
	"log/slog"

	"github.com/m-mizutani/masq"
)

// redact masks any struct field tagged `masq:"secret"` in a logged attribute,
// so sensitive connection values never reach the operator log. It is applied to
// every handler format via chainReplaceAttr.
var redact = masq.New(masq.WithTag("secret"))

// chainReplaceAttr composes slog ReplaceAttr functions left to right, skipping
// nil entries. It lets us layer masq redaction on top of a handler's existing
// ReplaceAttr without either clobbering the other.
func chainReplaceAttr(fns ...func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		for _, fn := range fns {
			if fn != nil {
				a = fn(groups, a)
			}
		}
		return a
	}
}
