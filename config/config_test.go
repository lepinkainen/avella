package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	if err := os.Mkdir(watchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeTestConfig(t, tmpDir, `
watch:
  - `+watchDir+`
rules:
  - name: test_rule
    match:
      filename_regex: ".*\\.txt$"
    actions:
      - move:
          dest: `+tmpDir+`
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Watch) != 1 {
		t.Fatalf("expected 1 watch dir, got %d", len(cfg.Watch))
	}
	if cfg.Watch[0] != watchDir {
		t.Errorf("expected watch dir %q, got %q", watchDir, cfg.Watch[0])
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "test_rule" {
		t.Errorf("expected rule name %q, got %q", "test_rule", cfg.Rules[0].Name)
	}
}

func TestLoadNoWatchDirs(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := writeTestConfig(t, tmpDir, `
watch: []
rules: []
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for empty watch dirs")
	}
}

func TestLoadInvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	if err := os.Mkdir(watchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeTestConfig(t, tmpDir, `
watch:
  - `+watchDir+`
rules:
  - name: bad_regex
    match:
      filename_regex: "[invalid"
    actions:
      - move:
          dest: /tmp
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoadMissingWatchDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := writeTestConfig(t, tmpDir, `
watch:
  - /nonexistent/path/that/does/not/exist
rules: []
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for nonexistent watch dir")
	}
}

func TestLoadUnknownSSHHost(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	if err := os.Mkdir(watchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeTestConfig(t, tmpDir, `
watch:
  - `+watchDir+`
rules:
  - name: scp_rule
    match:
      filename_regex: ".*"
    actions:
      - scp:
          host: nonexistent
          dest: /tmp
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for unknown SSH host")
	}
}

func TestLoadNoActions(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	if err := os.Mkdir(watchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeTestConfig(t, tmpDir, `
watch:
  - `+watchDir+`
rules:
  - name: no_action
    match:
      filename_regex: ".*"
    actions: []
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for rule with no actions")
	}
}
