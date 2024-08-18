package errors

import (
	"fmt"
	"log/slog"
)

type ValueError struct {
	message string
	value   any
	cause   error
	stack   Stack
}

func (e ValueError) BackTrace() []byte {
	return BackTrace(e)
}

func (e ValueError) TrimStack(parent Stack) error {
	trimmedStack, ok := e.stack.Trim(parent)
	if ok {
		return &ValueError{
			message: e.message,
			cause:   e.cause,
			value:   e.value,
			stack:   trimmedStack,
		}
	}
	return e
}

func (e ValueError) Stack() Stack {
	return e.stack
}

func (e ValueError) Error() string {
	return ErrorString(e.message, e.cause)
}

func (e ValueError) LogValue() slog.Value {
	result := LogValues(e.message, e.cause, e.stack)
	result["value"] = e.value
	return slog.AnyValue(result)
}

func (e ValueError) Unwrap() error {
	return e.cause
}

func (e ValueError) Message() string {
	return e.message
}

func (e ValueError) Value() any {
	return e.value
}

func NewValueError(value any, cause error, message string, args ...any) error {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	result := ValueError{
		message: message,
		value:   value,
		cause:   cause,
		stack:   NewStack(1), // skip NewValueError
	}
	if stacker, ok := cause.(StackTrimmer); ok {
		result.cause = stacker.TrimStack(result.stack)
	}
	return result
}

func NewPanicError(v any, skipStack int) error {
	if e, ok := v.(error); ok {
		result := &simple{
			message: "panic",
			cause:   e,
			// skip NewPanicError, (parents), runtime.gopanic, runtime.panicmem, runtime.sigpanic
			stack: NewStack(1 + skipStack + 3),
		}
		if stacker, ok := e.(StackTrimmer); ok {
			result.cause = stacker.TrimStack(result.stack)
		}
		return result
	} else {
		return ValueError{
			message: "panic",
			value:   v,
			// skip NewPanicError, (parents), runtime.gopanic, runtime.panicmem, runtime.sigpanic
			stack: NewStack(1 + skipStack + 3),
		}
	}
}
