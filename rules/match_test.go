package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/lepinkainen/avella/config"
)

func createTestFile(t *testing.T, dir, name string, size int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMatchesRegex(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "video.mkv", 100)
	info, _ := os.Stat(path)

	re := regexp.MustCompile(`.*\.mkv$`)
	rule := config.MatchRule{FilenameRegex: `.*\.mkv$`}

	if !Matches(path, info, rule, re) {
		t.Error("expected match for .mkv file")
	}

	reNoMatch := regexp.MustCompile(`.*\.txt$`)
	ruleNoMatch := config.MatchRule{FilenameRegex: `.*\.txt$`}

	if Matches(path, info, ruleNoMatch, reNoMatch) {
		t.Error("expected no match for .txt regex on .mkv file")
	}
}

func TestMatchesMinAge(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "old.txt", 10)

	// Set modtime to 1 hour ago
	oldTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)

	rule := config.MatchRule{MinAgeSeconds: 60}
	if !Matches(path, info, rule, nil) {
		t.Error("expected match for file older than 60s")
	}

	rule = config.MatchRule{MinAgeSeconds: 7200}
	if Matches(path, info, rule, nil) {
		t.Error("expected no match for file younger than 7200s")
	}
}

func TestMatchesSize(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "medium.bin", 500)
	info, _ := os.Stat(path)

	// MinSize pass
	rule := config.MatchRule{MinSize: 100}
	if !Matches(path, info, rule, nil) {
		t.Error("expected match for file >= 100 bytes")
	}

	// MinSize fail
	rule = config.MatchRule{MinSize: 1000}
	if Matches(path, info, rule, nil) {
		t.Error("expected no match for file < 1000 bytes")
	}

	// MaxSize pass
	rule = config.MatchRule{MaxSize: 1000}
	if !Matches(path, info, rule, nil) {
		t.Error("expected match for file <= 1000 bytes")
	}

	// MaxSize fail
	rule = config.MatchRule{MaxSize: 100}
	if Matches(path, info, rule, nil) {
		t.Error("expected no match for file > 100 bytes")
	}
}

func TestMatchesCombined(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "data.csv", 200)

	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)

	re := regexp.MustCompile(`.*\.csv$`)
	rule := config.MatchRule{
		FilenameRegex: `.*\.csv$`,
		MinAgeSeconds: 60,
		MinSize:       100,
		MaxSize:       1000,
	}

	if !Matches(path, info, rule, re) {
		t.Error("expected match for combined predicates")
	}

	// Fail one predicate
	rule.MinSize = 500
	if Matches(path, info, rule, re) {
		t.Error("expected no match when one predicate fails")
	}
}

func TestMatchesEmptyRule(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "anything.xyz", 50)
	info, _ := os.Stat(path)

	rule := config.MatchRule{}
	if !Matches(path, info, rule, nil) {
		t.Error("empty match rule should match everything")
	}
}
