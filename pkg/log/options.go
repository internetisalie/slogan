package log

import (
	"log/slog"
)

// AttrReplacer is implemented by types that can transform log attributes.
// It is used in handler options to customize attribute formatting, sanitization,
// or other transformations on a per-record basis. Replacers can be chained for
// composable attribute processing.
type AttrReplacer interface {
	// ReplaceAttr returns a modified version of the attribute, potentially with
	// a different key, value, or both. Groups contains the nesting path for
	// grouped attributes. Returning an invalid attribute (key="") suppresses the attribute.
	ReplaceAttr(groups []string, a slog.Attr) slog.Attr
}

// AttrReplacerFunc is an adapter to allow ordinary functions to be used as AttrReplacer implementations.
type AttrReplacerFunc func(groups []string, a slog.Attr) slog.Attr

// ReplaceAttr calls f(groups, a).
func (f AttrReplacerFunc) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	return f(groups, a)
}

// Sanitizer is implemented by types that can mask, redact, or transform sensitive
// attribute values in logs. Sanitizers are applied before attributes reach log handlers,
// protecting sensitive data like API keys, passwords, or PII from being logged.
type Sanitizer interface {
	// Sanitize returns a modified value for the given attribute, or the original
	// value if no sanitization is needed. The attribute's key may be used to
	// determine if sanitization should be applied.
	Sanitize(a slog.Attr) slog.Value
}

// SanitizerFunc is an adapter to allow ordinary functions to be used as Sanitizer implementations.
type SanitizerFunc func(value slog.Attr) slog.Value

// Sanitize calls f(a).
func (f SanitizerFunc) Sanitize(a slog.Attr) slog.Value {
	return f(a)
}

var sanitizer Sanitizer

// RegisterSanitizer registers a global sanitizer that is applied to all log attributes
// that don't have a local sanitizer. This allows for application-wide redaction of
// sensitive data types.
func RegisterSanitizer(s Sanitizer) {
	sanitizer = s
}

// SanitizerAttrReplacer is an AttrReplacer that applies sanitization to attribute values
// and optionally delegates to a next replacer in a chain. This enables composing multiple
// transformation stages (sanitize → timestamp format → other custom transformations).
type SanitizerAttrReplacer struct {
	sanitizer Sanitizer
	next      AttrReplacer
}

// ReplaceAttr sanitizes the attribute value and delegates to the next replacer if configured.
func (r SanitizerAttrReplacer) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	s := r.sanitizer
	if s == nil {
		s = sanitizer
	}

	if s != nil {
		a.Value = s.Sanitize(a)
	}

	if r.next != nil {
		a = r.next.ReplaceAttr(groups, a)
	}

	return a
}

// NewSanitizerAttrReplacer creates a new SanitizerAttrReplacer with optional local sanitizer
// and next replacer in the chain. If sanitizer is nil, the global registered sanitizer is used.
// If next is nil, no further transformation is applied after sanitization.
func NewSanitizerAttrReplacer(sanitizer Sanitizer, next AttrReplacer) *SanitizerAttrReplacer {
	return &SanitizerAttrReplacer{
		sanitizer: sanitizer,
		next:      next,
	}
}

type multiSanitizer struct {
	sanitizers []Sanitizer
}

func (m multiSanitizer) Sanitize(a slog.Attr) slog.Value {
	v := a.Value
	for _, s := range m.sanitizers {
		v = s.Sanitize(slog.Attr{Key: a.Key, Value: v})
	}
	return v
}

// MultiSanitizer combines multiple Sanitizer instances into a single Sanitizer
// that applies each sanitizer in sequence to the attribute value. This allows
// layering multiple sanitization strategies (e.g., redact passwords, then mask emails).
func MultiSanitizer(sanitizers ...Sanitizer) Sanitizer {
	return multiSanitizer{sanitizers: sanitizers}
}

// TimestampAttrReplacer is an AttrReplacer that formats timestamp attributes using
// a custom time layout. Only top-level timestamp attributes (not nested in groups)
// are formatted; this prevents redundant formatting of timestamps in nested structures.
type TimestampAttrReplacer struct {
	format string
	next   AttrReplacer
}

// ReplaceAttr formats the timestamp using the configured format and delegates to the next replacer.
func (t *TimestampAttrReplacer) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		a.Value = slog.StringValue(a.Value.Time().Format(t.format))
	}

	if t.next != nil {
		a = t.next.ReplaceAttr(groups, a)
	}

	return a
}

// NewTimestampAttrReplacer creates a new TimestampAttrReplacer with the specified format string
// (e.g., time.RFC3339, "15:04:05.000000", etc.) and optional next replacer in the chain.
func NewTimestampAttrReplacer(next AttrReplacer, format string) *TimestampAttrReplacer {
	return &TimestampAttrReplacer{
		format: format,
		next:   next,
	}
}
