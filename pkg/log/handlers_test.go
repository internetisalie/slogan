package log

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mapEnv struct {
	vars map[string]string
}

func (m *mapEnv) Getenv(key string) string {
	return m.vars[key]
}

func (m *mapEnv) Setenv(key, value string) error {
	if m.vars == nil {
		m.vars = make(map[string]string)
	}
	m.vars[key] = value
	return nil
}

func TestNewConsoleHandler_InvalidFormat(t *testing.T) {
	oldEnv := env
	defer func() { env = oldEnv }()

	m := &mapEnv{
		vars: map[string]string{
			"LOG_FORMAT": "invalid-format-123",
		},
	}
	env = m

	// This should trigger the fallback and log an error to StandardLogger
	h := NewConsoleHandler(&slog.HandlerOptions{}, nil, "")
	assert.NotNil(t, h)

	// It should have reset the env var in our mock
	assert.Equal(t, FormatDefault, m.vars["LOG_FORMAT"])
}

func TestRegisterRemoteHandlerFactory(t *testing.T) {
	called := false
	RegisterRemoteHandlerFactory(func() slog.Handler {
		called = true
		return slog.Default().Handler()
	})

	h := NewRemoteHandler()
	assert.NotNil(t, h)
	// Trigger the proxy
	h.Enabled(context.Background(), slog.LevelInfo)
	assert.True(t, called)
}
