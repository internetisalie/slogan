package log

import (
	"bytes"
	"fmt"
	"iter"
	"log/slog"
	"runtime"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

// Callers returns an iterator over the call stack frames.
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

type Frame uintptr

func (f Frame) pc() uintptr {
	return uintptr(f) - 1
}

func (f Frame) FileLine() (string, int) {
	fn := runtime.FuncForPC(f.pc())
	if fn != nil {
		return fn.FileLine(f.pc())
	}
	return "unknown", 0
}

func (f Frame) Function() string {
	fn := runtime.FuncForPC(f.pc())
	if fn != nil {
		return fn.Name()
	}
	return "unknown"
}

func (f Frame) FunctionShort() string {
	fn := f.Function()
	parts := strings.Split(fn, "/")
	return parts[len(parts)-1]
}

func (f Frame) Equals(other Frame) bool {
	return uintptr(f) == uintptr(other)
}

func (f Frame) LogValue() slog.Value {
	file, line := f.FileLine()
	function := f.Function()

	return slog.StringValue(fmt.Sprintf("%s (%s:%d)", function, file, line))
}

type Stacker interface {
	Stack() Stack
}

type StackTrimmer interface {
	TrimStack(parent Stack) error
}

type Stack []Frame

func (s Stack) Trim(parent Stack) (Stack, bool) {
	count := len(s)
	otherCount := len(parent)

	if count < otherCount {
		return s, false
	}

	if count == 0 || otherCount == 0 {
		return s, false
	}

	// Trim matching stack traces
	idx := 1
	for s[count-idx-1].Equals(parent[otherCount-idx-1]) && otherCount > idx+1 {
		idx++
	}

	return s[:count-idx], idx > 0
}

func (s Stack) LogValue() slog.Value {
	return slog.AnyValue(lo.Map(s, func(item Frame, _ int) string {
		return item.LogValue().String()
	}))
}

func NewStack(skip int) Stack {
	var s Stack
	for f := range Callers(skip + 1) {
		s = append(s, f)
	}
	return s
}

type BackTracer interface {
	BackTrace() []byte
}

type Decorator interface {
	error
	IsDecorator()
}

type Messager interface {
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

func WrapSentinel(cause error, message string) error {
	result := &simple{
		message: message,
		cause:   cause,
	}
	return result
}

func NewSentinel(message string) error {
	result := &simple{
		message: message,
		cause:   nil,
	}
	return result
}

type Unwrapper interface {
	Unwrap() error
}

func BackTrace(err error) []byte {
	buffer := new(bytes.Buffer)
	backTrace(err, buffer, true)
	return buffer.Bytes()
}

func backTrace(err error, buffer *bytes.Buffer, root bool) {
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

func ErrorString(message string, cause error) string {
	if cause == nil {
		return message
	}
	return fmt.Sprintf("%s: %s", message, cause.Error())
}

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

const MessageSeparator = ": "

func Message(err error) string {
	if messager, ok := err.(Messager); ok {
		return messager.Message()
	}

	message := strings.Split(err.Error(), MessageSeparator)
	return message[0]
}

func Messages(err error) []string {
	return strings.Split(err.Error(), MessageSeparator)
}
