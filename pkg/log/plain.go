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

// PlainHandler outputs log records as plain text (message only), without structure.
// This is useful for scenarios where only the message text is desired (e.g., writing
// to the console of a containerized application) or for compatibility with systems
// that don't support structured logging. PlainHandler is thread-safe.
type PlainHandler struct {
	opts slog.HandlerOptions
	mu   *sync.Mutex
	out  io.Writer
}

// NewPlainHandler creates a new PlainHandler that writes plain text log messages to the given writer.
// If opts is nil, a default HandlerOptions with LevelInfo is used.
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
	if errorTextValue, ok := GetValueAtPath(attrs, ErrorKey, ErrorMessageKey); ok {
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
