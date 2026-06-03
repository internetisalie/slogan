package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/samber/mo"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	slogmulti "github.com/samber/slog-multi"
)

// Log output format constants for console handlers.
const (
	// FormatLogFmt outputs structured logs in logfmt format (key=value pairs).
	FormatLogFmt = "logfmt"
	// FormatJson outputs structured logs as JSON objects.
	FormatJson = "json"
	// FormatHuman outputs logs in a readable multi-line human format.
	FormatHuman = "human"
	// FormatTint outputs logs with ANSI color formatting for terminals.
	FormatTint = "tint"
	// FormatDefault is the default output format (FormatLogFmt).
	FormatDefault = FormatLogFmt
	// FormatPlain outputs logs as plain text without structure.
	FormatPlain = "plain"
)

// TerminalFormat specifies the format to use for terminal output when auto-detection
// is enabled. If empty, defaults to FormatTint. Can be set via environment or configuration.
var TerminalFormat string

type environment interface {
	Getenv(key string) string
	Setenv(key, value string) error
}

type realEnv struct{}

func (realEnv) Getenv(key string) string       { return os.Getenv(key) }
func (realEnv) Setenv(key, value string) error { return os.Setenv(key, value) }

var env environment = realEnv{}

// NewConsoleHandler creates an slog.Handler that writes logs to a writer with the
// specified format. If writer is nil, uses os.Stdout. If format is empty, auto-detects
// based on whether the output is a terminal, respecting LOG_FORMAT environment variable
// and TerminalFormat preferences. Supported formats are FormatJson, FormatLogFmt,
// FormatTint, FormatHuman, and FormatPlain.
//
// The handler applies options like log level, attribute replacement, and source location
// information through the provided HandlerOptions.
func NewConsoleHandler(ho *slog.HandlerOptions, writer io.Writer, format string) slog.Handler {
	if writer == nil {
		writer = os.Stdout
	}

	if format == "" {
		format = FormatDefault

		isTerminal := false
		if f, ok := writer.(*os.File); ok {
			isTerminal = isatty.IsTerminal(f.Fd())
		}

		if isTerminal {
			format = mo.EmptyableToOption(TerminalFormat).OrElse(FormatTint)
		}

		if requestedFormat := env.Getenv("LOG_FORMAT"); requestedFormat != "" {
			requestedFormat = strings.ToLower(requestedFormat)

			switch requestedFormat {
			case FormatJson, FormatLogFmt, FormatHuman, FormatTint, FormatPlain:
				format = requestedFormat
			default:
				_ = env.Setenv("LOG_FORMAT", format)
				StandardLogger().Error(fmt.Sprintf(
					"Unknown log format %q.  Defaulting to %q",
					requestedFormat, format))
			}
		}
	}

	var console slog.Handler
	switch format {
	case FormatJson:
		console = slog.NewJSONHandler(writer, ho)
	case FormatLogFmt:
		console = slog.NewTextHandler(writer, ho)
	case FormatTint:
		noColor := true
		if f, ok := writer.(*os.File); ok {
			noColor = !isatty.IsTerminal(f.Fd())
		}

		console = tint.NewHandler(writer, &tint.Options{
			AddSource:   ho.AddSource,
			Level:       ho.Level,
			ReplaceAttr: ho.ReplaceAttr,
			TimeFormat:  "15:04:05.000000",
			NoColor:     noColor,
		})
	case FormatHuman:
		console = NewHumanHandler(writer, ho)
	case FormatPlain:
		console = NewPlainHandler(writer, ho)
	default:
		StandardLogger().Warn(fmt.Sprintf(
			"Unknown log format %q. Defaulting to %q",
			format, FormatDefault))
		console = slog.NewTextHandler(writer, ho)
	}
	return console
}

// RemoteProxyHandler is a handler that defers handler creation until the handler factory
// is registered. This enables decoupling logger setup from handler availability, allowing
// loggers to be created before handlers are configured. Attributes and groups are buffered
// until a real handler becomes available.
type RemoteProxyHandler struct {
	attrs  []slog.Attr // tree of saved attributes
	groups []string    // current group path names
}

// Enabled reports whether the handler will process records at the given level.
// If no handler factory is registered, returns false.
func (h *RemoteProxyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if n := h.leafHandler(); n != nil {
		return n.Enabled(ctx, level)
	}
	return false
}

