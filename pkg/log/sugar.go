package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/samber/lo"
)

const (
	ErrorKey          = "error"
	ErrorTextKey      = "text"
	ErrorBacktraceKey = "backtrace"

	LoggerKey    = "logger"
	OperationKey = "operation"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)

	Log(ctx context.Context, level slog.Level, msg string, args ...any)
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
}

type FormattingLogger interface {
	Logger

	Trace(msg string, args ...any)

	TraceContext(ctx context.Context, msg string, args ...any)

	Tracef(template string, vals ...any)
	Debugf(template string, vals ...any)
	Infof(template string, vals ...any)
	Warnf(template string, vals ...any)
	Errorf(template string, vals ...any)

	TracefContext(ctx context.Context, template string, vals ...any)
	DebugfContext(ctx context.Context, template string, vals ...any)
	InfofContext(ctx context.Context, template string, vals ...any)
	WarnfContext(ctx context.Context, template string, vals ...any)
	ErrorfContext(ctx context.Context, template string, vals ...any)

	With(args ...any) FormattingLogger
	WithGroup(group string) FormattingLogger
	WithAttrs(attrs ...slog.Attr) FormattingLogger
	WithFields(fields map[string]any) FormattingLogger
	WithContext(ctx context.Context) FormattingLogger
	WithError(err error) FormattingLogger
}

type formattingLogger struct {
	logger *slog.Logger
}

func (f *formattingLogger) pc() uintptr {
	var pcs [1]uintptr
	runtime.Callers(4, pcs[:]) // skip [Callers, pc, log, Sugar]
	return pcs[0]
}

// internal log any handler
func (f *formattingLogger) log(ctx context.Context, level slog.Level, msg string, values ...any) {
	if !f.logger.Enabled(ctx, level) {
		return
	}

	// ensure handler grouping is respected
	l := &formattingLogger{
		logger: f.logger.With(values...),
	}

	record := slog.NewRecord(time.Now(), level, msg, f.pc())

	if ctx != nil {
		attrs := logAttrsFromContext(ctx)
		record.Add(lo.ToAnySlice(attrs)...)
	}

	_ = l.logger.Handler().Handle(ctx, record)
}

// internal log attr handler
func (f *formattingLogger) logAttrs(ctx context.Context, level slog.Level, msg string, values ...slog.Attr) {
	if !f.logger.Enabled(ctx, level) {
		return
	}

	// ensure handler grouping is respected
	l := &formattingLogger{
		logger: f.logger.With(lo.ToAnySlice(values)...),
	}

	record := slog.NewRecord(time.Now(), level, msg, f.pc())

	if ctx != nil {
		attrs := logAttrsFromContext(ctx)
		record.Add(lo.ToAnySlice(attrs)...)
	}

	_ = l.logger.Handler().Handle(ctx, record)
}

func (f *formattingLogger) Trace(msg string, args ...any) {
	f.log(nil, LevelTrace, msg, args...)
}

func (f *formattingLogger) Debug(msg string, args ...any) {
	f.log(nil, LevelDebug, msg, args...)
}

func (f *formattingLogger) Info(msg string, args ...any) {
	f.log(nil, LevelInfo, msg, args...)
}

func (f *formattingLogger) Warn(msg string, args ...any) {
	f.log(nil, LevelWarn, msg, args...)
}

func (f *formattingLogger) Error(msg string, args ...any) {
	f.log(nil, LevelError, msg, args...)
}

func (f *formattingLogger) TraceContext(ctx context.Context, msg string, args ...any) {
	f.log(ctx, LevelTrace, msg, args...)
}

func (f *formattingLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	f.log(ctx, LevelDebug, msg, args...)
}

func (f *formattingLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	f.log(ctx, LevelInfo, msg, args...)
}

func (f *formattingLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	f.log(ctx, LevelWarn, msg, args...)
}

func (f *formattingLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	f.log(ctx, LevelError, msg, args...)
}

func (f *formattingLogger) Log(ctx context.Context, level slog.Level, msg string, values ...any) {
	f.log(ctx, level, msg, values...)
}

func (f *formattingLogger) LogAttrs(ctx context.Context, level slog.Level, msg string, values ...slog.Attr) {
	f.logAttrs(ctx, level, msg, values...)
}

func (f *formattingLogger) Tracef(template string, vals ...any) {
	f.log(nil, LevelTrace, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) Debugf(template string, vals ...any) {
	f.log(nil, LevelDebug, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) Infof(template string, vals ...any) {
	f.log(nil, LevelInfo, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) Warnf(template string, vals ...any) {
	f.log(nil, LevelWarn, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) Errorf(template string, vals ...any) {
	f.log(nil, LevelError, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) TracefContext(ctx context.Context, template string, vals ...any) {
	f.log(ctx, LevelDebug, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) DebugfContext(ctx context.Context, template string, vals ...any) {
	f.log(ctx, LevelDebug, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) InfofContext(ctx context.Context, template string, vals ...any) {
	f.log(ctx, LevelInfo, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) WarnfContext(ctx context.Context, template string, vals ...any) {
	f.log(ctx, LevelWarn, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) ErrorfContext(ctx context.Context, template string, vals ...any) {
	f.log(ctx, LevelError, fmt.Sprintf(template, vals...))
}

func (f *formattingLogger) With(args ...any) FormattingLogger {
	if len(args) == 0 {
		return f
	}

	logger := f.logger.With(args...)
	return &formattingLogger{logger: logger}
}

func (f *formattingLogger) WithGroup(group string) FormattingLogger {
	if group == "" {
		return f
	}

	logger := f.logger.WithGroup(group)
	return &formattingLogger{logger: logger}
}

func (f *formattingLogger) WithAttrs(attrs ...slog.Attr) FormattingLogger {
	return f.With(lo.ToAnySlice(attrs)...)
}

func (f *formattingLogger) WithFields(fields map[string]any) FormattingLogger {
	return f.WithAttrs(MapAttrs(fields)...)
}

func (f *formattingLogger) WithContext(ctx context.Context) FormattingLogger {
	return f.WithAttrs(logAttrsFromContext(ctx)...)
}

func (f *formattingLogger) WithError(err error) FormattingLogger {
	var result FormattingLogger = f
	result = result.WithAttrs(slog.Any(ErrorKey, err))
	return result
}

func NewFormattingLogger(name string, attrs ...slog.Attr) FormattingLogger {
	return &formattingLogger{
		logger: NewLogger(name, attrs...),
	}
}

func NewPackageLogger(attrs ...slog.Attr) FormattingLogger {
	name := PackageLoggerName(2)
	return NewFormattingLogger(name, attrs...)
}
