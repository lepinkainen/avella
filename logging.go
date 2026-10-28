package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lepinkainen/avella/internal/pathutil"
	"github.com/lepinkainen/humanlog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Log file rotation defaults. Avella runs under launchd with KeepAlive, so the
// log would otherwise grow without bound.
const (
	logMaxSizeMB  = 10
	logMaxBackups = 3
	logMaxAgeDays = 30
)

// newLogHandler builds the slog handler for the daemon. When path is empty the
// handler writes coloured output to stderr. Otherwise it writes plain text to a
// size-rotated file at path (creating parent directories as needed) and returns
// a closer the caller must invoke on shutdown.
func newLogHandler(path string, level slog.Level) (slog.Handler, io.Closer, error) {
	if path == "" {
		return humanlog.NewHandler(os.Stderr, &humanlog.Options{Level: level}), nil, nil
	}

	expanded, err := pathutil.ExpandHome(path)
	if err != nil {
		return nil, nil, fmt.Errorf("expand log path: %w", err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(expanded), 0o750); mkErr != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", mkErr)
	}

	writer := &lumberjack.Logger{
		Filename:   expanded,
		MaxSize:    logMaxSizeMB,
		MaxBackups: logMaxBackups,
		MaxAge:     logMaxAgeDays,
	}
	handler := humanlog.NewHandler(writer, &humanlog.Options{
		Level:        level,
		DisableColor: true,
		TimeFormat:   "2006-01-02 15:04:05",
	})
	return handler, writer, nil
}
