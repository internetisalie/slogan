package log

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync/atomic"
)

// rootDestination is one configured root destination. A nil writer means os.Stdout,
// the rule NewConsoleHandler applies to a nil writer.
type rootDestination struct {
	writer io.Writer
}

// rootDest is the process's root destination. It is nil until ConfigureRoot is called,
// which means os.Stdout.
var rootDest atomic.Pointer[rootDestination]

// ConfigureRoot sets the process's root destination to w and makes the root slog logger
// (slog.Default) write to it. A nil w means os.Stdout; it is not an error and not a reset.
//
// Call it once, at the start of a command, before configuration loads. Every logger built
// without WithWriter follows the root destination, those created before the call included,
// so no logger is rebuilt. The destination is os.Stdout until ConfigureRoot is called.
// A logger built with WithWriter keeps its explicit writer.
//
// ConfigureRoot creates no named logger and does not touch StandardLogger. It replaces the
// slog default logger, which also routes the standard library's log package to w.
func ConfigureRoot(w io.Writer) {
	rootDest.Store(&rootDestination{writer: w})

	ho := slog.HandlerOptions{
		Level:       LevelTrace,
		ReplaceAttr: NewTimestampAttrReplacer(nil, FormatTimestampMicro).ReplaceAttr,
	}
	slog.SetDefault(slog.New(NewConsoleHandler(&ho, w, "")))
}

// handlerOp is one WithAttrs or WithGroup call applied to a rootConsoleHandler: either
// a non-empty attrs slice or a non-empty group name.
type handlerOp struct {
	attrs []slog.Attr
	group string
}

// rootHandlerCache is the concrete handler built for one root destination.
type rootHandlerCache struct {
	dest    *rootDestination
	handler slog.Handler
}

// rootConsoleHandler is a console handler whose destination is the process's root
// destination. It holds what NewConsoleHandler needs, and the ordered WithAttrs and
// WithGroup calls applied to it, and builds the concrete handler on first use and again
// whenever ConfigureRoot has changed the destination.
//
// Level is answered from the held leveler alone: the root's level is never consulted.
// Terminal detection and LOG_FORMAT are evaluated by NewConsoleHandler for each destination,
// at first use and at each destination change, not when the logger is created.
type rootConsoleHandler struct {
	options slog.HandlerOptions
	format  string
	ops     []handlerOp
	cache   atomic.Pointer[rootHandlerCache]
}

func newRootConsoleHandler(ho *slog.HandlerOptions, format string) *rootConsoleHandler {
	return &rootConsoleHandler{options: *ho, format: format}
}

// Enabled reports whether level passes the held leveler.
func (h *rootConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.options.Level.Level() <= level
}

// Handle writes the record to the root destination as it is now.
func (h *rootConsoleHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.current().Handle(ctx, record)
}

// WithAttrs returns a handler that carries attrs after the attributes and groups already applied.
func (h *rootConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.with(handlerOp{attrs: slices.Clone(attrs)})
}

// WithGroup returns a handler that opens the named group after the operations already applied.
func (h *rootConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.with(handlerOp{group: name})
}

func (h *rootConsoleHandler) with(op handlerOp) *rootConsoleHandler {
	return &rootConsoleHandler{
		options: h.options,
		format:  h.format,
		ops:     append(slices.Clone(h.ops), op),
	}
}

// current returns the concrete handler for the present root destination, building it when
// none is cached or the destination has changed.
func (h *rootConsoleHandler) current() slog.Handler {
	dest := rootDest.Load()
	if cached := h.cache.Load(); cached != nil && cached.dest == dest {
		return cached.handler
	}

	handler := h.build(dest)
	h.cache.Store(&rootHandlerCache{dest: dest, handler: handler})
	return handler
}

func (h *rootConsoleHandler) build(dest *rootDestination) slog.Handler {
	var writer io.Writer
	if dest != nil {
		writer = dest.writer
	}

	ho := h.options
	handler := NewConsoleHandler(&ho, writer, h.format)
	for _, op := range h.ops {
		if op.group != "" {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(op.attrs)
		}
	}
	return handler
}
