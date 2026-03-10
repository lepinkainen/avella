# hazel-lite — Implementation Plan (Go)

A lightweight personal automation daemon inspired by Hazel.  
Runs locally, watches directories, evaluates rules, and performs actions such as moving files, SCP uploads, or executing commands.

No GUI configuration is required. Rules are defined in YAML.

Optional macOS menu bar indicator is implemented using `github.com/getlantern/systray`.

---

# Core Features

Required functionality:

- Watch directories for new or modified files
- Wait until files are fully downloaded before processing
- Apply rules based on:
  - filename / extension
  - file age
  - regex
  - size
- Perform actions:
  - move file
  - upload via SCP
  - run executable with file as parameter
- Configuration via YAML
- Optional macOS menu bar indicator

---

# High-Level Architecture

hazel-lite
│
├── main.go
├── config/
│   └── config.go
│
├── watcher/
│   └── watcher.go
│
├── stabilizer/
│   └── stabilizer.go
│
├── rules/
│   ├── engine.go
│   └── match.go
│
├── actions/
│   ├── move.go
│   ├── exec.go
│   └── scp.go
│
├── ssh/
│   └── ssh.go
│
└── ui/
└── systray.go

---

# Key Go Libraries

Filesystem watching

github.com/fsnotify/fsnotify

YAML configuration

gopkg.in/yaml.v3

SSH / SCP

golang.org/x/crypto/ssh
github.com/pkg/sftp

Menu bar icon

github.com/getlantern/systray

---

# Configuration Format

Example `config.yaml`

```yaml
watch:
  - ~/Downloads

ssh_hosts:
  seedbox:
    host: seedbox.example.com
    user: user
    key: ~/.ssh/id_rsa

rules:
  - name: torrents
    match:
      filename_regex: ".*\\.torrent$"
    actions:
      - scp:
          host: seedbox
          dest: /torrents/incoming/

  - name: video_files
    match:
      filename_regex: ".*\\.(mkv|mp4)$"
    actions:
      - move:
          dest: ~/Media/Unsorted/

  - name: subtitles
    match:
      filename_regex: ".*\\.srt$"
    actions:
      - exec:
          command: /usr/local/bin/subtitle_tool


⸻

Config Structures

type Config struct {
    Watch    []string        `yaml:"watch"`
    SSHHosts map[string]SSH  `yaml:"ssh_hosts"`
    Rules    []Rule          `yaml:"rules"`
}

type SSH struct {
    Host string `yaml:"host"`
    User string `yaml:"user"`
    Key  string `yaml:"key"`
}

type Rule struct {
    Name    string     `yaml:"name"`
    Match   MatchRule  `yaml:"match"`
    Actions []Action   `yaml:"actions"`
}

type MatchRule struct {
    FilenameRegex string `yaml:"filename_regex"`
    MinAgeSeconds int    `yaml:"min_age_seconds"`
}


⸻

Directory Watching

Use fsnotify.

watcher, _ := fsnotify.NewWatcher()

for _, dir := range config.Watch {
    watcher.Add(dir)
}

for {
    select {
    case event := <-watcher.Events:
        if event.Op&fsnotify.Create == fsnotify.Create {
            go processFile(event.Name)
        }
    }
}


⸻

Detecting Finished Downloads

Files should not be processed while downloading.

Simple and reliable strategy:

Wait until the file size stabilizes.

size1 = stat(file).size
sleep 5
size2 = stat(file).size

if size1 == size2
    file is stable

Example implementation:

func waitForStableFile(path string) {
    for {
        s1 := filesize(path)
        time.Sleep(5 * time.Second)
        s2 := filesize(path)

        if s1 == s2 {
            return
        }
    }
}

Optional improvements:
 • ignore extensions like .part, .tmp, .download
 • require stability for multiple cycles

⸻

Rule Engine

Each file is checked against rules.

file event
   ↓
wait until stable
   ↓
evaluate rules
   ↓
execute actions

Example:

func matchRule(file string, rule MatchRule) bool {
    if rule.FilenameRegex != "" {
        r := regexp.MustCompile(rule.FilenameRegex)
        if !r.MatchString(filepath.Base(file)) {
            return false
        }
    }
    return true
}


⸻

Actions

Move File

func MoveFile(src, destDir string) error {
    dest := filepath.Join(destDir, filepath.Base(src))
    return os.Rename(src, dest)
}


⸻

Execute Command

func RunCommand(cmd string, file string) error {
    c := exec.Command(cmd, file)
    return c.Run()
}


⸻

SCP Upload

Uses SSH + SFTP.

client, err := sftp.NewClient(sshClient)
dst, err := client.Create(destPath)

src, _ := os.Open(file)
io.Copy(dst, src)


⸻

Processing Pipeline

fsnotify event
      ↓
waitForStableFile()
      ↓
for each rule
      ↓
matchRule()
      ↓
execute actions


⸻

Optional macOS Menu Bar Icon

Implemented using:

github.com/getlantern/systray

Example:

func onReady() {
    systray.SetTitle("HazelLite")
    systray.SetTooltip("Hazel Lite running")

    quit := systray.AddMenuItem("Quit", "Quit the app")

    go func() {
        <-quit.ClickedCh
        systray.Quit()
    }()
}

Start systray:

func main() {
    go runDaemon()
    systray.Run(onReady, func(){})
}

Possible enhancements:
 • icon changes when processing files
 • display rule activity
 • show number of pending files

⸻

Running as a Background Service

Simplest approach:

hazel-lite &

Better macOS integration:

Create a launchd agent.

Example:

~/Library/LaunchAgents/com.user.hazellite.plist

This ensures:
 • automatic startup
 • restart on crash

⸻

Future Improvements

Possible enhancements:
 • rule testing CLI
 • dry-run mode
 • logging system
 • retry queue for failed uploads
 • per-rule concurrency limits
 • metrics / Prometheus
 • optional Web UI

⸻

Estimated Implementation Size

Rough code size:

watcher      ~100 lines
rule engine  ~150 lines
actions      ~200 lines
ssh/scp      ~150 lines
config       ~100 lines
systray      ~80 lines

Total:

~600–800 lines of Go

A weekend-sized project.


