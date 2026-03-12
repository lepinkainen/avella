package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"strings"

	"github.com/alecthomas/kong"
	"github.com/lepinkainen/avella/config"
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

	var sshPool *ssh.Pool
	if len(cfg.SSHHosts) > 0 {
		sshPool = ssh.NewPool(cfg.SSHHosts)
		defer func() {
			if closeErr := sshPool.Close(); closeErr != nil {
				slog.Error("failed to close SSH pool", "error", closeErr)
			}
		}()
	}

	engine, err := rules.NewEngine(cfg.Rules, cfg.Ignored, sshPool, cli.DryRun)
	if err != nil {
		slog.Error("failed to create rule engine", "error", err)
		os.Exit(1)
	}

	if cli.DryRun {
		slog.Info("dry-run mode enabled, no actions will be executed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cli.Once {
		runOnce(ctx, cfg, engine)
		return
	}

	u := ui.New()
	u.SetRules(ruleInfoFromConfig(cfg.Rules))
	u.SetDryRunToggle(cli.DryRun, func(enabled bool) {
		engine.SetDryRun(enabled)
	})
	u.Run(ctx, stop, func(ctx context.Context) {
		runDaemon(ctx, cfg, engine, u)
	})

	slog.Info("shutting down")
}

func runOnce(ctx context.Context, cfg *config.Config, engine *rules.Engine) {
	slog.Info("running once, processing existing files")
	for _, dir := range cfg.Watch {
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
			if engine.ShouldIgnore(path) {
				slog.Debug("ignoring file (config)", "path", path)
				continue
			}
			if err := engine.Process(ctx, path); err != nil {
				slog.Error("failed to process file", "path", path, "error", err)
			}
		}
	}
	slog.Info("done")
}

func runDaemon(ctx context.Context, cfg *config.Config, engine *rules.Engine, u ui.UI) {
	w, err := watcher.New(cfg.Watch)
	if err != nil {
		slog.Error("failed to create watcher", "error", err)
		return
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			slog.Error("failed to close watcher", "error", closeErr)
		}
	}()

	w.IgnoreFunc = engine.ShouldIgnore
	files := w.Start(ctx)

	slog.Info("avella running, press Ctrl+C to stop")
	for path := range files {
		u.SetStatus("Processing " + path)
		if err := engine.Process(ctx, path); err != nil {
			slog.Error("failed to process file", "path", path, "error", err)
		}
		u.IncProcessed()
		u.SetStatus("Idle")
	}
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
