package log

import (
	"log/slog"
)

type levelDecorator struct {
	err   error
	level slog.Level
}

func (d levelDecorator) Error() string     { return d.err.Error() }
func (d levelDecorator) Unwrap() error     { return d.err }
func (d levelDecorator) IsDecorator()      {}
func (d levelDecorator) Level() slog.Level { return d.level }
func (d levelDecorator) Message() string   { return Message(d.err) }

func (d levelDecorator) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String(ErrorMessageKey, d.err.Error()),
		slog.Any("level", d.level),
	)
}


type attrsDecorator struct {
	err   error
	attrs []slog.Attr
}

func (d attrsDecorator) Error() string         { return d.err.Error() }
func (d attrsDecorator) Unwrap() error         { return d.err }
func (d attrsDecorator) IsDecorator()          {}
func (d attrsDecorator) LogAttrs() []slog.Attr { return d.attrs }
func (d attrsDecorator) Message() string       { return Message(d.err) }

func (d attrsDecorator) LogValue() slog.Value {
	return slog.GroupValue(
		append([]slog.Attr{slog.String(ErrorMessageKey, d.err.Error())}, d.attrs...)...,
	)
}


type stackDecorator struct {
	err   error
	stack Stack
}

func (d stackDecorator) Error() string       { return d.err.Error() }
func (d stackDecorator) Unwrap() error       { return d.err }
func (d stackDecorator) IsDecorator()        {}
func (d stackDecorator) Stack() Stack        { return d.stack }
func (d stackDecorator) Message() string     { return Message(d.err) }

func (d stackDecorator) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String(ErrorMessageKey, d.err.Error()),
		slog.Any(ErrorStackKey, d.stack),
	)
}


// ErrorOption is a functional option for decorating or transforming errors.
// Used with Wrap, ErrWithLevel, ErrWithAttrs, and ErrWithStack to attach logging metadata to errors.
type ErrorOption func(error) error

// WithErrorLevel returns an ErrorOption that assigns a log level to an error.
// This causes the error to be logged at the specified level when processed by the logger.
// Example: Wrap(err, WithErrorLevel(LevelWarn)) logs the error at warning level instead of error.
func WithErrorLevel(level slog.Level) ErrorOption {
	return func(err error) error { return levelDecorator{err: err, level: level} }
}

// WithErrorAttrs returns an ErrorOption that attaches structured attributes to an error.
// These attributes are automatically included in log records when the decorated error is logged.
// Example: Wrap(err, WithErrorAttrs(slog.String("user_id", "123"))).
func WithErrorAttrs(attrs ...slog.Attr) ErrorOption {
	return func(err error) error { return attrsDecorator{err: err, attrs: attrs} }
}

// WithErrorStack returns an ErrorOption that captures a stack trace for the error.
// If the error already has a stack trace, it is not modified. Skips 3 frames
// (WithErrorStack, Wrap, and the caller) to point to the actual error source.
func WithErrorStack() ErrorOption {
	return func(err error) error {
		if _, ok := err.(Stacker); ok {
			return err
		}
		return stackDecorator{err: err, stack: NewStack(3)} // skip WithErrorStack.func, Wrap, caller
	}
}

// Wrap applies a series of ErrorOptions to an error, decorating it with logging metadata.
// Returns nil if the input error is nil. Options are applied in order, allowing composition
// like: Wrap(err, WithErrorLevel(LevelWarn), WithErrorAttrs(...), WithErrorStack()).
func Wrap(err error, opts ...ErrorOption) error {
	if err == nil {
		return nil
	}

	for _, opt := range opts {
		err = opt(err)
	}

	return err
}

// ErrWithLevel is a convenience function that wraps an error with a custom log level.
// Equivalent to Wrap(err, WithErrorLevel(level)). Returns nil if the input error is nil.
func ErrWithLevel(err error, level slog.Level) error {
	return Wrap(err, WithErrorLevel(level))
}

// ErrWithAttrs is a convenience function that wraps an error with custom attributes.
// Equivalent to Wrap(err, WithErrorAttrs(attrs...)). Returns nil if the input error is nil.
func ErrWithAttrs(err error, attrs ...slog.Attr) error {
	return Wrap(err, WithErrorAttrs(attrs...))
}

// ErrWithStack is a convenience function that wraps an error with a captured stack trace.
// Equivalent to Wrap(err, WithErrorStack()). Returns nil if the input error is nil.
func ErrWithStack(err error) error {
	return Wrap(err, WithErrorStack())
}
