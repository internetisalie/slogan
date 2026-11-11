package log

import (
	"bytes"
	"context"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restoreRootAfterTest puts the root destination back to unconfigured and the slog default
// logger and the standard log package back to what they were, when the test ends: restoring
// the slog default does not undo the rerouting of the log package, so its writer, flags and
// prefix are put back too. It also unregisters the remote handler factory for the test: with
// one registered, the remote handler of every logger writes a second copy of each record to
// the slog default logger, which ConfigureRoot points at the test's buffer.
func restoreRootAfterTest(t *testing.T) {
	t.Helper()

	t.Setenv("LOG_FORMAT", FormatLogFmt)
	previous := slog.Default()
	previousWriter, previousFlags, previousPrefix := stdlog.Writer(), stdlog.Flags(), stdlog.Prefix()
	previousFactory := registeredRemoteHandlerFactory
	registeredRemoteHandlerFactory = nil
	t.Cleanup(func() {
		rootDest.Store(nil)
		slog.SetDefault(previous)
		stdlog.SetOutput(previousWriter)
		stdlog.SetFlags(previousFlags)
		stdlog.SetPrefix(previousPrefix)
		registeredRemoteHandlerFactory = previousFactory
	})
}

func TestRestoreRootAfterTestPutsTheStandardLogPackageBack(t *testing.T) {
	writer, flags, prefix := stdlog.Writer(), stdlog.Flags(), stdlog.Prefix()

	t.Run("configures the root", func(t *testing.T) {
		restoreRootAfterTest(t)
		ConfigureRoot(&bytes.Buffer{})
		require.NotSame(t, writer, stdlog.Writer())
	})

	assert.Same(t, writer, stdlog.Writer())
	assert.Equal(t, flags, stdlog.Flags())
	assert.Equal(t, prefix, stdlog.Prefix())
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
	assert.Regexp(t, `\sa=1 g\.b=2 g\.c=3\s*$`, buf.String())
}

func TestPackageLoggerLevelGatesRecordsToRoot(t *testing.T) {
	restoreRootAfterTest(t)
	info := NewLoggerWithOpts("info", WithLeveler(slog.LevelInfo))
	debug := NewLoggerWithOpts("debug", WithLeveler(slog.LevelDebug))

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	info.Log(context.Background(), LevelTrace, "info trace record")
	info.Debug("info debug record")
	info.Info("info info record")
	debug.Debug("debug debug record")

	assert.NotContains(t, buf.String(), "info trace record")
	assert.NotContains(t, buf.String(), "info debug record")
	assert.Contains(t, buf.String(), `msg="info info record"`)
	assert.Contains(t, buf.String(), `msg="debug debug record"`)
}

func TestPackageLoggerWithFormatKeepsItOnTheRootPath(t *testing.T) {
	restoreRootAfterTest(t)
	logger := NewLoggerWithOpts("formatted", WithFormat(FormatJson))

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	logger.Info("formatted record")

	assert.True(t, strings.HasPrefix(buf.String(), "{"), buf.String())
	assert.Contains(t, buf.String(), `"msg":"formatted record"`)
}

func TestSiblingLoggersDerivedFromOneParentCarryOnlyTheirOwnAttrs(t *testing.T) {
	restoreRootAfterTest(t)
	parent := NewLoggerWithOpts("parent").With("a", "1").With("b", "2")
	first := parent.With("first_attr", "x")
	second := parent.With("second_attr", "y")

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	first.Info("first record")
	second.Info("second record")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "first_attr=x")
	assert.NotContains(t, lines[0], "second_attr")
	assert.Contains(t, lines[1], "second_attr=y")
	assert.NotContains(t, lines[1], "first_attr")
}

func TestDerivedLoggersWriteConcurrentlyToOneRootWriter(t *testing.T) {
	restoreRootAfterTest(t)
	parent := NewLoggerWithOpts("concurrent").With("a", "1")
	first := parent.With("which", "first")
	second := parent.With("which", "second")

	buf := &bytes.Buffer{}
	ConfigureRoot(buf)
	const records = 100
	var wg sync.WaitGroup
	for _, logger := range []*slog.Logger{first, second, slog.Default()} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range records {
				logger.Info("concurrent record")
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 3*records, strings.Count(buf.String(), `msg="concurrent record"`))
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
	require.NoError(t, reader.Close())
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
	slog.Default().Debug("default debug record")
	slog.Default().Info("default record")

	assert.NotContains(t, buf.String(), "default debug record")
	assert.Contains(t, buf.String(), `msg="default record"`)
}
