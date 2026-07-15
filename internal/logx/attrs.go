package logx

import "log/slog"

func ErrAttr(err error) slog.Attr {
	return slog.Any("error", err)
}
