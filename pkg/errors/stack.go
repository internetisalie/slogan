package errors

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

const framePageSize = 16

type callersPage struct {
	skip   int
	frames []uintptr
	off    int
}

// nextPage returns the next page of caller frames
func (c callersPage) nextPage() callersPage {
	// skip Next() and nextPage() calls
	const skip = 2
	page := newCallersPage(c.skip + len(c.frames) + skip)
	page.skip -= skip
	return page
}

// Next returns the next available frame, and an updated cursor, and a success indicator.
func (c callersPage) Next() (uintptr, callersPage, bool) {
	switch len(c.frames) {
	case 0:
		return uintptr(0), callersPage{}, false
	case c.off:
		return c.nextPage().Next()
	default:
		frame := c.frames[c.off]
		c.off++
		return frame, c, true
	}
}

func newCallersPage(skip int) callersPage {
	frames := make([]uintptr, framePageSize)
	count := runtime.Callers(skip+2, frames) // skip this function and runtime.Callers
	if count == 0 {
		return callersPage{}
	}
	return callersPage{
		skip:   skip,
		frames: frames[:count],
	}
}

type callers struct {
	scanned []Frame
	page    callersPage
}

func (c callers) Next() (uintptr, callers, bool) {
	frame, page, ok := c.page.Next()
	if ok {
		if len(c.scanned)%framePageSize == 0 {
			c.scanned = slices.Grow(c.scanned, framePageSize)
		}
		c.scanned = append(c.scanned, Frame(frame))
		c.page = page
		return frame, c, true
	}
	return uintptr(0), c, false
}

func (c callers) Frames() []Frame {
	return c.scanned
}

func newCallers(skip int) callers {
	return callers{
		page: newCallersPage(skip + 1),
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
	var ok bool

	c := newCallers(skip + 1) // skip NewStack()
	_, c, ok = c.Next()
	for ok {
		_, c, ok = c.Next()
	}
	return c.Frames()
}

type BackTracer interface {
	BackTrace() []byte
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
	}
	return result
}

type Unwrapper interface {
	Unwrap() error
}

func BackTrace(err error) []byte {
	buffer := new(bytes.Buffer)

	unwrapper, ok := err.(Unwrapper)
	var cause error
	if ok {
		cause = unwrapper.Unwrap()
	}

	if cause != nil {
		if backTracer, ok := cause.(BackTracer); ok {
			buffer.Write(backTracer.BackTrace())
		} else {
			causeMessages := Messages(cause)
			slices.Reverse(causeMessages)
			buffer.WriteString("Root Cause: ")
			buffer.WriteString(causeMessages[0])
			buffer.WriteString("\n")
			for i, causeMessage := range causeMessages {
				if i == 0 {
					continue
				}
				buffer.WriteString("Caused: ")
				buffer.WriteString(causeMessage)
				buffer.WriteString("\n")
			}
		}

		buffer.WriteString("Caused: ")
	} else {
		buffer.WriteString("Root Cause: ")
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

	return buffer.Bytes()
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
