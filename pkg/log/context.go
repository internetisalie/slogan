package log

import (
	"context"
	"log/slog"
)

type contextKey string

const contextKeyLogAttrs = contextKey("LogAttrs")

func ContextWithOperation(ctx context.Context, op string) context.Context {
	return ContextWithLogAttrs(ctx, slog.String(OperationKey, op))
}

func OperationFromContext(ctx context.Context) string {
	attrs := logAttrsFromContext(ctx)
	val, ok := GetValueAtPath(attrs, OperationKey)
	if !ok {
		return ""
	}
	return val.String()
}

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
