package log

import (
	"context"
	"log/slog"
)

type contextKey string

const contextKeyLogAttrs = contextKey("LogAttrs")

func ContextWithLogAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	// merge with any existing attributes
	currentAttrs, _ := ctx.Value(contextKeyLogAttrs).([]slog.Attr)
	if currentAttrs != nil {
		attrs = MergeAttrs(currentAttrs, attrs)
	}
	return context.WithValue(ctx, contextKeyLogAttrs, attrs)
}

func logAttrsFromContext(ctx context.Context) []slog.Attr {
	v, _ := ctx.Value(contextKeyLogAttrs).([]slog.Attr)
	return v
}
