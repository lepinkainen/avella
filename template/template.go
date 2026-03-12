package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// FileData holds the template variables available for path expansion.
type FileData struct {
	Filename string // Base filename (e.g. "movie.mkv")
	Year     string // Four-digit year from file mod time (e.g. "2025")
	Month    string // Two-digit month from file mod time (e.g. "09")
	Day      string // Two-digit day from file mod time (e.g. "05")
	Ext      string // File extension without dot (e.g. "mkv")
	Type     string // File type category (e.g. "Video", "Image")
}

// NewFileData builds template data from a file path.
// It stats the file to get the modification time.
func NewFileData(path string) (FileData, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileData{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return newFileData(path, info.ModTime()), nil
}

func newFileData(path string, modTime time.Time) FileData {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	return FileData{
		Filename: filepath.Base(path),
		Year:     modTime.Format("2006"),
		Month:    modTime.Format("01"),
		Day:      modTime.Format("02"),
		Ext:      ext,
		Type:     ClassifyExt(ext),
	}
}

// Expand resolves Go template placeholders in a path string using file metadata.
func Expand(tmpl string, data FileData) (string, error) {
	t, err := template.New("dest").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", tmpl, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", tmpl, err)
	}

	return buf.String(), nil
}

// HasPlaceholders reports whether the string contains Go template syntax.
func HasPlaceholders(s string) bool {
	return strings.Contains(s, "{{")
}

// ResolveDest expands template placeholders in tmpl using metadata from filePath.
// If tmpl contains no placeholders it is returned unchanged.
func ResolveDest(tmpl, filePath string) (string, error) {
	if !HasPlaceholders(tmpl) {
		return tmpl, nil
	}
	data, err := NewFileData(filePath)
	if err != nil {
		return "", err
	}
	return Expand(tmpl, data)
}
