package main

import (
	"errors"
	"os"
	"sync"
	"time"
)

// SweepInterval controls how often watched directories are rescanned for files
// that may have aged into eligibility (for example due to min_age).
const SweepInterval = 15 * time.Minute

type fileFingerprint struct {
	size    int64
	modTime time.Time
}

type processedRecord struct {
	fingerprint fileFingerprint
	dryRun      bool
}

type processTracker struct {
	mu        sync.Mutex
	processed map[string]processedRecord
}

func newProcessTracker() *processTracker {
	return &processTracker{processed: make(map[string]processedRecord)}
}

func fingerprintFor(info os.FileInfo) fileFingerprint {
	return fileFingerprint{size: info.Size(), modTime: info.ModTime()}
}

// ShouldProcess reports whether an unchanged file should be processed again.
// Files already processed successfully are skipped until they change. Dry-run
// executions are only considered final for subsequent dry-run sweeps; turning
// dry-run off allows the unchanged file to be executed for real.
func (t *processTracker) ShouldProcess(path string, info os.FileInfo, dryRun bool) bool {
	if t == nil {
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.processed[path]
	if !ok {
		return true
	}

	fp := fingerprintFor(info)
	if rec.fingerprint != fp {
		return true
	}

	return rec.dryRun && !dryRun
}

func (t *processTracker) MarkProcessed(path string, info os.FileInfo, dryRun bool) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.processed[path] = processedRecord{fingerprint: fingerprintFor(info), dryRun: dryRun}
}

// Prune removes entries for files that no longer exist on disk.
func (t *processTracker) Prune() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for path := range t.processed {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			delete(t.processed, path)
		}
	}
}

func (t *processTracker) Reset() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.processed = make(map[string]processedRecord)
}