func (h *RemoteProxyHandler) Handle(ctx context.Context, record slog.Record) error {
	if n := h.leafHandler(); n != nil {
		return n.Handle(ctx, record)
	}
	return nil
}

func (h *RemoteProxyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if n := h.leafHandler(); n != nil {
		return n.WithAttrs(attrs)
	}
	if len(attrs) == 0 {
		return h
	}

	return &RemoteProxyHandler{
		attrs:  SetAttrsAtPath(h.attrs, h.groups, attrs),
		groups: h.groups,
	}
}

func (h *RemoteProxyHandler) WithGroup(name string) slog.Handler {
	if n := h.leafHandler(); n != nil {
		return n.WithGroup(name)
	}

	if name == "" {
		return h
	}

	return &RemoteProxyHandler{
		attrs:  h.attrs,
		groups: AddGroup(h.groups, name),
	}
}

func (h *RemoteProxyHandler) leafHandler() slog.Handler {
	if registeredRemoteHandlerFactory == nil {
		return nil
	}

	result := registeredRemoteHandlerFactory()
	result = result.WithAttrs(h.attrs)
	for _, group := range h.groups {
		result = result.WithGroup(group)
	}
	return result
}

// RemoteHandlerFactory is a factory function that creates slog.Handler instances.
// Used to defer handler creation until it's needed.
type RemoteHandlerFactory func() slog.Handler

var registeredRemoteHandlerFactory RemoteHandlerFactory

// RegisterRemoteHandlerFactory registers the factory used to create handlers
// for RemoteProxyHandler instances. This must be called before loggers using
// RemoteProxyHandler will actually emit logs. It enables decoupled initialization
// of logging infrastructure.
func RegisterRemoteHandlerFactory(factory RemoteHandlerFactory) {
	registeredRemoteHandlerFactory = factory
}

// NewRemoteHandler creates a new RemoteProxyHandler that defers to a registered
// factory for actual handler creation. Useful for delaying handler setup until
// all initialization is complete. Use RegisterRemoteHandlerFactory to provide
// the handler factory.
func NewRemoteHandler() slog.Handler {
	return new(RemoteProxyHandler)
}

func walkError(err error, fn func(error)) {
	if err == nil {
		return
	}
	fn(err)
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		walkError(u.Unwrap(), fn)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			walkError(e, fn)
		}
	}
}



// NewErrorAttrsMiddleware creates a middleware that extracts and promotes error attributes
// from nested errors. When a record contains an "error" attribute with a complex error tree,
// this middleware flattens and hoists metadata (levels and attributes) from all errors
// in the chain to the top level, making error details more discoverable in logs.
func NewErrorAttrsMiddleware() slogmulti.Middleware {
	return slogmulti.NewHandleInlineMiddleware(func(ctx context.Context, record slog.Record, next func(context.Context, slog.Record) error) error {
		var err error
		var hasError bool
		record.Attrs(func(a slog.Attr) bool {
			if a.Key == ErrorKey {
				err, _ = a.Value.Any().(error)
				hasError = true
				return false
			}
			return true
		})

		if !hasError || err == nil {
			return next(ctx, record)
		}

		// Create a new record to avoid duplicate ErrorKey
		newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
		record.Attrs(func(a slog.Attr) bool {
			if a.Key != ErrorKey {
				newRecord.AddAttrs(a)
			}
			return true
		})

		// Recursively extract metadata from the error chain/tree
		walkError(err, func(e error) {
			// Promote level if any error in the chain has a higher level
			if l, ok := e.(interface{ Level() slog.Level }); ok {
				if level := l.Level(); level > newRecord.Level {
					newRecord.Level = level
				}
			}

			// Merge attributes from any error in the chain
			if a, ok := e.(interface{ LogAttrs() []slog.Attr }); ok {
				newRecord.AddAttrs(a.LogAttrs()...)
			}
		})

		// Add enriched error group
		backTraceBytes := BackTrace(err)
		newRecord.AddAttrs(slog.Attr{
			Key: ErrorKey,
			Value: slog.GroupValue(
				slog.String(
					ErrorStackKey,
					strings.TrimSpace(string(backTraceBytes)),
				),
				slog.String(
					ErrorMessageKey,
					err.Error(),
				),
			),
		})

		return next(ctx, newRecord)
	})
}
