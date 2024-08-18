// Copyright © 2022, Cisco Systems Inc.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file or at https://opensource.org/licenses/MIT.

package log

import (
	"bufio"
	"io"
	"log"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/samber/slog-multi"
)

const FrameworkRootLogger = "glimmer"

const (
	FormatTimestampMicro = "2006-01-02T15:04:05.000000Z07:00"
	FormatTimestampHuman = "15:04:05.000000"
)

var (
	standardLogger     *slog.Logger
	standardLoggerOnce sync.Once
)

func StandardLogger() *slog.Logger {
	standardLoggerOnce.Do(func() {
		standardLogger = NewLogger(FrameworkRootLogger)
	})

	return standardLogger
}

// LoggingLogger returns a logger for use by logging handler implementations' internal logging
func LoggingLogger() *slog.Logger {
	handler := NewConsoleHandler(&slog.HandlerOptions{
		Level:       LevelTrace,
		ReplaceAttr: NewTimestampAttrReplacer(nil, FormatTimestampMicro).ReplaceAttr,
	})
	return slog.New(handler)
}

var regexpVersionSuffix = regexp.MustCompile(`^v\d+$`)

var packageLoggerPrefixes = []string{
	"code.internetisalie.net/",
}

func RegisterPackageLoggerPrefix(prefix string) {
	packageLoggerPrefixes = append(packageLoggerPrefixes, prefix)
}

func PackageLoggerName(skip int) string {
	pc, _, _, _ := runtime.Caller(skip)
	longFunc := runtime.FuncForPC(pc).Name()
	longFuncSuffixIndex := strings.LastIndex(longFunc, ".")
	longPackage := longFunc[:longFuncSuffixIndex]

	// trim the first known prefixes
	shortPackage := longPackage
	for _, packagePrefix := range packageLoggerPrefixes {
		if strings.HasPrefix(shortPackage, packagePrefix) {
			shortPackage = strings.TrimPrefix(longPackage, packagePrefix)
			break
		}
	}

	// trim any module version suffix
	packageParts := strings.Split(shortPackage, "/")
	lastPackagePart := len(packageParts) - 1
	if regexpVersionSuffix.MatchString(packageParts[lastPackagePart]) {
		packageParts = packageParts[:lastPackagePart]
	}

	return strings.Join(packageParts, ".")
}

func NewLogger(name string, attrs ...slog.Attr) *slog.Logger {
	var replacer AttrReplacer

	// Inject our replacer middleware
	replacer = NewSanitizerAttrReplacer(replacer)
	replacer = NewTimestampAttrReplacer(replacer, FormatTimestampMicro)

	ho := slog.HandlerOptions{
		Level:       GetLoggerLeveler(name),
		ReplaceAttr: replacer.ReplaceAttr,
	}

	console := NewConsoleHandler(&ho)
	remote := NewRemoteHandler()

	// inject our handler middleware
	handler := slogmulti.
		Pipe(NewErrorAttrsMiddleware()).
		Handler(slogmulti.Fanout(
			console,
			remote,
		))

	logger := slog.New(handler)

	// Apply logger name
	logger = logger.With(slog.Attr{
		Key:   LoggerKey,
		Value: slog.StringValue(name),
	})

	// Apply passed attributes
	for _, attr := range attrs {
		logger = logger.With(attr)
	}

	return logger
}

type goLogWriter struct {
	*io.PipeWriter
	scanner *bufio.Scanner
	logger  StdLogger
}

func (g goLogWriter) Pump() {
	for g.scanner.Scan() {
		g.logger.Print(g.scanner.Text())
	}
}

func NewGoLogger(parent FormattingLogger, level slog.Level) *log.Logger {
	r, w := io.Pipe()
	s := bufio.NewScanner(r)
	logger := NewLevelLogger(parent, level)
	out := goLogWriter{PipeWriter: w, scanner: s, logger: logger}
	go out.Pump()
	return log.New(out, "", 0)
}
