package log

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterSanitizer(t *testing.T) {
	buf := &bytes.Buffer{}

	// Register a global sanitizer
	RegisterSanitizer(SanitizerFunc(func(a slog.Attr) slog.Value {
		if a.Key == "global-secret" {
			return slog.StringValue("REDACTED")
		}
		return a.Value
	}))

	logger := NewLoggerWithOpts("test",
		WithWriter(buf),
		WithFormat(FormatLogFmt),
		WithTimeFormat(""),
	)

	logger.Info("test", slog.String("global-secret", "12345"))
	assert.Contains(t, buf.String(), "global-secret=REDACTED")
}

func TestAttrReplacerFunc(t *testing.T) {
	var f AttrReplacerFunc = func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "foo" {
			return slog.String("foo", "bar")
		}
		return a
	}
	
	attr := f.ReplaceAttr(nil, slog.String("foo", "orig"))
	assert.Equal(t, "bar", attr.Value.String())
}
