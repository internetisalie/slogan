package log

import (
	"io"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/samber/lo"
	"github.com/samber/slog-multi"
)

// RootLoggerName is the name of the framework-level root logger instance.
const RootLoggerName = "slogan"

// Error attribute key names for structured error logging.
const (
	// ErrorKey is the attribute key for error values.
	ErrorKey = "error"
	// ErrorMessageKey is the attribute key for error message strings.
	ErrorMessageKey = "message"
	// ErrorStackKey is the attribute key for error stack traces.
	ErrorStackKey = "stack"

	// LoggerKey is the attribute key for logger names.
	LoggerKey = "logger"
	// OperationKey is the attribute key for operation identifiers.
	OperationKey = "operation"
)

// FormatTimestampMicro is the timestamp format for structured logs with microsecond precision.
// Format: "2006-01-02T15:04:05.000000Z07:00"
const FormatTimestampMicro = "2006-01-02T15:04:05.000000Z07:00"

// FormatTimestampHuman is the timestamp format for human-readable logs without date.
// Format: "15:04:05.000000"
const FormatTimestampHuman = "15:04:05.000000"

var (
	standardLogger     *slog.Logger
	standardLoggerOnce sync.Once
)

// StandardLogger returns the global framework-level logger singleton (named "slogan").
// Consumers should generally prefer NewLogger or NewPackageLogger for application
// logic to ensure logs are properly scoped and searchable. StandardLogger is
// intended for high-level lifecycle events or framework diagnostic logging.
func StandardLogger() *slog.Logger {
	standardLoggerOnce.Do(func() {
		standardLogger = NewLogger(RootLoggerName)
	})

	return standardLogger
}

// LoggingLogger returns a logger for use by logging handler implementations' internal logging
func LoggingLogger() *slog.Logger {
	handler := NewConsoleHandler(&slog.HandlerOptions{
		Level:       LevelTrace,
		ReplaceAttr: NewTimestampAttrReplacer(nil, FormatTimestampMicro).ReplaceAttr,
	}, nil, "")
	return slog.New(handler)
}

var regexpVersionSuffix = regexp.MustCompile(`^v\d+$`)

var packageLoggerPrefixes = []string{
	"code.internetisalie.net/",
}

// RegisterPackageLoggerPrefix registers a custom package prefix for logger name derivation.
// When NewPackageLogger is called, any packages starting with this prefix will have
// it stripped during logger name generation, allowing for shorter, more readable logger names.
//
// Example: RegisterPackageLoggerPrefix("mycompany.com/") will transform
// "mycompany.com/service/api" to "service.api".
func RegisterPackageLoggerPrefix(prefix string) {
	packageLoggerPrefixes = append(packageLoggerPrefixes, prefix)
}

// PackageLoggerName derives a logger name from the calling package. The skip parameter
// indicates how many stack frames to skip (skip=0 starts at the caller of PackageLoggerName).
// Returns a dot-separated package name with registered prefixes stripped and module
// version suffixes (v0, v1, etc.) removed.
//
// Example: For a caller in "mycompany.com/service/handlers", returns "service.handlers".
func PackageLoggerName(skip int) string {
	pc, _, _, _ := runtime.Caller(skip)
	longFunc := runtime.FuncForPC(pc).Name()
	longFuncSuffixIndex := strings.LastIndex(longFunc, ".")
	longPackage := longFunc[:longFuncSuffixIndex]

	// trim the first known prefixes
	shortPackage := longPackage
	for _, packagePrefix := range packageLoggerPrefixes {
		if strings.HasPrefix(shortPackage, packagePrefix) {
			shortPackage = strings.TrimPrefix(longPackage, packagePrefix)
			break
		}
	}

	// trim any module version suffix
	packageParts := strings.Split(shortPackage, "/")
	lastPackagePart := len(packageParts) - 1
	if regexpVersionSuffix.MatchString(packageParts[lastPackagePart]) {
		packageParts = packageParts[:lastPackagePart]
	}

	return strings.Join(packageParts, ".")
}

type consoleOptions struct {
	writer io.Writer
	format string
}

type loggerOptions struct {
	console    *consoleOptions
	attrs      []slog.Attr
	timeFormat string
	sanitizers []Sanitizer
	leveler    slog.Leveler
	handlers   []slog.Handler
	middleware []slogmulti.Middleware
	addSource  bool
}

// LoggerOption is a functional option for configuring logger behavior.
// LoggerOptions are passed to NewLoggerWithOpts to customize logger creation.
type LoggerOption func(*loggerOptions)

// WithWriter sets the output writer for console logging.
// If not specified, defaults to os.Stdout.
func WithWriter(w io.Writer) LoggerOption {
	return func(o *loggerOptions) {
		if o.console == nil {
			o.console = &consoleOptions{}
		}
		o.console.writer = w
	}
}

// WithAttrs appends structured attributes to all log records from this logger.
// Attributes are merged with any attributes already set and new ones take precedence.
func WithAttrs(attrs ...slog.Attr) LoggerOption {
	return func(o *loggerOptions) {
		o.attrs = append(o.attrs, attrs...)
	}
}

// WithTimeFormat sets the timestamp format for log output.
// Format should be a valid time layout string (e.g., time.RFC3339).
// Defaults to FormatTimestampMicro.
func WithTimeFormat(f string) LoggerOption {
	return func(o *loggerOptions) {
		o.timeFormat = f
	}
}

