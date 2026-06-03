package log

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextWithLogAttrs(t *testing.T) {
	ctx := context.Background()
	attrs := []slog.Attr{slog.String("a", "b")}

	ctx = ContextWithLogAttrs(ctx, attrs...)
	got := logAttrsFromContext(ctx)
	assert.Equal(t, attrs, got)

	// merge
	attrs2 := []slog.Attr{slog.Int("c", 1)}
	ctx = ContextWithLogAttrs(ctx, attrs2...)
	got = logAttrsFromContext(ctx)
	assert.Len(t, got, 2)
}

func TestContextWithOperation(t *testing.T) {
	ctx := context.Background()
	op := "test-operation"
	
	ctx = ContextWithOperation(ctx, op)
	got := OperationFromContext(ctx)
	assert.Equal(t, op, got)
	
	// Test missing
	assert.Empty(t, OperationFromContext(context.Background()))
}
