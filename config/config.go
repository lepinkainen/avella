package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lepinkainen/avella/internal/pathutil"
	"github.com/spf13/viper"
)

// IgnoreRule defines a rule for ignoring files before stabilization.
type IgnoreRule struct {
	Match MatchRule `mapstructure:"match"`
}

// Config is the top-level configuration.
type Config struct {
	Watch    []string              `mapstructure:"watch"`
	SSHHosts map[string]SSH        `mapstructure:"ssh_hosts"`
	Ignored  map[string]IgnoreRule `mapstructure:"ignored"`
	Rules    []Rule                `mapstructure:"rules"`
}

// SSH defines an SSH host connection.
type SSH struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Key  string `mapstructure:"key"`
}

// Rule defines a file matching rule and its actions.
type Rule struct {
	Name      string         `mapstructure:"name"`
	Match     MatchRule      `mapstructure:"match"`
	Actions   []ActionConfig `mapstructure:"actions"`
	OnSuccess []ActionConfig `mapstructure:"on_success"`
	OnFail    []ActionConfig `mapstructure:"on_fail"`
}

// MatchRule defines criteria for matching files.
type MatchRule struct {
	FilenameRegex string `mapstructure:"filename_regex"`
	FilenameGlob  string `mapstructure:"filename_glob"`
	MinAge        string `mapstructure:"min_age"`
	MinAgeSeconds int    `mapstructure:"min_age_seconds"`
	MinSize       int64  `mapstructure:"min_size"`
	MaxSize       int64  `mapstructure:"max_size"`
}

// ActionConfig defines an action to perform on a matched file.
// Only one field should be set per action.
type ActionConfig struct {
	Move        *MoveConfig        `mapstructure:"move"`
	SCP         *SCPConfig         `mapstructure:"scp"`
	Exec        *ExecConfig        `mapstructure:"exec"`
	ValidateZip *ValidateZipConfig `mapstructure:"validate_zip"`
	Notify      *NotifyConfig      `mapstructure:"notify"`
}

// MoveConfig defines a file move action.
type MoveConfig struct {
	Dest string `mapstructure:"dest"`
}

// SCPConfig defines an SCP upload action.
type SCPConfig struct {
	Host         string `mapstructure:"host"`
	Dest         string `mapstructure:"dest"`
	DeleteSource bool   `mapstructure:"delete_source"`
}

// ExecConfig defines a command execution action.
type ExecConfig struct {
	Command string `mapstructure:"command"`
}

// ValidateZipConfig defines a ZIP validation action.
type ValidateZipConfig struct {
	Mode string `mapstructure:"mode"` // "true" for structure check, "full" for CRC-32
}

// NotifyConfig defines a macOS notification action.
type NotifyConfig struct {
	Message string `mapstructure:"message"` // supports template vars (e.g. {{.Filename}})
}

// TypeName returns the action type as a string.
func (ac ActionConfig) TypeName() string {
	switch {
	case ac.Move != nil:
		return "move"
	case ac.SCP != nil:
		return "scp"
	case ac.Exec != nil:
		return "exec"
	case ac.ValidateZip != nil:
		return "validate_zip"
	case ac.Notify != nil:
		return "notify"
	default:
		return "unknown"
	}
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return home + "/.config/avella/config.yaml"
}

// Load reads and parses a config file at the given path.
func Load(path string) (*Config, error) {
	expanded, err := pathutil.ExpandHome(path)
	if err != nil {
		return nil, fmt.Errorf("expand config path: %w", err)
	}

	viper.SetConfigFile(expanded)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.expandPaths(); err != nil {
		return nil, fmt.Errorf("expand paths: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	if len(c.Watch) == 0 {
		return fmt.Errorf("no watch directories configured")
	}

	for _, dir := range c.Watch {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("watch directory %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("watch path %q is not a directory", dir)
		}
	}

	for name, ig := range c.Ignored {
		if err := validateMatchRule(name, ig.Match); err != nil {
			return fmt.Errorf("ignored %q: %w", name, err)
		}
	}

	for i, rule := range c.Rules {
		if rule.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if err := validateMatchRule(rule.Name, rule.Match); err != nil {
			return fmt.Errorf("rule %q: %w", rule.Name, err)
		}
		for j, action := range rule.Actions {
			if err := c.validateAction(rule.Name, "action", j, action); err != nil {
				return err
			}
		}
		for j, action := range rule.OnSuccess {
			if err := c.validateAction(rule.Name, "on_success", j, action); err != nil {
				return err
			}
		}
		for j, action := range rule.OnFail {
			if err := c.validateAction(rule.Name, "on_fail", j, action); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateMatchRule(_ string, m MatchRule) error {
	if m.FilenameRegex != "" {
		if _, err := regexp.Compile(m.FilenameRegex); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	if m.FilenameGlob != "" {
		if _, err := filepath.Match(m.FilenameGlob, "test"); err != nil {
			return fmt.Errorf("invalid glob %q: %w", m.FilenameGlob, err)
		}
	}
	if m.MinAge != "" {
		if _, err := time.ParseDuration(m.MinAge); err != nil {
			return fmt.Errorf("invalid min_age %q: %w", m.MinAge, err)
		}
	}
	return nil
}

func (c *Config) validateAction(ruleName, list string, index int, action ActionConfig) error {
	if action.Move == nil && action.SCP == nil && action.Exec == nil && action.ValidateZip == nil && action.Notify == nil {
		return fmt.Errorf("rule %q %s %d: no action type specified", ruleName, list, index)
	}
	if action.SCP != nil {
		if _, ok := c.SSHHosts[action.SCP.Host]; !ok {
			return fmt.Errorf("rule %q %s %d: unknown SSH host %q", ruleName, list, index, action.SCP.Host)
		}
	}
	if action.ValidateZip != nil {
		switch action.ValidateZip.Mode {
		case "true", "full":
			// valid
		default:
			return fmt.Errorf("rule %q %s %d: invalid validate_zip mode %q (must be \"true\" or \"full\")", ruleName, list, index, action.ValidateZip.Mode)
		}
	}
	return nil
}

func (c *Config) expandPaths() error {
	for i, dir := range c.Watch {
		expanded, err := pathutil.ExpandHome(dir)
		if err != nil {
			return fmt.Errorf("watch dir %q: %w", dir, err)
		}
		c.Watch[i] = expanded
	}

	for name, ssh := range c.SSHHosts {
		expanded, err := pathutil.ExpandHome(ssh.Key)
		if err != nil {
			return fmt.Errorf("ssh host %q key: %w", name, err)
		}
		ssh.Key = expanded
		c.SSHHosts[name] = ssh
	}

	for i, rule := range c.Rules {
		for j, action := range rule.Actions {
			if action.Move != nil {
				expanded, err := pathutil.ExpandHome(action.Move.Dest)
				if err != nil {
					return fmt.Errorf("rule %q move dest: %w", rule.Name, err)
				}
				c.Rules[i].Actions[j].Move.Dest = expanded
			}
		}
	}

	return nil
}
