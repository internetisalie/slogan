package log

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/samber/lo"
)

const (
	LevelTrace = slog.Level(-8)
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

var (
	loggerLevels     = make(map[string]*slog.LevelVar)
	loggerLevelsLock sync.Mutex
)

func SetAllLoggerLevels(level slog.Level) {
	names := func() []string {
		loggerLevelsLock.Lock()
		defer loggerLevelsLock.Unlock()

		return lo.Keys(loggerLevels)
	}()

	for _, name := range names {
		SetLoggerLevel(name, level)
	}
}

func SetLoggerLevel(name string, level slog.Level) {
	GetLoggerLeveler(name).Set(level)
}

func GetLoggerLeveler(name string) *slog.LevelVar {
	loggerLevelsLock.Lock()
	defer loggerLevelsLock.Unlock()

	existingLevel, ok := loggerLevels[name]
	if !ok {
		existingLevel = new(slog.LevelVar)
		loggerLevels[name] = existingLevel
	}

	return existingLevel
}

type LevelLogger struct {
	parent FormattingLogger
	level  slog.Level
}

type StdLogger interface {
	Print(...interface{})
	Printf(string, ...interface{})
	Println(...interface{})

	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Fatalln(...interface{})

	Panic(...interface{})
	Panicf(string, ...interface{})
	Panicln(...interface{})
}

func NewLevelLogger(logger FormattingLogger, level slog.Level) StdLogger {
	return &LevelLogger{
		parent: logger,
		level:  level,
	}
}

func (l *LevelLogger) Printf(template string, values ...interface{}) {
	msg := fmt.Sprintf(template, values...)
	l.parent.Log(nil, l.level, msg)
}

func (l *LevelLogger) Print(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, l.level, msg)
}

func (l *LevelLogger) Println(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, l.level, msg)
}

func (l *LevelLogger) Fatal(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Fatalf(template string, values ...interface{}) {
	msg := fmt.Sprintf(template, values...)
	l.parent.Log(nil, LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Fatalln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Panic(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Panicf(template string, values ...interface{}) {
	msg := fmt.Sprintf(template, values...)
	l.parent.Log(nil, LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Panicln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(nil, LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Write(data []byte) (int, error) {
	l.Print(string(data))
	return len(data), nil
}
