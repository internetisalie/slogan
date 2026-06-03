package log

import (
	"fmt"
	"iter"
	"log/slog"
	"runtime"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"github.com/valyala/bytebufferpool"
)

// Callers returns an iterator over call stack frames starting from the caller.
// The skip parameter indicates how many frames to skip before recording (skip=0
// includes the frame of the Callers call itself). Frames are returned lazily via
// an iterator, allowing efficient processing of potentially large stacks.
func Callers(skip int) iter.Seq[Frame] {
	return func(yield func(Frame) bool) {
		const framePageSize = 32
		pcs := make([]uintptr, framePageSize)
		for {
			n := runtime.Callers(skip+2, pcs)
			if n == 0 {
				return
			}
			for i := 0; i < n; i++ {
				if !yield(Frame(pcs[i])) {
					return
				}
			}
			if n < framePageSize {
				return
			}
			skip += n
		}
	}
}

// Frame represents a single program counter (PC) value within a call stack.
// It wraps a uintptr and provides methods to extract function and file/line information.
// Frame values are used in Stack traces to represent individual stack frames.
type Frame uintptr

func (f Frame) pc() uintptr {
	return uintptr(f) - 1
}

// FileLine returns the source file path and line number for this frame.
// Returns ("unknown", 0) if the file/line information cannot be determined.
func (f Frame) FileLine() (string, int) {
	fn := runtime.FuncForPC(f.pc())
	if fn != nil {
		return fn.FileLine(f.pc())
	}
	return "unknown", 0
}

// Function returns the full qualified function name for the frame
// (e.g., "github.com/user/package.(*Type).Method"), or "unknown" if unavailable.
func (f Frame) Function() string {
	fn := runtime.FuncForPC(f.pc())
	if fn != nil {
		return fn.Name()
	}
	return "unknown"
}

// FunctionShort returns the short function name without the package path
// (e.g., "(*Type).Method" from the full "github.com/user/package.(*Type).Method").
func (f Frame) FunctionShort() string {
	fn := f.Function()
	parts := strings.Split(fn, "/")
	return parts[len(parts)-1]
}

// Equals reports whether the frame points to the same code location as another frame.
func (f Frame) Equals(other Frame) bool {
	return uintptr(f) == uintptr(other)
}

// LogValue returns a structured log value representation of the frame in the format
// "function (file:line)", suitable for use in log attributes.
func (f Frame) LogValue() slog.Value {
	file, line := f.FileLine()
	function := f.Function()

	return slog.StringValue(fmt.Sprintf("%s (%s:%d)", function, file, line))
}

// Stacker is implemented by types that carry stack trace information.
type Stacker interface {
	// Stack returns the call stack frames captured at the time of creation.
	Stack() Stack
}

// StackTrimmer is implemented by types that can remove redundant common frames
// from their stack traces (e.g., removing frames that are common to a parent call).
type StackTrimmer interface {
	// TrimStack removes stack frames that are common with the parent Stack,
	// returning a new error with the trimmed stack. If no trimming is possible, returns itself.
	TrimStack(parent Stack) error
}

// Stack is a sequence of call stack frames.
type Stack []Frame

// Trim removes trailing frames from the stack that match the parent stack,
// reducing redundancy when stacks are nested. Returns the trimmed stack and
// a boolean indicating whether any frames were removed.
func (s Stack) Trim(parent Stack) (Stack, bool) {
	count := len(s)
	otherCount := len(parent)

	if count < otherCount {
		return s, false
	}

	if count == 0 || otherCount == 0 {
		return s, false
	}

	// Trim matching stack traces from the end of the stack that are common with parent.
	// Compare frames from the end backwards until a mismatch is found.
	idx := 0
	for idx < otherCount && idx < count && s[count-1-idx].Equals(parent[otherCount-1-idx]) {
		idx++
	}

	return s[:count-idx], idx > 0
}

// LogValue returns a structured log value representation of the stack as an array of strings.
func (s Stack) LogValue() slog.Value {
	return slog.AnyValue(lo.Map(s, func(item Frame, _ int) string {
		return item.LogValue().String()
	}))
}

// NewStack captures the current call stack and returns it as a Stack.
// The skip parameter indicates how many frames to skip (skip=0 starts at the caller of NewStack).
func NewStack(skip int) Stack {
	var s Stack
	for f := range Callers(skip + 1) {
		s = append(s, f)
	}
	return s
}

// BackTracer is implemented by types that can render their error chain as a formatted backtrace.
type BackTracer interface {
	// BackTrace returns a byte representation of the error chain formatted for display.
	BackTrace() []byte
}

// Decorator is implemented by error types that decorate or wrap other errors
// with additional context or metadata.
type Decorator interface {
	error
	// IsDecorator marks this error as a decorator. It distinguishes decorators
	// from regular errors to enable special handling in logging and error tracing.
	IsDecorator()
}

// Messager is implemented by types that provide a message string for logging.
type Messager interface {
	// Message returns the primary message for this error.
	Message() string
}

// simple is a standard error with optional cause and stack.
type simple struct {
	message string
	cause   error
	stack   Stack
}

func (s *simple) Error() string {
	if s.cause == nil {
		return s.message
	}
	return fmt.Sprintf("%s: %s", s.message, s.cause.Error())
}

