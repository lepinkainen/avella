package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogHandler_Stderr(t *testing.T) {
	handler, closer, err := newLogHandler("", slog.LevelInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler == nil {
		t.Fatal("expected handler")
	}
	if closer != nil {
		t.Fatal("stderr handler should not return a closer")
	}
}

func TestNewLogHandler_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "avella.log")

	handler, closer, err := newLogHandler(path, slog.LevelInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closer == nil {
		t.Fatal("file handler must return a closer")
	}

	slog.New(handler).Info("hello", "key", "value")
	if closeErr := closer.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "hello") || !strings.Contains(content, "key=value") {
		t.Fatalf("unexpected log content: %q", content)
	}
	if strings.Contains(content, "\x1b[") {
		t.Fatalf("file log must not contain ANSI colour codes: %q", content)
	}
}

func TestNewLogHandler_ExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, closer, err := newLogHandler("~/logs/avella.log", slog.LevelInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = closer.Close() }()

	if _, statErr := os.Stat(filepath.Join(home, "logs")); statErr != nil {
		t.Fatalf("expected log directory under HOME: %v", statErr)
	}
}
