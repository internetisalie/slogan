package log

import (
	"context"
	"log/slog"
)

type contextKey string

const contextKeyLogAttrs = contextKey("LogAttrs")

// ContextWithOperation returns a new context with an operation identifier attached.
// This identifier is automatically included in all log records created from the context,
// allowing logs from different operations to be correlated. Typically used for request IDs,
// transaction IDs, or other operation-scoping values.
func ContextWithOperation(ctx context.Context, op string) context.Context {
	return ContextWithLogAttrs(ctx, slog.String(OperationKey, op))
}

// OperationFromContext retrieves the operation identifier from the context if present.
// Returns an empty string if no operation has been set. Used with ContextWithOperation.
func OperationFromContext(ctx context.Context) string {
	attrs := logAttrsFromContext(ctx)
	val, ok := GetValueAtPath(attrs, OperationKey)
	if !ok {
		return ""
	}
	return val.String()
}

// ContextWithLogAttrs returns a new context with log attributes attached.
// These attributes are automatically included in all log records created from the context.
// If the context already has attributes, the new attributes are merged (with new ones taking precedence).
// Use this to propagate request-scoped attributes like user ID, tenant ID, or correlation IDs through a call chain.
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
