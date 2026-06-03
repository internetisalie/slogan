package log

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHumanHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	ho := &slog.HandlerOptions{Level: slog.LevelInfo}
	h := NewHumanHandler(buf, ho)
	
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("foo", "bar"), slog.Group("g", slog.Int("a", 1)))
	
	err := h.Handle(context.Background(), r)
	assert.NoError(t, err)
	
	output := buf.String()
	assert.Contains(t, output, "INF")
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "foo:")
	assert.Contains(t, output, "bar")
	assert.Contains(t, output, "g:")
	assert.Contains(t, output, "a:")
	assert.Contains(t, output, "1")
}

func TestHumanHandler_Enabled(t *testing.T) {
	h := NewHumanHandler(nil, &slog.HandlerOptions{Level: slog.LevelWarn})
	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
}

func TestHumanHandler_With(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewHumanHandler(buf, nil)
	
	h2 := h.WithAttrs([]slog.Attr{slog.String("attr", "val")})
	h3 := h2.WithGroup("group").WithAttrs([]slog.Attr{slog.Int("inner", 1)})
	
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "msg", 0)
	h3.Handle(context.Background(), r)
	
	output := buf.String()
	assert.Contains(t, output, "attr:")
	assert.Contains(t, output, "val")
	assert.Contains(t, output, "group:")
	assert.Contains(t, output, "inner:")
	assert.Contains(t, output, "1")
}

func TestHumanHandler_LevelColors(t *testing.T) {
	levels := []slog.Level{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError}
	for _, l := range levels {
		buf := &bytes.Buffer{}
		h := NewHumanHandler(buf, nil)
		r := slog.NewRecord(time.Time{}, l, "msg", 0)
		h.Handle(context.Background(), r)
		assert.NotEmpty(t, buf.String())
	}
}
