package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// ANSI modes
const (
	ansiReset         = "\033[0m"
	ansiFaint         = "\033[2m"
	ansiBlack         = "\033[30m"
	ansiBrightRed     = "\033[91m"
	ansiBrightGreen   = "\033[92m"
	ansiBrightYellow  = "\033[93m"
	ansiBrightBlue    = "\033[94m"
	ansiBrightMagenta = "\033[95m"
	ansiBrightCyan    = "\033[96m"
	ansiBrightWhite   = "\033[97m"
)

type HumanHandler struct {
	opts   slog.HandlerOptions
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
	out    io.Writer
}

func NewHumanHandler(out io.Writer, opts *slog.HandlerOptions) *HumanHandler {
	h := &HumanHandler{out: out, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *HumanHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// !+WithGroup
func (h *HumanHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	// Add an unopened group to h2 without modifying h.
	h2.groups = AddGroup(h.groups, name)
	return &h2
}

//!-WithGroup

// !+WithAttrs
func (h *HumanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := *h
	h2.attrs = SetAttrsAtPath(h.attrs, h.groups, attrs)
	return &h2
}

//!-WithAttrs

// !+Handle
func (h *HumanHandler) Handle(ctx context.Context, r slog.Record) error {
	bufp := allocBuf()
	buf := *bufp
	defer func() {
		*bufp = buf
		freeBuf(bufp)
	}()

	// write time
	if !r.Time.IsZero() {
		buf = h.appendAttr(buf, slog.Time(slog.TimeKey, r.Time), 0)
	}

	// write level
	buf = h.appendAttr(buf, slog.Any(slog.LevelKey, r.Level), 0)

	// write logger
	for _, attr := range h.attrs {
		if attr.Key == LoggerKey {
			buf = h.appendAttr(buf, attr, 0)
			break
		}
	}

	// write source
	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		// Optimize to minimize allocation.
		srcbufp := allocBuf()
		defer freeBuf(srcbufp)
		if f.File != "" {
			dir, file := filepath.Split(f.File)
			*srcbufp = append(*srcbufp, filepath.Base(dir)...)
			*srcbufp = append(*srcbufp, filepath.Separator)
			*srcbufp = append(*srcbufp, file...)
			*srcbufp = append(*srcbufp, ':')
			*srcbufp = strconv.AppendInt(*srcbufp, int64(f.Line), 10)
			buf = h.appendAttr(buf, slog.String(slog.SourceKey, string(*srcbufp)), 0)
		}
	}

	// write message
	buf = h.appendAttr(buf, slog.String(slog.MessageKey, r.Message), -1)

	// add handler-stored attributes
	for _, attr := range h.attrs {
		if attr.Key == LoggerKey {
			continue
		}
		buf = h.appendAttr(buf, attr, 1)
	}

	if r.NumAttrs() > 0 {
		// add record-stored attributes
		r.Attrs(func(a slog.Attr) bool {
			buf = h.appendAttr(buf, a, len(h.groups)+1)
			return true
		})
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf)
	return err
}

// !-Handle
func (h *HumanHandler) appendAttr(buf []byte, a slog.Attr, indentLevel int) []byte {
	// Resolve the Attr's value before doing anything else.
	a.Value = a.Value.Resolve()
	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return buf
	}

	if indentLevel < 1 {
		// no indentation, no field name, no newline
		switch a.Key {
		case slog.TimeKey:
			buf = a.Value.Time().AppendFormat(buf, FormatTimestampHuman)
		case slog.LevelKey:
			switch a.Value.Any().(slog.Level) {
			case LevelTrace:
				buf = append(buf, ansiBrightBlue...)
				buf = append(buf, "TRC"...)
				buf = append(buf, ansiReset...)
			case LevelDebug:
				buf = append(buf, ansiBrightCyan...)
				buf = append(buf, "DBG"...)
				buf = append(buf, ansiReset...)
			case LevelInfo:
				buf = append(buf, ansiBrightGreen...)
				buf = append(buf, "INF"...)
				buf = append(buf, ansiReset...)
			case LevelWarn:
				buf = append(buf, ansiBrightYellow...)
				buf = append(buf, "WRN"...)
				buf = append(buf, ansiReset...)
			case LevelError:
				buf = append(buf, ansiBrightRed...)
				buf = append(buf, "ERR"...)
				buf = append(buf, ansiReset...)
				buf = append(buf, " "...)
			}
		case slog.SourceKey:
			buf = append(buf, ansiFaint...)
			buf = append(buf, a.Value.String()...)
			buf = append(buf, ansiReset...)
		case LoggerKey:
			buf = append(buf, a.Value.String()...)
		case slog.MessageKey:
			buf = append(buf, ansiBrightWhite...)
			buf = append(buf, a.Value.String()...)
			buf = append(buf, ansiReset...)
		}
		if indentLevel < 0 {
			buf = append(buf, "\n"...)
		} else {
			buf = append(buf, " "...)
		}
		return buf
	}

	buf = h.appendIndent(buf, indentLevel)
	buf = h.appendValue(buf, a, indentLevel)
	return buf
}

