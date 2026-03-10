package stabilizer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrFileRemoved indicates the file was removed during stabilization.
var ErrFileRemoved = errors.New("file removed during stabilization")

// SkipExtensions are file extensions that indicate incomplete downloads.
var SkipExtensions = []string{".part", ".tmp", ".crdownload", ".download"}

// ShouldSkip returns true if the file has an extension indicating it's still downloading.
func ShouldSkip(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, skip := range SkipExtensions {
		if ext == skip {
			return true
		}
	}
	return false
}

// WaitStable polls a file's size and returns nil once the size is unchanged
// for the given number of consecutive checks at the given interval.
func WaitStable(ctx context.Context, path string, interval time.Duration, checks int) error {
	if checks < 1 {
		checks = 1
	}

	lastSize := int64(-1)
	stable := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: %s", ErrFileRemoved, path)
			}
			return fmt.Errorf("stat %s: %w", path, err)
		}

		size := info.Size()
		if size == lastSize {
			stable++
			if stable >= checks {
				return nil
			}
		} else {
			stable = 0
			lastSize = size
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
