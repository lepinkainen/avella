package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lepinkainen/avella/stabilizer"
)

// DefaultStableInterval is the default polling interval for file stabilization.
const DefaultStableInterval = 2 * time.Second

// DefaultStableChecks is the default number of consecutive stable checks required.
const DefaultStableChecks = 3

// Watcher watches directories for new files and emits stabilized file paths.
type Watcher struct {
	fsw            *fsnotify.Watcher
	stableInterval time.Duration
	stableChecks   int
	IgnoreFunc     func(path string) bool // optional; called before stabilization
}

// New creates a Watcher that monitors the given directories.
func New(dirs []string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if err := fsw.Add(dir); err != nil {
			_ = fsw.Close()
			return nil, err
		}
		slog.Info("watching directory", "dir", dir)
	}

	return &Watcher{
		fsw:            fsw,
		stableInterval: DefaultStableInterval,
		stableChecks:   DefaultStableChecks,
	}, nil
}

// Start begins watching and returns a channel of stabilized file paths.
// The channel is closed when the context is cancelled.
func (w *Watcher) Start(ctx context.Context) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)

		var wg sync.WaitGroup
		pending := make(map[string]bool)
		var mu sync.Mutex

		defer wg.Wait()

		for {
			select {
			case <-ctx.Done():
				return

			case event, ok := <-w.fsw.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Create == 0 {
					continue
				}

				path := event.Name

				if stabilizer.ShouldSkip(path) {
					slog.Debug("skipping temporary file", "path", path)
					continue
				}

				if w.IgnoreFunc != nil && w.IgnoreFunc(path) {
					slog.Debug("ignoring file (config)", "path", path)
					continue
				}

				mu.Lock()
				if pending[path] {
					mu.Unlock()
					continue
				}
				pending[path] = true
				mu.Unlock()

				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						mu.Lock()
						delete(pending, path)
						mu.Unlock()
					}()

					slog.Debug("waiting for file to stabilize", "path", path)
					if err := stabilizer.WaitStable(ctx, path, w.stableInterval, w.stableChecks); err != nil {
						slog.Warn("file stabilization failed", "path", path, "error", err)
						return
					}

					slog.Info("file ready", "path", path)
					select {
					case out <- path:
					case <-ctx.Done():
					}
				}()

			case err, ok := <-w.fsw.Errors:
				if !ok {
					return
				}
				slog.Error("watcher error", "error", err)
			}
		}
	}()

	return out
}

// Close stops the underlying fsnotify watcher.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}
