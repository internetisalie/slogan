package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

const (
	LevelTrace = slog.Level(-8)
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

var (
	loggerLevels        = make(map[string]*slog.LevelVar)
	loggerLevelsLock    sync.Mutex
	defaultLoggerLevel  slog.Level = LevelInfo
)

func SetAllLoggerLevels(level slog.Level) {
	loggerLevelsLock.Lock()
	defer loggerLevelsLock.Unlock()

	defaultLoggerLevel = level
	for _, v := range loggerLevels {
		v.Set(level)
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
		existingLevel.Set(defaultLoggerLevel)
		loggerLevels[name] = existingLevel
	}

	return existingLevel
}

type LevelLogger struct {
	parent *slog.Logger
	level  slog.Level
}

type StdLogger interface {
	Print(values ...interface{})
	Printf(format string, values ...interface{})
	Println(values ...interface{})

	Fatal(values ...interface{})
	Fatalf(format string, values ...interface{})
	Fatalln(values ...interface{})

	Panic(values ...interface{})
	Panicf(format string, values ...interface{})
	Panicln(values ...interface{})
}

func NewLevelLogger(parent *slog.Logger, level slog.Level) *LevelLogger {
	return &LevelLogger{
		parent: parent,
		level:  level,
	}
}

func (l *LevelLogger) Printf(format string, values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprintf(format, values...))
}

func (l *LevelLogger) Print(values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprint(values...))
}

func (l *LevelLogger) Println(values ...interface{}) {
	l.parent.Log(context.Background(), l.level, fmt.Sprint(values...))
}

func (l *LevelLogger) Fatal(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Fatalf(format string, values ...interface{}) {
	msg := fmt.Sprintf(format, values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Fatalln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	os.Exit(1)
}

func (l *LevelLogger) Panic(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Panicf(format string, values ...interface{}) {
	msg := fmt.Sprintf(format, values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Panicln(values ...interface{}) {
	msg := fmt.Sprint(values...)
	l.parent.Log(context.Background(), LevelError, msg)
	panic(msg)
}

func (l *LevelLogger) Write(data []byte) (int, error) {
	l.Print(string(data))
	return len(data), nil
}
