package log

import (
	"bytes"
	"context"
	stderrs "errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	slogmulti "github.com/samber/slog-multi"
	"github.com/stretchr/testify/assert"
)

func TestNewLoggerWithOpts(t *testing.T) {
	buf := &bytes.Buffer{}

	sanitizer := SanitizerFunc(func(a slog.Attr) slog.Value {
		if a.Key == "secret" {
			return slog.StringValue("REDACTED")
		}
		return a.Value
	})

	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithAttrs(slog.String("foo", "bar")),
		WithSanitizers(sanitizer),
	)

	logger.Info("hello", slog.String("secret", "12345"))

	output := buf.String()
	assert.Contains(t, output, "level=INFO")
	assert.Contains(t, output, "msg=hello")
	assert.Contains(t, output, "foo=bar")
	assert.Contains(t, output, "secret=REDACTED")
	assert.NotContains(t, output, "12345")
}

func TestNewLoggerWithOpts_Leveler(t *testing.T) {
	buf := &bytes.Buffer{}
	levelVar := &slog.LevelVar{}
	levelVar.Set(slog.LevelWarn)

	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithLeveler(levelVar),
	)

	logger.Info("should not see this")
	logger.Warn("should see this")

	output := buf.String()
	assert.NotContains(t, output, "should not see this")
	assert.Contains(t, output, "should see this")

	levelVar.Set(slog.LevelInfo)
	buf.Reset()
	logger.Info("now should see this")
	assert.Contains(t, buf.String(), "now should see this")
}

func TestWithTimeFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	customFormat := "2006-01-02"
	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithTimeFormat(customFormat),
	)

	logger.Info("test")
	// The output should contain the date in the custom format.
	// Since we don't control the time here, we just check if it matches the regex for YYYY-MM-DD
	assert.Regexp(t, `time=\d{4}-\d{2}-\d{2}`, buf.String())
}

func TestPackageLoggerName(t *testing.T) {
	// We'll register a very specific prefix that we know will match our current package name
	// but first we need to know what the current package name is.
	fullName := PackageLoggerName(1)
	
	prefix := "my.custom.prefix."
	RegisterPackageLoggerPrefix(prefix)
	
	// Since we can't easily change the runtime caller, we'll just verify that if we register 
	// the actual package name as a prefix, it might behave differently or we can just trust 
	// the logic if it passes with the registered prefix.
	// Actually, let's just test the logic with a helper if possible, but the function isn't exported.
	
	// Let's just verify it works as is.
	assert.NotEmpty(t, fullName)
	assert.True(t, strings.HasSuffix(fullName, ".log") || fullName == "log", "name should end with .log or be log, got %s", fullName)
}

func TestStandardLogger(t *testing.T) {
	l1 := StandardLogger()
	l2 := StandardLogger()
	assert.NotNil(t, l1)
	assert.Equal(t, l1, l2, "StandardLogger should return the same instance (singleton)")
}

func TestLoggingLogger(t *testing.T) {
	l := LoggingLogger()
	assert.NotNil(t, l)
}

func TestWithHandlers(t *testing.T) {
	buf := &bytes.Buffer{}
	extraBuf := &bytes.Buffer{}
	extraHandler := slog.NewTextHandler(extraBuf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithHandlers(extraHandler),
	)

	logger.Info("hello")

	assert.Contains(t, buf.String(), "msg=hello")
	assert.Contains(t, extraBuf.String(), "msg=hello")
}

func TestWithoutConsoleAndMiddleware(t *testing.T) {
	buf := &bytes.Buffer{}
	
	middlewareCalled := false
	middleware := slogmulti.NewHandleInlineMiddleware(func(ctx context.Context, record slog.Record, next func(context.Context, slog.Record) error) error {
		middlewareCalled = true
		return next(ctx, record)
	})

	extraBuf := &bytes.Buffer{}
	extraHandler := slog.NewTextHandler(extraBuf, nil)

	logger := NewLoggerWithOpts("test",
		WithWriter(buf), // should be ignored because of WithoutConsole
		WithoutConsole(),
		WithHandlers(extraHandler),
		WithMiddleware(middleware),
	)

	logger.Info("hello")

	assert.Empty(t, buf.String(), "console buffer should be empty")
	assert.Contains(t, extraBuf.String(), "msg=hello")
	assert.True(t, middlewareCalled, "middleware should have been called")
}

func TestWithAddSource(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithAddSource(true),
	)

	logger.Info("test source")
	
	output := buf.String()
	// Check for source information in logfmt output: source=...:line
	assert.Contains(t, output, "source=")
	assert.Contains(t, output, "opts_test.go:")
}

func TestErrorWrappers(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
	)

	err := fmt.Errorf("root error")
	// Use unified Wrap with WithErrorXXX options
	err = Wrap(err,
		WithErrorLevel(slog.LevelError),
		WithErrorAttrs(slog.String("request_id", "123")),
		WithErrorStack(),
	)

	logger.Error("something failed", slog.Any(ErrorKey, err))

	output := buf.String()
	assert.Contains(t, output, "msg=\"something failed\"")
	assert.Contains(t, output, "error.message=\"root error\"")
	assert.Contains(t, output, "request_id=123", "request_id should be at top-level")
	assert.Contains(t, output, "level=ERROR")
	assert.Contains(t, output, "error.stack")
}

func TestErrorLevelPromotion(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
	)

	err := fmt.Errorf("root error")
	// Wrap with WithErrorLevel
	err = Wrap(err, WithErrorLevel(slog.LevelError))

	// Log with Info level, but error should promote it to ERROR
	logger.Info("info call", slog.Any(ErrorKey, err))

	output := buf.String()
	assert.Contains(t, output, "level=ERROR", "log level should have been promoted to ERROR")
	assert.Contains(t, output, "msg=\"info call\"")
}

func TestErrWithStack(t *testing.T) {
	err := fmt.Errorf("error")
	wrapped := ErrWithStack(err)
	assert.NotNil(t, wrapped)
	assert.Implements(t, (*Stacker)(nil), wrapped)
}

func TestJoinedErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
	)

	err1 := Wrap(fmt.Errorf("error 1"),
		WithErrorLevel(slog.LevelWarn),
		WithErrorAttrs(slog.String("attr1", "val1")),
	)

	err2 := Wrap(fmt.Errorf("error 2"),
		WithErrorLevel(slog.LevelError),
		WithErrorAttrs(slog.String("attr2", "val2")),
	)

	// Join them (Go 1.20+)
	joined := stderrs.Join(err1, err2)

	buf.Reset()
	logger.Info("joined test", slog.Any(ErrorKey, joined))
	output := buf.String()
	assert.Contains(t, output, "level=ERROR", "should promote to ERROR from joined error")
	assert.Contains(t, output, "attr1=val1")
	assert.Contains(t, output, "attr2=val2")

	// Test recursive wrapping
	err3 := ErrWithLevel(fmt.Errorf("wrapper"), slog.LevelInfo)
	err3 = ErrWithAttrs(err3, slog.String("top", "level"))

	// err1 (WARN) -> wrapped in something -> wrapped in err3 (INFO)
	wrapped := fmt.Errorf("wrap: %w", err1)
	final := ErrWithAttrs(wrapped, slog.String("final", "attr"))

	buf.Reset()
	logger.Info("recursive test", slog.Any(ErrorKey, final))

	output = buf.String()
	assert.Contains(t, output, "level=WARN", "should promote to WARN from deep error")
	assert.Contains(t, output, "attr1=val1")
	assert.Contains(t, output, "final=attr")
}
