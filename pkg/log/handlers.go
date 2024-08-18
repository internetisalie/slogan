// Copyright © 2022, Cisco Systems Inc.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file or at https://opensource.org/licenses/MIT.

package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/samber/mo"

	"code.internetisalie.net/slogan/pkg/errors"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	slogmulti "github.com/samber/slog-multi"
)

const (
	FormatLogFmt  = "logfmt"
	FormatJson    = "json"
	FormatHuman   = "human"
	FormatTint    = "tint"
	FormatDefault = FormatLogFmt
	FormatPlain   = "plain"
)

var TerminalFormat string

func NewConsoleHandler(ho *slog.HandlerOptions) slog.Handler {
	format := FormatDefault
	if isatty.IsTerminal(os.Stdout.Fd()) {
		format = mo.EmptyableToOption(TerminalFormat).OrElse(FormatTint)
	}

	if requestedFormat := os.Getenv("LOG_FORMAT"); requestedFormat != "" {
		requestedFormat = strings.ToLower(requestedFormat)

		switch requestedFormat {
		case FormatJson, FormatLogFmt, FormatHuman, FormatTint, FormatPlain:
			format = requestedFormat
		default:
			_ = os.Setenv("LOG_FORMAT", format)
			StandardLogger().Error(fmt.Sprintf(
				"Unknown log format %q.  Defaulting to %q",
				requestedFormat, format))
		}
	}

	consoleWriter := os.Stdout

	var console slog.Handler
	switch format {
	case FormatJson:
		console = slog.NewJSONHandler(consoleWriter, ho)
	case FormatLogFmt:
		console = slog.NewTextHandler(consoleWriter, ho)
	case FormatTint:
		console = tint.NewHandler(consoleWriter, &tint.Options{
			AddSource:   ho.AddSource,
			Level:       ho.Level,
			ReplaceAttr: ho.ReplaceAttr,
			TimeFormat:  "15:04:05.000000",
			NoColor:     !isatty.IsTerminal(consoleWriter.Fd()),
		})
	case FormatHuman:
		console = NewHumanHandler(consoleWriter, ho)
	case FormatPlain:
		console = NewPlainHandler(consoleWriter, ho)
	}
	return console
}

type RemoteProxyHandler struct {
	attrs  []slog.Attr // tree of saved attributes
	groups []string    // current group path names
}

func (h *RemoteProxyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if n := h.leafHandler(); n != nil {
		return n.Enabled(ctx, level)
	}
	return false
}

func (h *RemoteProxyHandler) Handle(ctx context.Context, record slog.Record) error {
	if n := h.leafHandler(); n != nil {
		return n.Handle(ctx, record)
	}
	return nil
}

func (h *RemoteProxyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if n := h.leafHandler(); n != nil {
		return n.WithAttrs(attrs)
	}
	if len(attrs) == 0 {
		return h
	}

	return &RemoteProxyHandler{
		attrs:  SetAttrsAtPath(h.attrs, h.groups, attrs),
		groups: h.groups,
	}
}

func (h *RemoteProxyHandler) WithGroup(name string) slog.Handler {
	if n := h.leafHandler(); n != nil {
		return n.WithGroup(name)
	}

	if name == "" {
		return h
	}

	return &RemoteProxyHandler{
		attrs:  h.attrs,
		groups: AddGroup(h.groups, name),
	}
}

func (h *RemoteProxyHandler) leafHandler() slog.Handler {
	if registeredRemoteHandlerFactory == nil {
		return nil
	}

	result := registeredRemoteHandlerFactory()
	result = result.WithAttrs(h.attrs)
	for _, group := range h.groups {
		result = result.WithGroup(group)
	}
	return result
}

type RemoteHandlerFactory func() slog.Handler

var registeredRemoteHandlerFactory RemoteHandlerFactory

func RegisterRemoteHandlerFactory(factory RemoteHandlerFactory) {
	registeredRemoteHandlerFactory = factory
}

func NewRemoteHandler() slog.Handler {
	return new(RemoteProxyHandler)
}

func NewErrorAttrsMiddleware() slogmulti.Middleware {
	return slogmulti.NewWithAttrsInlineMiddleware(func(attrs []slog.Attr, next func([]slog.Attr) slog.Handler) slog.Handler {
		// Extract error
		var err error
		var ia int
		for i, a := range attrs {
			if a.Key == ErrorKey {
				ia = i
				err, _ = a.Value.Any().(error)
				break
			}
		}

		if err == nil {
			return next(attrs)
		}

		attrs[ia] = Attr(ErrorKey, err)

		backTraceBytes := errors.BackTrace(err)
		attrs = MergeAttrs(attrs, []slog.Attr{
			{
				Key: ErrorKey,
				Value: slog.GroupValue(
					slog.String(
						ErrorBacktraceKey,
						strings.TrimSpace(string(backTraceBytes)),
					),
					slog.String(
						ErrorTextKey,
						err.Error(),
					),
				),
			},
		})

		return next(attrs)
	})
}
