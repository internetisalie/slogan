package log

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevels(t *testing.T) {
	defer SetAllLoggerLevels(LevelInfo)
	
	SetLoggerLevel("test-level", slog.LevelDebug)
	l := GetLoggerLeveler("test-level")
	assert.Equal(t, slog.LevelDebug, l.Level())
	
	SetAllLoggerLevels(slog.LevelWarn)
	l = GetLoggerLeveler("any")
	assert.Equal(t, slog.LevelWarn, l.Level())
}

func TestLevelLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	parent := NewLoggerWithOpts("test", WithWriter(buf), WithFormat(FormatLogFmt), WithTimeFormat(""))
	logger := NewLevelLogger(parent, slog.LevelInfo)
	
	logger.Print("hello")
	assert.Contains(t, buf.String(), "msg=hello")
	
	buf.Reset()
	logger.Printf("formatted %s", "msg")
	assert.Contains(t, buf.String(), "msg=\"formatted msg\"")
	
	buf.Reset()
	logger.Println("line")
	assert.Contains(t, buf.String(), "msg=line")
	
	// Test Panic
	assert.Panics(t, func() {
		logger.Panic("panic msg")
	})
	assert.Contains(t, buf.String(), "msg=\"panic msg\"")

	assert.Panics(t, func() {
		logger.Panicf("panic %s", "fmt")
	})
	assert.Contains(t, buf.String(), "msg=\"panic fmt\"")

	assert.Panics(t, func() {
		logger.Panicln("panic line")
	})
	assert.Contains(t, buf.String(), "msg=\"panic line\"")

	// Test Write
	buf.Reset()
	n, err := logger.Write([]byte("write msg"))
	assert.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Contains(t, buf.String(), "msg=\"write msg\"")
}
