package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessTrackerSkipsUnchangedProcessedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "example.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tracker := newProcessTracker()
	if !tracker.ShouldProcess(path, info, false) {
		t.Fatal("expected new file to be processed")
	}

	tracker.MarkProcessed(path, info, false)
	if tracker.ShouldProcess(path, info, false) {
		t.Fatal("expected unchanged file to be skipped after successful processing")
	}
}

func TestProcessTrackerAllowsRealRunAfterDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "example.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tracker := newProcessTracker()
	tracker.MarkProcessed(path, info, true)

	if tracker.ShouldProcess(path, info, true) {
		t.Fatal("expected unchanged file to be skipped during repeated dry-run sweeps")
	}
	if !tracker.ShouldProcess(path, info, false) {
		t.Fatal("expected unchanged file to run for real after dry-run is disabled")
	}
}

func TestProcessTrackerReprocessesChangedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "example.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tracker := newProcessTracker()
	tracker.MarkProcessed(path, info, false)

	newTime := info.ModTime().Add(2 * time.Minute)
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	updatedInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !tracker.ShouldProcess(path, updatedInfo, false) {
		t.Fatal("expected changed file to be processed again")
	}
}

func TestProcessTrackerPruneRemovesDeletedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "example.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tracker := newProcessTracker()
	tracker.MarkProcessed(path, info, false)

	if tracker.ShouldProcess(path, info, false) {
		t.Fatal("expected file to be skipped before removal")
	}

	os.Remove(path)
	tracker.Prune()

	// Re-create the file so ShouldProcess can be called with valid info.
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !tracker.ShouldProcess(path, info, false) {
		t.Fatal("expected pruned file to be eligible for processing again")
	}
}
