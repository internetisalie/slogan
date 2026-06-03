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


type ErrorOption func(error) error

func WithErrorLevel(level slog.Level) ErrorOption {
	return func(err error) error { return levelDecorator{err: err, level: level} }
}

func WithErrorAttrs(attrs ...slog.Attr) ErrorOption {
	return func(err error) error { return attrsDecorator{err: err, attrs: attrs} }
}

func WithErrorStack() ErrorOption {
	return func(err error) error {
		if _, ok := err.(Stacker); ok {
			return err
		}
		return stackDecorator{err: err, stack: NewStack(3)} // skip WithErrorStack.func, Wrap, caller
	}
}

func Wrap(err error, opts ...ErrorOption) error {
	if err == nil {
		return nil
	}

	for _, opt := range opts {
		err = opt(err)
	}

	return err
}

func ErrWithLevel(err error, level slog.Level) error {
	return Wrap(err, WithErrorLevel(level))
}

func ErrWithAttrs(err error, attrs ...slog.Attr) error {
	return Wrap(err, WithErrorAttrs(attrs...))
}

func ErrWithStack(err error) error {
	return Wrap(err, WithErrorStack())
}
