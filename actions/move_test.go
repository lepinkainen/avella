package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveAction(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "dest")
	action := &MoveAction{Dest: destDir}

	if err := action.Execute(context.Background(), src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file still exists after move")
	}

	// Dest should exist with same content
	dest := filepath.Join(destDir, "source.txt")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("dest content = %q, want %q", string(data), "hello")
	}
}

func TestMoveActionCreatesDestDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "a", "b", "c")
	action := &MoveAction{Dest: destDir}

	if err := action.Execute(context.Background(), src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest := filepath.Join(destDir, "file.txt")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("dest file does not exist: %v", err)
	}
}

func TestMoveActionOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(src, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existing := filepath.Join(destDir, "new.txt")
	if err := os.WriteFile(existing, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	action := &MoveAction{Dest: destDir}
	if err := action.Execute(context.Background(), src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("dest content = %q, want %q", string(data), "new content")
	}
}
