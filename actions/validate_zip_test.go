package actions

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func createValidZip(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	w, err := zw.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func createCorruptZip(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("this is not a zip file"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateZipHeaderValid(t *testing.T) {
	dir := t.TempDir()
	path := createValidZip(t, dir, "good.zip")

	if err := validateZip(path, false); err != nil {
		t.Errorf("expected valid ZIP, got error: %v", err)
	}
}

func TestValidateZipFullValid(t *testing.T) {
	dir := t.TempDir()
	path := createValidZip(t, dir, "good.zip")

	if err := validateZip(path, true); err != nil {
		t.Errorf("expected valid ZIP with full check, got error: %v", err)
	}
}

func TestValidateZipHeaderCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := createCorruptZip(t, dir, "bad.zip")

	if err := validateZip(path, false); err == nil {
		t.Error("expected error for corrupt ZIP, got nil")
	}
}

func TestValidateZipFullCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := createCorruptZip(t, dir, "bad.zip")

	if err := validateZip(path, true); err == nil {
		t.Error("expected error for corrupt ZIP with full check, got nil")
	}
}

func TestValidateZipNonexistent(t *testing.T) {
	if err := validateZip("/nonexistent/file.zip", false); err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}
