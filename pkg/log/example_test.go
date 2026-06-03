package log_test

import (
	"errors"
	"log/slog"
	"os"

	"code.internetisalie.net/slogan/pkg/log"
)

func ExampleNewLoggerWithOpts() {
	// Create a logger with custom options and no time for deterministic output
	logger := log.NewLoggerWithOpts("example",
		log.WithFormat(log.FormatLogFmt),
		log.WithAttrs(slog.String("version", "1.0.0")),
		log.WithTimeFormat(""), // Suppress time key entirely
		log.WithWriter(os.Stdout),
	)

	logger.Info("application started")
	// Output:
	// time="" level=INFO msg="application started" logger=example version=1.0.0
}

func ExampleErrWithLevel() {
	logger := log.NewLoggerWithOpts("example",
		log.WithFormat(log.FormatLogFmt),
		log.WithTimeFormat(""), // Suppress time key entirely
		log.WithWriter(os.Stdout),
	)

	// An error occurred that we consider critical
	err := errors.New("database connection lost")
	// Use Wrap with WithErrorLevel
	err = log.Wrap(err, log.WithErrorLevel(slog.LevelError))

	// Even though we log at INFO level, the record is promoted to ERROR
	logger.Info("could not reach database", slog.Any(log.ErrorKey, err))
	// Output:
	// time="" level=ERROR msg="could not reach database" logger=example error.stack="Root Cause: database connection lost" error.message="database connection lost"
}

func ExampleErrWithAttrs() {
	logger := log.NewLoggerWithOpts("example",
		log.WithFormat(log.FormatLogFmt),
		log.WithTimeFormat(""), // Suppress time key entirely
		log.WithWriter(os.Stdout),
	)

	// Attach context-specific attributes to the error itself
	err := errors.New("permission denied")
	// Use Wrap with WithErrorAttrs
	err = log.Wrap(err,
		log.WithErrorAttrs(slog.String("user_id", "456")),
	)

	// Attributes are hoisted to the top level of the log record automatically
	logger.Warn("access failure", slog.Any(log.ErrorKey, err))
	// Output:
	// time="" level=WARN msg="access failure" logger=example user_id=456 error.stack="Root Cause: permission denied" error.message="permission denied"
}

func Example_joinedErrors() {
	logger := log.NewLoggerWithOpts("example",
		log.WithFormat(log.FormatLogFmt),
		log.WithTimeFormat(""), // Suppress time key entirely
		log.WithWriter(os.Stdout),
	)

	// Errors decorated with different options
	err1 := log.Wrap(errors.New("disk full"), log.WithErrorLevel(slog.LevelError))
	err2 := log.Wrap(errors.New("cleanup failed"), log.WithErrorAttrs(slog.String("retry", "true")))

	joined := errors.Join(err1, err2)

	// Result: level=ERROR, retry=true
	logger.Info("multiple failures", slog.Any(log.ErrorKey, joined))
	// Output:
	// time="" level=ERROR msg="multiple failures" logger=example retry=true error.stack="Root Cause: disk full\ncleanup failed\nCaused: disk full\nCaused: cleanup failed" error.message="disk full\ncleanup failed"
}

func ExampleWrap() {
	logger := log.NewLoggerWithOpts("example",
		log.WithFormat(log.FormatLogFmt),
		log.WithTimeFormat(""),
		log.WithWriter(os.Stdout),
	)

	err := errors.New("failed to process request")
	// Chain multiple decorations in a single Wrap call
	err = log.Wrap(err,
		log.WithErrorLevel(slog.LevelError),
		log.WithErrorAttrs(slog.String("request_id", "123")),
	)

	logger.Error("handler error", slog.Any(log.ErrorKey, err))
	// Output:
	// time="" level=ERROR msg="handler error" logger=example request_id=123 error.stack="Root Cause: failed to process request" error.message="failed to process request"
}

func Example_customSanitizer() {
	// Define a custom sanitizer for sensitive data
	ssnSanitizer := log.SanitizerFunc(func(a slog.Attr) slog.Value {
		if a.Key == "ssn" {
			return slog.StringValue("***-**-****")
		}
		return a.Value
	})

	logger := log.NewLoggerWithOpts("secure-app",
		log.WithSanitizers(ssnSanitizer),
		log.WithFormat(log.FormatLogFmt),
		log.WithTimeFormat(""), // Suppress time key entirely
		log.WithWriter(os.Stdout),
	)

	logger.Info("user profile updated", slog.String("ssn", "123-45-6789"))
	// Output:
	// time="" level=INFO msg="user profile updated" logger=secure-app ssn=***-**-****
}
