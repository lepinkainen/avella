package actions

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
)

// ValidateZipAction checks that a file is a valid ZIP archive.
type ValidateZipAction struct {
	Full bool // true = verify CRC-32 checksums, false = structure only
}

func (a *ValidateZipAction) String() string {
	mode := "structure"
	if a.Full {
		mode = "full"
	}
	return fmt.Sprintf("validate_zip (%s)", mode)
}

// Describe returns a human-readable description for dry-run output.
func (a *ValidateZipAction) Describe(path string) string {
	return a.String()
}

// Execute validates the ZIP file at path.
func (a *ValidateZipAction) Execute(_ context.Context, path string) error {
	return validateZip(path, a.Full)
}

// validateZip checks that a ZIP file is intact.
// If full is true, it reads all entry data to verify CRC-32 checksums.
// If full is false, it only validates the ZIP structure (headers + central directory).
func validateZip(path string, full bool) (retErr error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fmt.Errorf("invalid ZIP structure: %w", err)
	}

	if !full {
		return nil
	}

	for _, entry := range zr.File {
		if err := validateZipEntry(entry); err != nil {
			return fmt.Errorf("entry %q: %w", entry.Name, err)
		}
	}

	return nil
}

func validateZipEntry(entry *zip.File) error {
	rc, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Reading triggers CRC-32 verification.
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("CRC check failed: %w", err)
	}

	return nil
}
