package log

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restoreRootAfterTest puts the root destination back to unconfigured and the slog default
// logger back to what it was, when the test ends. It also unregisters the remote handler
// factory for the test: with one registered, the remote handler of every logger writes a
// second copy of each record to the slog default logger, which ConfigureRoot points at the
// test's buffer.
func restoreRootAfterTest(t *testing.T) {
	t.Helper()

	t.Setenv("LOG_FORMAT", FormatLogFmt)
	previous := slog.Default()
	previousFactory := registeredRemoteHandlerFactory
	registeredRemoteHandlerFactory = nil
	t.Cleanup(func() {
		rootDest.Store(nil)
		slog.SetDefault(previous)
		registeredRemoteHandlerFactory = previousFactory
	})
}

func TestPackageLoggerCreatedBeforeConfigureRootWritesToRoot(t *testing.T) {
	restoreRootAfterTest(t)
	logger := NewLogger("early")

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	logger.Info("early record")

	assert.Contains(t, buf.String(), `msg="early record"`)
}

func TestPackageLoggerCreatedAfterConfigureRootWritesToRoot(t *testing.T) {
	restoreRootAfterTest(t)

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	logger := NewLogger("late")
	logger.Info("late record")

	assert.Contains(t, buf.String(), `msg="late record"`)
}

func TestDerivedLoggerCarriesAttrsAndGroupInOrderToRoot(t *testing.T) {
	restoreRootAfterTest(t)
	logger := NewLogger("derived")

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	logger.With("a", "1").WithGroup("g").With("b", "2").Info("derived record", "c", "3")

	assert.Contains(t, buf.String(), `msg="derived record"`)
	assert.Contains(t, buf.String(), "a=1 g.b=2 g.c=3")
}

func TestConfigureRootDestinationChangeMovesSubsequentRecords(t *testing.T) {
	restoreRootAfterTest(t)
	logger := NewLogger("moving")

	first := &bytes.Buffer{}
	second := &bytes.Buffer{}
	ConfigureRoot(first)
	logger.Info("before change")
	ConfigureRoot(second)
	logger.Info("after change")

	assert.Contains(t, first.String(), `msg="before change"`)
	assert.NotContains(t, first.String(), "after change")
	assert.Contains(t, second.String(), `msg="after change"`)
	assert.NotContains(t, second.String(), "before change")
}

func TestPackageLoggerWritesToStdoutBeforeConfigureRoot(t *testing.T) {
	restoreRootAfterTest(t)

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	previous := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = previous })

	NewLogger("unconfigured").Info("stdout record")

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(output), `msg="stdout record"`)
}

func TestLoggerWithExplicitWriterKeepsItAfterConfigureRoot(t *testing.T) {
	restoreRootAfterTest(t)

	explicit := &bytes.Buffer{}
	logger := NewLoggerWithOpts("explicit", WithWriter(explicit))

	root := &bytes.Buffer{}
	ConfigureRoot(root)
	logger.Info("explicit record")

	assert.Contains(t, explicit.String(), `msg="explicit record"`)
	assert.Empty(t, root.String())
}

func TestDefaultSlogLoggerWritesToRootAfterConfigureRoot(t *testing.T) {
	restoreRootAfterTest(t)

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	slog.Default().Info("default record")

	assert.Contains(t, buf.String(), `msg="default record"`)
}