func (h *HumanHandler) appendValue(buf []byte, a slog.Attr, indentLevel int) []byte {
	kind := a.Value.Kind()
	switch kind {
	case slog.KindString:
		// Quote string values, to make them easy to parse.
		buf = h.appendKey(buf, a.Key, " ")
		str := a.Value.String()
		if strings.Contains(str, "\n") {
			buf = append(buf, "\n"...)
			idx := strings.Index(str, "\n")
			for idx != -1 {
				buf = h.appendIndent(buf, indentLevel+1)
				buf = append(buf, str[:idx+1]...)
				str = str[idx+1:]
				idx = strings.Index(str, "\n")
			}
		}
		buf = append(buf, str...)
		buf = append(buf, '\n')
	case slog.KindTime:
		// Write times in a standard way, without the monotonic time.
		buf = h.appendKey(buf, a.Key, " ")
		buf = a.Value.Time().AppendFormat(buf, FormatTimestampMicro)
		buf = append(buf, '\n')
	case slog.KindGroup:
		attrs := a.Value.Group()
		// Ignore empty groups.
		if len(attrs) == 0 {
			return buf
		}
		// If the key is non-empty, write it out and indent the rest of the attrs.
		// Otherwise, inline the attrs.
		if a.Key != "" {
			buf = h.appendKey(buf, a.Key, "\n")
			indentLevel++
		}
		for _, ga := range attrs {
			buf = h.appendAttr(buf, ga, indentLevel)
		}
	default:
		if a.Value.Kind() == slog.KindAny {
			a.Value = Value(a.Value.Any())
			if a.Value.Kind() != kind {
				return h.appendValue(buf, a, indentLevel)
			}
		}

		buf = h.appendKey(buf, a.Key, " ")
		buf = append(buf, a.Value.String()...)
		buf = append(buf, '\n')
	}
	return buf
}

func (h *HumanHandler) appendIndent(buf []byte, indentLevel int) []byte {
	// Indent 2 spaces per level.
	return append(buf, fmt.Sprintf("%*s", indentLevel*2, "")...)
}

func (h *HumanHandler) appendKey(buf []byte, name string, trailer string) []byte {
	buf = append(buf, ansiFaint...)
	buf = append(buf, name...)
	buf = append(buf, ":"...)
	buf = append(buf, ansiReset...)
	buf = append(buf, trailer...)
	return buf
}

// !+pool
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return &b
	},
}

func allocBuf() *[]byte {
	return bufPool.Get().(*[]byte)
}

func freeBuf(b *[]byte) {
	// To reduce peak allocation, return only smaller buffers to the pool.
	const maxBufferSize = 16 << 10
	if cap(*b) <= maxBufferSize {
		*b = (*b)[:0]
		bufPool.Put(b)
	}
}

//!-pool