// WithSanitizers adds data sanitizers to redact or mask sensitive information
// from log records. Multiple sanitizers are combined and applied in order.
func WithSanitizers(s ...Sanitizer) LoggerOption {
	return func(o *loggerOptions) {
		o.sanitizers = append(o.sanitizers, s...)
	}
}

// WithFormat sets the output format for console logging.
// Supported formats: "json", "logfmt", "tint", "human", "plain".
// If not specified, automatically detects terminal vs non-terminal output.
func WithFormat(f string) LoggerOption {
	return func(o *loggerOptions) {
		if o.console == nil {
			o.console = &consoleOptions{}
		}
		o.console.format = f
	}
}

// WithLeveler sets a custom leveler to control log levels dynamically.
// If not specified, defaults to the leveler for the logger's name registered with SetLogLevel.
func WithLeveler(l slog.Leveler) LoggerOption {
	return func(o *loggerOptions) {
		o.leveler = l
	}
}

// WithHandlers appends custom slog.Handler implementations to the logger.
// Handlers are called in addition to the console handler and allow custom
// log record processing and persistence.
func WithHandlers(handlers ...slog.Handler) LoggerOption {
	return func(o *loggerOptions) {
		o.handlers = append(o.handlers, handlers...)
	}
}

// WithMiddleware appends middleware that intercepts all log records.
// Middleware can modify, filter, or augment records before they reach handlers.
func WithMiddleware(middleware ...slogmulti.Middleware) LoggerOption {
	return func(o *loggerOptions) {
		o.middleware = append(o.middleware, middleware...)
	}
}

// WithAddSource enables source code location information (file and line number)
// in log records. This adds a small performance overhead.
func WithAddSource(addSource bool) LoggerOption {
	return func(o *loggerOptions) {
		o.addSource = addSource
	}
}

// WithoutConsole disables the default console handler, allowing manual handler setup
// via WithHandlers or RemoteProxyHandler.
func WithoutConsole() LoggerOption {
	return func(o *loggerOptions) {
		o.console = nil
	}
}

// NewLogger creates a new named logger with default configuration.
// The logger name is used as a structured attribute and for level management.
// Additional attributes can be passed and will be attached to all log records.
// This is a convenience wrapper around NewLoggerWithOpts with console output enabled.
func NewLogger(name string, attrs ...slog.Attr) *slog.Logger {
	return NewLoggerWithOpts(name, WithAttrs(attrs...), WithHandlers(NewRemoteHandler()))
}

// NewLoggerWithOpts creates a new named logger with customizable configuration
// via functional options. The logger integrates console output, structured attributes,
// custom handlers, middleware, and attribute replacement for sanitization and
// formatting. All options can be combined to build complex logging pipelines.
//
// Example:
//
//	logger := NewLoggerWithOpts("myapp.handler",
//		WithFormat("json"),
//		WithSanitizers(NewCredentialsSanitizer()),
//		WithMiddleware(NewTraceIDMiddleware()),
//	)
func NewLoggerWithOpts(name string, opts ...LoggerOption) *slog.Logger {
	o := &loggerOptions{
		console:    &consoleOptions{},
		timeFormat: FormatTimestampMicro,
	}
	for _, opt := range opts {
		opt(o)
	}

	var replacer AttrReplacer

	// Inject our replacer middleware
	var s Sanitizer
	if len(o.sanitizers) > 0 {
		s = MultiSanitizer(o.sanitizers...)
	}
	replacer = NewSanitizerAttrReplacer(s, replacer)
	replacer = NewTimestampAttrReplacer(replacer, o.timeFormat)

	leveler := o.leveler
	if leveler == nil {
		leveler = GetLoggerLeveler(name)
	}

	ho := slog.HandlerOptions{
		Level:       leveler,
		ReplaceAttr: replacer.ReplaceAttr,
		AddSource:   o.addSource,
	}

	var allHandlers []slog.Handler
	if o.console != nil {
		allHandlers = append(allHandlers, NewConsoleHandler(&ho, o.console.writer, o.console.format))
	}
	allHandlers = append(allHandlers, o.handlers...)

	// inject our handler middleware
	pipe := slogmulti.Pipe(NewErrorAttrsMiddleware())
	for _, m := range o.middleware {
		pipe = pipe.Pipe(m)
	}

	handler := pipe.Handler(slogmulti.Fanout(
		allHandlers...,
	))

	logger := slog.New(handler)

	// Apply logger name
	logger = logger.With(slog.Attr{
		Key:   LoggerKey,
		Value: slog.StringValue(name),
	})

	// Apply passed attributes
	if len(o.attrs) > 0 {
		logger = logger.With(lo.ToAnySlice(o.attrs)...)
	}

	return logger
}

// NewPackageLogger creates a logger automatically named after the calling package.
// The logger name is derived from the caller's package path with registered prefixes
// stripped. Additional attributes can be passed and will be attached to all log records.
// Typical usage in application code:
//
//	var log = NewPackageLogger()
func NewPackageLogger(attrs ...slog.Attr) *slog.Logger {
	name := PackageLoggerName(2)
	return NewLogger(name, attrs...)
}
