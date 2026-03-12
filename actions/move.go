package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lepinkainen/avella/template"
)

// MoveAction moves a file to a destination directory.
type MoveAction struct {
	Dest string
}

func (a *MoveAction) String() string { return fmt.Sprintf("move → %s", a.Dest) }

// Describe returns the resolved destination for a specific file.
func (a *MoveAction) Describe(path string) string {
	dest, err := template.ResolveDest(a.Dest, path)
	if err != nil {
		return a.String()
	}
	return fmt.Sprintf("move → %s", dest)
}

// Execute moves the file at path to the destination directory.
// The Dest field may contain Go template placeholders (e.g. {{.Year}}, {{.Type}})
// that are resolved using the file's metadata before the move.
func (a *MoveAction) Execute(_ context.Context, path string) error {
	destDir, err := template.ResolveDest(a.Dest, path)
	if err != nil {
		return fmt.Errorf("resolve dest for %s: %w", path, err)
	}

	if mkdirErr := os.MkdirAll(destDir, 0o755); mkdirErr != nil {
		return fmt.Errorf("create dest dir %s: %w", destDir, mkdirErr)
	}

	dest := filepath.Join(destDir, filepath.Base(path))

	err = os.Rename(path, dest)
	if err == nil {
		slog.Info("moved file", "src", path, "dest", dest)
		return nil
	}

	// Fall back to copy+remove for cross-device moves
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return fmt.Errorf("rename %s to %s: %w", path, dest, err)
	}

	if err := copyFile(path, dest); err != nil {
		return fmt.Errorf("copy %s to %s: %w", path, dest, err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove source %s after copy: %w", path, err)
	}

	slog.Info("moved file (cross-device)", "src", path, "dest", dest)
	return nil
}

func copyFile(src, dst string) (retErr error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
