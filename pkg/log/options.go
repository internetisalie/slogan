package log

import (
	"log/slog"
)

type AttrReplacer interface {
	ReplaceAttr(groups []string, a slog.Attr) slog.Attr
}

type AttrReplacerFunc func(groups []string, a slog.Attr) slog.Attr

func (f AttrReplacerFunc) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	return f(groups, a)
}

type Sanitizer interface {
	Sanitize(a slog.Attr) slog.Value
}

type SanitizerFunc func(value slog.Attr) slog.Value

func (f SanitizerFunc) Sanitize(a slog.Attr) slog.Value {
	return f(a)
}

var sanitizer Sanitizer

func RegisterSanitizer(s Sanitizer) {
	sanitizer = s
}

type SanitizerAttrReplacer struct {
	sanitizer Sanitizer
	next      AttrReplacer
}

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

func MultiSanitizer(sanitizers ...Sanitizer) Sanitizer {
	return multiSanitizer{sanitizers: sanitizers}
}

type TimestampAttrReplacer struct {
	format string
	next   AttrReplacer
}

func (t *TimestampAttrReplacer) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		a.Value = slog.StringValue(a.Value.Time().Format(t.format))
	}

	if t.next != nil {
		a = t.next.ReplaceAttr(groups, a)
	}

	return a
}

func NewTimestampAttrReplacer(next AttrReplacer, format string) *TimestampAttrReplacer {
	return &TimestampAttrReplacer{
		format: format,
		next:   next,
	}
}
