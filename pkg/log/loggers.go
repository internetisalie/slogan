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

const RootLoggerName = "slogan"

const (
	ErrorKey        = "error"
	ErrorMessageKey = "message"
	ErrorStackKey   = "stack"

	LoggerKey    = "logger"
	OperationKey = "operation"
)

const (
	FormatTimestampMicro = "2006-01-02T15:04:05.000000Z07:00"
	FormatTimestampHuman = "15:04:05.000000"
)

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

func RegisterPackageLoggerPrefix(prefix string) {
	packageLoggerPrefixes = append(packageLoggerPrefixes, prefix)
}

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

type LoggerOption func(*loggerOptions)

func WithWriter(w io.Writer) LoggerOption {
	return func(o *loggerOptions) {
		if o.console == nil {
			o.console = &consoleOptions{}
		}
		o.console.writer = w
	}
}

func WithAttrs(attrs ...slog.Attr) LoggerOption {
	return func(o *loggerOptions) {
		o.attrs = append(o.attrs, attrs...)
	}
}

func WithTimeFormat(f string) LoggerOption {
	return func(o *loggerOptions) {
		o.timeFormat = f
	}
}

func WithSanitizers(s ...Sanitizer) LoggerOption {
	return func(o *loggerOptions) {
		o.sanitizers = append(o.sanitizers, s...)
	}
}

func WithFormat(f string) LoggerOption {
	return func(o *loggerOptions) {
		if o.console == nil {
			o.console = &consoleOptions{}
		}
		o.console.format = f
	}
}

func WithLeveler(l slog.Leveler) LoggerOption {
	return func(o *loggerOptions) {
		o.leveler = l
	}
}

func WithHandlers(handlers ...slog.Handler) LoggerOption {
	return func(o *loggerOptions) {
		o.handlers = append(o.handlers, handlers...)
	}
}

func WithMiddleware(middleware ...slogmulti.Middleware) LoggerOption {
	return func(o *loggerOptions) {
		o.middleware = append(o.middleware, middleware...)
	}
}

func WithAddSource(addSource bool) LoggerOption {
	return func(o *loggerOptions) {
		o.addSource = addSource
	}
}

func WithoutConsole() LoggerOption {
	return func(o *loggerOptions) {
		o.console = nil
	}
}

func NewLogger(name string, attrs ...slog.Attr) *slog.Logger {
	return NewLoggerWithOpts(name, WithAttrs(attrs...), WithHandlers(NewRemoteHandler()))
}

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

func NewPackageLogger(attrs ...slog.Attr) *slog.Logger {
	name := PackageLoggerName(2)
	return NewLogger(name, attrs...)
}
