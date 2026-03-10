package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	action := &ExecAction{Command: "echo"}
	if err := action.Execute(context.Background(), path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecActionNonexistentCommand(t *testing.T) {
	action := &ExecAction{Command: "/nonexistent/command"}
	err := action.Execute(context.Background(), "/tmp/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}
