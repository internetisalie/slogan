package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// Log level constants define severity levels for logging.
const (
	// LevelTrace is a trace level below debug for very detailed diagnostics.
	LevelTrace = slog.Level(-8)
	// LevelDebug is the debug level for development-time diagnostics.
	LevelDebug = slog.LevelDebug
	// LevelInfo is the info level for general informational messages.
	LevelInfo = slog.LevelInfo
	// LevelWarn is the warn level for warning messages.
	LevelWarn = slog.LevelWarn
	// LevelError is the error level for error messages.
	LevelError = slog.LevelError
)

var (
	loggerLevels        = make(map[string]*slog.LevelVar)
	loggerLevelsLock    sync.Mutex
	defaultLoggerLevel  slog.Level = LevelInfo
)

// SetAllLoggerLevels sets the log level for all currently registered loggers
// and establishes the default level for loggers created in the future.
// This is useful for dynamic log level adjustment at runtime.
func SetAllLoggerLevels(level slog.Level) {
	loggerLevelsLock.Lock()
	defer loggerLevelsLock.Unlock()

	defaultLoggerLevel = level
	for _, v := range loggerLevels {
		v.Set(level)
	}
}

// SetLoggerLevel sets the log level for a specific named logger.
// If the logger hasn't been created yet, the level is recorded and applied
// when the logger is first instantiated.
func SetLoggerLevel(name string, level slog.Level) {
	GetLoggerLeveler(name).Set(level)
}

// GetLoggerLeveler retrieves or creates the level control variable for a named logger.
// Multiple calls with the same name return the same LevelVar, allowing centralized
// control of logger levels. If called before the logger is created, the level is
// stored and applied when the logger is instantiated.
func GetLoggerLeveler(name string) *slog.LevelVar {
	loggerLevelsLock.Lock()
	defer loggerLevelsLock.Unlock()

	existingLevel, ok := loggerLevels[name]
	if !ok {
		existingLevel = new(slog.LevelVar)
		existingLevel.Set(defaultLoggerLevel)
		loggerLevels[name] = existingLevel
	}

	return existingLevel
}

// LevelLogger wraps an slog.Logger and enforces logging at a specific level,
// regardless of the logger's configured level. This is useful for compatibility
// with code expecting io.Writer or the standard library log interface, while
// maintaining structured logging. LevelLogger implements StdLogger and io.Writer
// for maximum compatibility.
type LevelLogger struct {
	parent *slog.Logger
	level  slog.Level
}

// StdLogger defines the interface for standard library logging compatibility.
// It provides methods compatible with the standard log package, allowing
// LevelLogger to be used as a drop-in replacement for *log.Logger in many contexts.
type StdLogger interface {
	// Print logs values at the logger's configured level.
	Print(values ...interface{})
	// Printf logs a formatted message at the logger's configured level.
	Printf(format string, values ...interface{})
	// Println logs values at the logger's configured level with a newline.
	Println(values ...interface{})

	// Fatal logs an error-level message and exits the program with status 1.
	Fatal(values ...interface{})
	// Fatalf logs a formatted error message and exits the program with status 1.
	Fatalf(format string, values ...interface{})
	// Fatalln logs values at error level and exits the program with status 1.
	Fatalln(values ...interface{})

	// Panic logs an error-level message and panics.
	Panic(values ...interface{})
	// Panicf logs a formatted error message and panics.
	Panicf(format string, values ...interface{})
	// Panicln logs values at error level and panics.
	Panicln(values ...interface{})
}

// NewLevelLogger creates a new LevelLogger that logs at a specific level,
// independent of the parent logger's configured level. This is useful for
// creating specialized loggers (e.g., error-level only loggers) from a
// parent structured logger.
func NewLevelLogger(parent *slog.Logger, level slog.Level) *LevelLogger {
	return &LevelLogger{
		parent: parent,
		level:  level,
	}
}

// Print logs one or more values at the logger's configured level.
// Values are formatted using fmt.Sprint and combined into a single message.
func (l *LevelLogger) Print(values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprint(values...))
}

// Printf logs a formatted message at the logger's configured level.
// The format string is interpolated with the provided values using fmt.Sprintf.
func (l *LevelLogger) Printf(format string, values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprintf(format, values...))
}

// Println logs one or more values at the logger's configured level.
// Values are formatted and separated by spaces using fmt.Sprint.
func (l *LevelLogger) Println(values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprint(values...))
}

// Fatal logs one or more values at error level and terminates the program with exit code 1.
// The program exits immediately without running defer functions.
func (l *LevelLogger) Fatal(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

// Fatalf logs a formatted message at error level and terminates the program with exit code 1.
// The format string is interpolated with the provided values using fmt.Sprintf.
func (l *LevelLogger) Fatalf(format string, values ...interface{}) {
	msg := fmt.Sprintf(format, values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

// Fatalln logs one or more values at error level and terminates the program with exit code 1.
// Values are formatted and separated by spaces using fmt.Sprint.
func (l *LevelLogger) Fatalln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

// Panic logs one or more values at error level and panics with the resulting message.
// Values are formatted using fmt.Sprint and combined into a single panic message.
func (l *LevelLogger) Panic(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

// Panicf logs a formatted message at error level and panics with the resulting message.
// The format string is interpolated with the provided values using fmt.Sprintf.
func (l *LevelLogger) Panicf(format string, values ...interface{}) {
	msg := fmt.Sprintf(format, values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

// Panicln logs one or more values at error level and panics with the resulting message.
// Values are formatted and separated by spaces using fmt.Sprint.
func (l *LevelLogger) Panicln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

// Write logs the given byte data as a message at the logger's configured level.
// This implements io.Writer, allowing LevelLogger to be used with functions that
// write to io.Writer (e.g., http.Server.ErrorLog). The write always succeeds and
// returns the number of bytes written.
func (l *LevelLogger) Write(data []byte) (int, error) {
	l.Print(string(data))
	return len(data), nil
}
