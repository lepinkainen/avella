# Avella

A lightweight file automation daemon for macOS/Linux. Watches directories for new files, matches them against rules, and performs actions automatically.

Think of it as a minimal, YAML-configured alternative to [Hazel](https://www.noodlesoft.com/) — covering just the subset of features I actually needed.

## What it does

Avella watches directories (e.g. `~/Downloads`) and processes new files through a rule pipeline:

1. **Detect** — picks up new files via fsnotify
2. **Stabilize** — waits for the file to finish writing (handles partial downloads, slow copies)
3. **Match** — evaluates rules top-down, first match wins
4. **Act** — runs actions: move, exec, SCP upload, ZIP validation, macOS notifications
5. **Hooks** — runs `on_success` / `on_fail` actions after the primary actions complete

## Example config

Config lives at `~/.config/avella/config.yaml` (override with `-c`). See [`config.yaml.example`](config.yaml.example) for a full annotated example.

```yaml
watch:
  - ~/Downloads

rules:
  - name: video_files
    match:
      filename_regex: ".*\\.(mkv|mp4|avi)$"
      min_size: 1048576
    actions:
      - move:
          dest: "~/Media/{{.Year}}-{{.Month}} {{.Type}}/"
    on_success:
      - notify:
          message: "Sorted {{.Filename}}"
```

### Match criteria

| Field | Description |
|-------|-------------|
| `filename_regex` | Regex against the filename |
| `filename_glob` | Glob pattern against the filename |
| `file_type` | Category: `Video`, `Image`, `Audio`, `Document`, `Archive`, `Other` |
| `min_size` / `max_size` | File size in bytes |
| `min_age` | Go duration (e.g. `1h`, `30m`) since last modification |

### Actions

| Action | Description |
|--------|-------------|
| `move` | Move/rename the file. Supports template variables (`{{.Filename}}`, `{{.Year}}`, `{{.Month}}`, `{{.Day}}`, `{{.Ext}}`, `{{.Type}}`) |
| `exec` | Run a command with the file path as an argument |
| `scp` | Upload via SFTP to a configured SSH host |
| `validate_zip` | Check ZIP integrity (`"true"` for structure, `"full"` for CRC-32 verification) |
| `notify` | macOS notification via `osascript` |

## Building

Requires [Go](https://go.dev/) and [Task](https://taskfile.dev/):

```bash
task build    # test + lint + build → build/avella
```

## Usage

```plain
avella                     # run as a daemon
avella -c config.yaml      # custom config path
avella --dry-run           # log actions without executing
avella --once              # process existing files and exit
avella -v                  # verbose logging
```

## Installation

This is a personal project — there's no package or installer. Fork the repo and build it yourself. The example plist files in the repo can be used to run it as a launchd service on macOS.

## License

MIT
