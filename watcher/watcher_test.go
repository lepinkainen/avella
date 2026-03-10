package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	w, err := New([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Use fast stabilization for tests
	w.stableInterval = 10 * time.Millisecond
	w.stableChecks = 2

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := w.Start(ctx)

	// Create a file after watcher is started
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		if got != path {
			t.Errorf("got path %q, want %q", got, path)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for file event")
	}
}

func TestWatcherSkipsTmpFiles(t *testing.T) {
	dir := t.TempDir()

	w, err := New([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	w.stableInterval = 10 * time.Millisecond
	w.stableChecks = 1

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch := w.Start(ctx)

	// Create a .tmp file — should be skipped
	tmpPath := filepath.Join(dir, "file.tmp")
	if err := os.WriteFile(tmpPath, []byte("tmp"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		t.Fatalf("expected no event for .tmp file, got %q", got)
	case <-ctx.Done():
		// Expected: no event received before timeout
	}
}

func TestWatcherContextCancel(t *testing.T) {
	dir := t.TempDir()

	w, err := New([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := w.Start(ctx)

	cancel()

	// Channel should close
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}
