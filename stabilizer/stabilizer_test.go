package stabilizer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file.txt", false},
		{"file.part", true},
		{"file.tmp", true},
		{"file.crdownload", true},
		{"file.download", true},
		{"file.PART", true},
		{"file.go", false},
		{"noext", false},
	}

	for _, tt := range tests {
		if got := ShouldSkip(tt.path); got != tt.want {
			t.Errorf("ShouldSkip(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestWaitStableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	err := WaitStable(ctx, path, 10*time.Millisecond, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitStableGrowingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "growing.txt")
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Append to the file a few times, then stop
	go func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		for i := 0; i < 3; i++ {
			time.Sleep(15 * time.Millisecond)
			f.WriteString("more")
		}
	}()

	ctx := context.Background()
	err := WaitStable(ctx, path, 20*time.Millisecond, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitStableFileRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doomed.txt")
	if err := os.WriteFile(path, []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Remove the file after a short delay
	go func() {
		time.Sleep(15 * time.Millisecond)
		os.Remove(path)
	}()

	ctx := context.Background()
	err := WaitStable(ctx, path, 10*time.Millisecond, 10)
	if !errors.Is(err, ErrFileRemoved) {
		t.Fatalf("expected ErrFileRemoved, got: %v", err)
	}
}

func TestWaitStableContextCancelled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cancel.txt")
	if err := os.WriteFile(path, []byte("wait"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()

	// Use a long interval so cancellation happens first
	err := WaitStable(ctx, path, 1*time.Second, 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}
