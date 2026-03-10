package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/avella/config"
)

func TestEngineFirstMatchWins(t *testing.T) {
	dir := t.TempDir()
	destDir1 := filepath.Join(dir, "dest1")
	destDir2 := filepath.Join(dir, "dest2")

	path := filepath.Join(dir, "test.torrent")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := []config.Rule{
		{
			Name:  "torrents",
			Match: config.MatchRule{FilenameRegex: `.*\.torrent$`},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: destDir1}},
			},
		},
		{
			Name:  "catch_all",
			Match: config.MatchRule{},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: destDir2}},
			},
		},
	}

	engine, err := NewEngine(rules, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.Process(context.Background(), path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be in dest1 (first rule), not dest2
	if _, err := os.Stat(filepath.Join(destDir1, "test.torrent")); err != nil {
		t.Error("file not found in first rule's dest dir")
	}
	if _, err := os.Stat(filepath.Join(destDir2, "test.torrent")); !os.IsNotExist(err) {
		t.Error("file should not be in second rule's dest dir")
	}
}

func TestEngineNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(path, []byte("# Hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := []config.Rule{
		{
			Name:  "videos",
			Match: config.MatchRule{FilenameRegex: `.*\.(mkv|mp4)$`},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: filepath.Join(dir, "videos")}},
			},
		},
	}

	engine, err := NewEngine(rules, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.Process(context.Background(), path); err != nil {
		t.Fatalf("unexpected error for no-match: %v", err)
	}

	// File should still be in original location
	if _, err := os.Stat(path); err != nil {
		t.Error("file should still exist when no rule matches")
	}
}

func TestEngineCorrectRuleSelected(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "videos")

	path := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(path, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := []config.Rule{
		{
			Name:  "subtitles",
			Match: config.MatchRule{FilenameRegex: `.*\.srt$`},
			Actions: []config.ActionConfig{
				{Exec: &config.ExecConfig{Command: "echo"}},
			},
		},
		{
			Name:  "videos",
			Match: config.MatchRule{FilenameRegex: `.*\.(mkv|mp4)$`},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: destDir}},
			},
		},
	}

	engine, err := NewEngine(rules, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.Process(context.Background(), path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be moved to videos dest
	if _, err := os.Stat(filepath.Join(destDir, "movie.mkv")); err != nil {
		t.Error("file not found in videos dest dir")
	}
}

func TestEngineDryRun(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "dest")

	path := filepath.Join(dir, "test.torrent")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := []config.Rule{
		{
			Name:  "torrents",
			Match: config.MatchRule{FilenameRegex: `.*\.torrent$`},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: destDir}},
			},
		},
	}

	engine, err := NewEngine(rules, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.Process(context.Background(), path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should NOT have moved
	if _, err := os.Stat(path); err != nil {
		t.Error("file should still exist in dry-run mode")
	}
	if _, err := os.Stat(filepath.Join(destDir, "test.torrent")); !os.IsNotExist(err) {
		t.Error("file should not have been moved in dry-run mode")
	}
}

func TestEngineBadRegex(t *testing.T) {
	rules := []config.Rule{
		{
			Name:  "bad",
			Match: config.MatchRule{FilenameRegex: "[invalid"},
			Actions: []config.ActionConfig{
				{Move: &config.MoveConfig{Dest: "/tmp"}},
			},
		},
	}

	_, err := NewEngine(rules, nil, false)
	if err == nil {
		t.Fatal("expected error for bad regex")
	}
}
