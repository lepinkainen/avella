package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/lepinkainen/avella/config"
	"github.com/lepinkainen/humanlog"
)

// Version is set at build time via ldflags.
var Version = "dev"

type CLI struct {
	Config  string `short:"c" help:"Path to config file." default:"" env:"AVELLA_CONFIG"`
	Verbose bool   `short:"v" help:"Enable verbose logging."`
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
}
