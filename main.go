package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fsnotify/fsnotify"
	"github.com/lepinkainen/avella/config"
	"github.com/lepinkainen/avella/internal/pathutil"
	"github.com/lepinkainen/avella/rules"
	"github.com/lepinkainen/avella/ssh"
	"github.com/lepinkainen/avella/stabilizer"
	"github.com/lepinkainen/avella/ui"
	"github.com/lepinkainen/avella/watcher"
	"github.com/lepinkainen/humanlog"
)

// Version is set at build time via ldflags.
var Version = "dev"

type CLI struct {
	Config  string `short:"c" help:"Path to config file." default:"" env:"AVELLA_CONFIG"`
	Verbose bool   `short:"v" help:"Enable verbose logging."`
	DryRun  bool   `help:"Log actions without executing them." name:"dry-run"`
	Once    bool   `help:"Process existing files in watch directories once and exit."`
	Version bool   `help:"Print version and exit."`
}

type runtimeState struct {
	mu       sync.RWMutex
	engine   *rules.Engine
	dryRun   bool
	sshPools []*ssh.Pool
}

func newRuntimeState(engine *rules.Engine, dryRun bool, pool *ssh.Pool) *runtimeState {
	st := &runtimeState{engine: engine, dryRun: dryRun}
	if pool != nil {
		st.sshPools = append(st.sshPools, pool)
	}
	return st
}

func (s *runtimeState) Process(ctx context.Context, path string) (rules.ProcessResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine.Process(ctx, path)
}

func (s *runtimeState) ShouldIgnore(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine.ShouldIgnore(path)
}

func (s *runtimeState) SetDryRun(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dryRun = enabled
	s.engine.SetDryRun(enabled)
}

func (s *runtimeState) DryRun() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dryRun
}

func (s *runtimeState) SwapEngine(engine *rules.Engine, pool *ssh.Pool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engine = engine
	if pool != nil {
		s.sshPools = append(s.sshPools, pool)
	}
}

func (s *runtimeState) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	for _, pool := range s.sshPools {
		if pool == nil {
			continue
		}
		if err := pool.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.sshPools = nil
	return firstErr
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("avella"),
		kong.Description("A lightweight file automation daemon."),
	)

	if cli.Version {
		fmt.Println("avella", Version)
		os.Exit(0)
	}

	level := slog.LevelInfo
	if cli.Verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(humanlog.NewHandler(os.Stderr, &humanlog.Options{Level: level})))

	cfgPath := cli.Config
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	if expandedCfgPath, err := pathutil.ExpandHome(cfgPath); err == nil {
		cfgPath = expandedCfgPath
	}

	slog.Info("loading config", "path", cfgPath)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded",
		"watch_dirs", cfg.Watch,
		"rules", len(cfg.Rules),
		"ssh_hosts", len(cfg.SSHHosts),
	)

	sshPool := newSSH(cfg)
	runtime := newRuntimeState(nil, cli.DryRun, sshPool)
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			slog.Error("failed to close SSH pool", "error", closeErr)
		}
	}()

	engine, err := rules.NewEngine(cfg.Rules, cfg.Ignored, sshPool, cli.DryRun)
	if err != nil {
		slog.Error("failed to create rule engine", "error", err)
		os.Exit(1)
	}
	runtime.SwapEngine(engine, nil)

	if cli.DryRun {
		slog.Info("dry-run mode enabled, no actions will be executed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cli.Once {
		runOnce(ctx, cfg.Watch, runtime)
		return
	}

	u := ui.New()
	u.SetVersion(Version)
	u.SetRules(ruleInfoFromConfig(cfg.Rules))
	u.SetConfigPath(cfgPath)
	u.SetDryRunToggle(cli.DryRun, func(enabled bool) {
		runtime.SetDryRun(enabled)
	})
	u.Run(ctx, stop, func(ctx context.Context) {
		runDaemon(ctx, cfgPath, cfg.Watch, runtime, u)
	})

	slog.Info("shutting down")
}

func newSSH(cfg *config.Config) *ssh.Pool {
	if len(cfg.SSHHosts) == 0 {
		return nil
	}
	return ssh.NewPool(cfg.SSHHosts)
}

func processExistingFiles(ctx context.Context, watchDirs []string, runtime *runtimeState, u ui.UI) {
	for _, dir := range watchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			slog.Error("failed to read watch directory", "dir", dir, "error", err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if stabilizer.ShouldSkip(path) {
				slog.Debug("skipping temporary file", "path", path)
				continue
			}
			if runtime.ShouldIgnore(path) {
				slog.Debug("ignoring file (config)", "path", path)
				continue
			}
			result, err := runtime.Process(ctx, path)
			if err != nil {
				slog.Error("failed to process file", "path", path, "error", err)
			}
			if u != nil && result.Matched {
				addRecentFromResult(u, path, result)
			}
		}
	}
}

func runOnce(ctx context.Context, watchDirs []string, runtime *runtimeState) {
	slog.Info("running once, processing existing files")
	processExistingFiles(ctx, watchDirs, runtime, nil)
	slog.Info("done")
}

