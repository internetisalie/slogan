package log

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPlainHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewPlainHandler(buf, nil)

	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "hello message", 0)
	err := h.Handle(context.Background(), r)
	assert.NoError(t, err)
	assert.Equal(t, "hello message\n", buf.String())

	// Test error enrichment in plain handler
	buf.Reset()
	// Use Wrap with an option to get a decorator that implements LogValue and returns a group
	errDecorated := Wrap(fmt.Errorf("root cause"), WithErrorLevel(LevelInfo))
	errAttr := slog.Any(ErrorKey, errDecorated)
	h2 := h.WithAttrs([]slog.Attr{errAttr})
	err = h2.Handle(context.Background(), r)
	assert.NoError(t, err)
	// Plain handler appends error to message
	assert.Contains(t, buf.String(), "hello message: root cause")
}

func TestPlainHandler_Enabled(t *testing.T) {
	h := NewPlainHandler(nil, &slog.HandlerOptions{Level: slog.LevelWarn})
	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
}

func TestPlainHandler_Groups(t *testing.T) {
	h := NewPlainHandler(nil, nil)
	assert.NotNil(t, h.WithGroup("foo"))
}