func (s *simple) Message() string {
	return s.message
}

func (s *simple) Unwrap() error {
	return s.cause
}

func (s *simple) Stack() Stack {
	return s.stack
}

func (s *simple) TrimStack(parent Stack) error {
	trimmedStack, ok := s.stack.Trim(parent)
	if ok {
		return &simple{
			message: s.message,
			cause:   s.cause,
			stack:   trimmedStack,
		}
	}
	return s
}

func (s *simple) BackTrace() []byte {
	return BackTrace(s)
}

func (s *simple) LogValue() slog.Value {
	return slog.AnyValue(LogValues(s.message, s.cause, s.stack))
}

func newSimple(message string, cause error) error {
	result := &simple{
		message: message,
		cause:   cause,
		stack:   NewStack(2), // skip newSimple and parent
	}
	if stacker, ok := cause.(StackTrimmer); ok {
		result.cause = stacker.TrimStack(result.stack)
	}
	return result
}

// WrapSentinel wraps a cause error with an additional message, creating a decorator
// error without capturing a new stack. Use when you want to annotate an error with
// context but preserve the original stack trace from the cause.
func WrapSentinel(cause error, message string) error {
	result := &simple{
		message: message,
		cause:   cause,
	}
	return result
}

// NewSentinel creates a sentinel error with the given message and no cause.
// Use for root errors that mark specific error conditions without wrapping an underlying error.
func NewSentinel(message string) error {
	result := &simple{
		message: message,
		cause:   nil,
	}
	return result
}

// Unwrapper is implemented by error types that can unwrap to reveal underlying causes.
type Unwrapper interface {
	// Unwrap returns the underlying error (for single errors) or nil if none.
	Unwrap() error
}

// BackTrace renders a formatted backtrace for an error chain. It processes errors
// recursively, starting from the root cause and tracing through all wrapped errors.
// Each error's message and stack trace (if available) are included. Returns a
// formatted byte representation suitable for display or logging.
func BackTrace(err error) []byte {
	buffer := bytebufferpool.Get()
	defer bytebufferpool.Put(buffer)
	backTrace(err, buffer, true)
	return buffer.Bytes()
}

func backTrace(err error, buffer *bytebufferpool.ByteBuffer, root bool) {
	if err == nil {
		return
	}

	interesting := isInteresting(err)
	if interesting {
		if root {
			buffer.WriteString("Root Cause: ")
		} else {
			buffer.WriteString("Caused: ")
		}
		buffer.WriteString(Message(err))
		buffer.WriteString("\n")

		if stacker, ok := err.(Stacker); ok {
			for _, frame := range stacker.Stack() {
				buffer.WriteString("  ")
				buffer.WriteString(frame.Function())
				buffer.WriteString("\n")

				file, line := frame.FileLine()
				buffer.WriteString("    ")
				buffer.WriteString(file)
				buffer.WriteString(":")
				buffer.WriteString(strconv.Itoa(line))
				buffer.WriteString("\n")
			}
		}
	}

	switch u := err.(type) {
	case interface{ Unwrap() error }:
		backTrace(u.Unwrap(), buffer, !interesting && root)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			backTrace(e, buffer, !interesting && root)
		}
	}
}

func isInteresting(err error) bool {
	if _, ok := err.(Decorator); ok {
		// If it has a stack, it's interesting despite being a decorator
		if s, ok := err.(Stacker); ok && len(s.Stack()) > 0 {
			return true
		}
		return false
	}
	return true
}

// ErrorString combines a message and cause error into a single error string.
// If cause is nil, returns just the message. Otherwise returns "message: cause.Error()".
// Useful for constructing error messages from components.
func ErrorString(message string, cause error) string {
	if cause == nil {
		return message
	}
	return fmt.Sprintf("%s: %s", message, cause.Error())
}

// LogValues creates a structured attribute map for logging an error with its message,
// cause chain, and stack trace. Returns a map suitable for use with slog.Any().
func LogValues(message string, cause error, stack Stack) map[string]any {
	const logKeyMessage = "message"
	const logKeyStack = "stack"
	const logKeyCause = "cause"

	result := map[string]any{
		logKeyMessage: message,
	}

	if cause != nil {
		if logValuer, ok := cause.(slog.LogValuer); ok {
			result[logKeyCause] = logValuer.LogValue().Any()
		} else {
			result[logKeyCause] = cause
		}
	}

	if len(stack) > 0 {
		result[logKeyStack] = stack.LogValue().Any()
	}

	return result
}

// MessageSeparator is the string used to separate multiple messages in error chains (": ").
const MessageSeparator = ": "

// Message extracts the primary message from an error. If the error implements Messager,
// its Message() method is called. Otherwise, the error string is split on MessageSeparator
// and the first component is returned.
func Message(err error) string {
	if messager, ok := err.(Messager); ok {
		return messager.Message()
	}

	message := strings.Split(err.Error(), MessageSeparator)
	return message[0]
}

// Messages splits an error string on MessageSeparator to extract all message components.
// Useful for analyzing multi-level error messages created by wrapping errors.
func Messages(err error) []string {
	return strings.Split(err.Error(), MessageSeparator)
}
