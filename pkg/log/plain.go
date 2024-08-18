package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

type errorHandler struct {
	err slog.Value
	slog.Handler
}

func (e errorHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Message += ": " + e.err.String()
	return e.Handler.Handle(ctx, record)
}

type PlainHandler struct {
	opts slog.HandlerOptions
	mu   *sync.Mutex
	out  io.Writer
}

func NewPlainHandler(out io.Writer, opts *slog.HandlerOptions) *PlainHandler {
	h := &PlainHandler{out: out, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *PlainHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *PlainHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *PlainHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if errorTextValue, ok := GetValueAtPath(attrs, ErrorKey, ErrorTextKey); ok {
		return errorHandler{
			err:     errorTextValue,
			Handler: h,
		}
	}
	return h
}

func (h *PlainHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprintln(h.out, r.Message)
	return err
}