func runDaemon(ctx context.Context, cfgPath string, watchDirs []string, runtime *runtimeState, u ui.UI) {
	w, err := watcher.New(watchDirs)
	if err != nil {
		slog.Error("failed to create watcher", "error", err)
		return
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			slog.Error("failed to close watcher", "error", closeErr)
		}
	}()

	w.IgnoreFunc = runtime.ShouldIgnore
	files := w.Start(ctx)

	startConfigReloader(ctx, cfgPath, watchDirs, runtime, u)

	slog.Info("processing existing files in watch directories")
	processExistingFiles(ctx, watchDirs, runtime, u)

	slog.Info("avella running, press Ctrl+C to stop")
	for path := range files {
		u.SetStatus("Processing " + path)
		result, err := runtime.Process(ctx, path)
		if err != nil {
			slog.Error("failed to process file", "path", path, "error", err)
		}
		if result.Matched {
			addRecentFromResult(u, path, result)
		}
		u.IncProcessed()
		u.SetStatus("Idle")
	}
}

func startConfigReloader(ctx context.Context, cfgPath string, watchDirs []string, runtime *runtimeState, u ui.UI) {
	watcherDir := filepath.Dir(cfgPath)
	watcherBase := filepath.Base(cfgPath)

	cfgWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("failed to start config watcher", "path", cfgPath, "error", err)
		return
	}
	if err := cfgWatcher.Add(watcherDir); err != nil {
		_ = cfgWatcher.Close()
		slog.Warn("failed to watch config directory", "dir", watcherDir, "error", err)
		return
	}

	slog.Info("watching config for changes", "path", cfgPath)

	go func() {
		defer func() {
			if closeErr := cfgWatcher.Close(); closeErr != nil {
				slog.Warn("failed to close config watcher", "error", closeErr)
			}
		}()

		var reloadTimer *time.Timer
		var reloadCh <-chan time.Time

		scheduleReload := func() {
			if reloadTimer == nil {
				reloadTimer = time.NewTimer(300 * time.Millisecond)
				reloadCh = reloadTimer.C
				return
			}
			if !reloadTimer.Stop() {
				select {
				case <-reloadTimer.C:
				default:
				}
			}
			reloadTimer.Reset(300 * time.Millisecond)
		}
		defer func() {
			if reloadTimer != nil {
				reloadTimer.Stop()
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-cfgWatcher.Events:
				if !ok {
					return
				}
				if filepath.Base(event.Name) != watcherBase {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Chmod) == 0 {
					continue
				}
				slog.Debug("config file changed", "path", event.Name, "op", event.Op.String())
				scheduleReload()
			case err, ok := <-cfgWatcher.Errors:
				if !ok {
					return
				}
				slog.Warn("config watcher error", "error", err)
			case <-reloadCh:
				reloadCh = nil
				reloadConfig(cfgPath, watchDirs, runtime, u)
			}
		}
	}()
}

func reloadConfig(cfgPath string, watchDirs []string, runtime *runtimeState, u ui.UI) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("config reload failed", "path", cfgPath, "error", err)
		notifyReload(fmt.Sprintf("Config reload failed: %v", err))
		return
	}

	watchChanged := !reflect.DeepEqual(cfg.Watch, watchDirs)
	if watchChanged {
		slog.Warn("config watch directories changed; restart required to apply", "configured", cfg.Watch, "active", watchDirs)
	}

	sshPool := newSSH(cfg)
	engine, err := rules.NewEngine(cfg.Rules, cfg.Ignored, sshPool, runtime.DryRun())
	if err != nil {
		if sshPool != nil {
			if closeErr := sshPool.Close(); closeErr != nil {
				slog.Warn("failed to close reloaded SSH pool", "error", closeErr)
			}
		}
		slog.Error("config reload failed", "path", cfgPath, "error", err)
		notifyReload(fmt.Sprintf("Config reload failed: %v", err))
		return
	}

	runtime.SwapEngine(engine, sshPool)
	u.SetRules(ruleInfoFromConfig(cfg.Rules))

	msg := fmt.Sprintf("Config reloaded (%d rules)", len(cfg.Rules))
	if watchChanged {
		msg += "; restart required for watch dir changes"
	}
	slog.Info("config reloaded", "path", cfgPath, "rules", len(cfg.Rules), "ssh_hosts", len(cfg.SSHHosts), "watch_dirs_changed", watchChanged)
	notifyReload(msg)
}

func notifyReload(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	script := fmt.Sprintf(`display notification %q with title "Avella"`, msg)
	if err := exec.CommandContext(ctx, "osascript", "-e", script).Run(); err != nil {
		slog.Warn("reload notification failed", "error", err)
	}
}

func addRecentFromResult(u ui.UI, path string, result rules.ProcessResult) {
	action := ""
	if len(result.Actions) > 0 {
		action = result.Actions[0]
	}
	u.AddRecentFile(ui.RecentFile{
		Filename: filepath.Base(path),
		Rule:     result.RuleName,
		Action:   action,
		DryRun:   result.DryRun,
		Time:     time.Now().Format(time.RFC3339),
	})
}

func ruleInfoFromConfig(cfgRules []config.Rule) []ui.RuleInfo {
	infos := make([]ui.RuleInfo, len(cfgRules))
	for i, r := range cfgRules {
		types := make([]string, 0, len(r.Actions))
		for _, ac := range r.Actions {
			types = append(types, ac.TypeName())
		}
		infos[i] = ui.RuleInfo{
			Name:       r.Name,
			ActionType: strings.Join(types, "+"),
		}
	}
	return infos
}
