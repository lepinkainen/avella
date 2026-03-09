package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/lepinkainen/avella/internal/pathutil"
	"github.com/spf13/viper"
)

// Config is the top-level configuration.
type Config struct {
	Watch    []string       `mapstructure:"watch"`
	SSHHosts map[string]SSH `mapstructure:"ssh_hosts"`
	Rules    []Rule         `mapstructure:"rules"`
}

// SSH defines an SSH host connection.
type SSH struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Key  string `mapstructure:"key"`
}

// Rule defines a file matching rule and its actions.
type Rule struct {
	Name    string         `mapstructure:"name"`
	Match   MatchRule      `mapstructure:"match"`
	Actions []ActionConfig `mapstructure:"actions"`
}

// MatchRule defines criteria for matching files.
type MatchRule struct {
	FilenameRegex string `mapstructure:"filename_regex"`
	MinAgeSeconds int    `mapstructure:"min_age_seconds"`
	MinSize       int64  `mapstructure:"min_size"`
	MaxSize       int64  `mapstructure:"max_size"`
}

// ActionConfig defines an action to perform on a matched file.
// Only one field should be set per action.
type ActionConfig struct {
	Move *MoveConfig `mapstructure:"move"`
	SCP  *SCPConfig  `mapstructure:"scp"`
	Exec *ExecConfig `mapstructure:"exec"`
}

// MoveConfig defines a file move action.
type MoveConfig struct {
	Dest string `mapstructure:"dest"`
}

// SCPConfig defines an SCP upload action.
type SCPConfig struct {
	Host string `mapstructure:"host"`
	Dest string `mapstructure:"dest"`
}

// ExecConfig defines a command execution action.
type ExecConfig struct {
	Command string `mapstructure:"command"`
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

	for i, rule := range c.Rules {
		if rule.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if rule.Match.FilenameRegex != "" {
			if _, err := regexp.Compile(rule.Match.FilenameRegex); err != nil {
				return fmt.Errorf("rule %q: invalid regex: %w", rule.Name, err)
			}
		}
		if len(rule.Actions) == 0 {
			return fmt.Errorf("rule %q: at least one action is required", rule.Name)
		}
		for j, action := range rule.Actions {
			if action.Move == nil && action.SCP == nil && action.Exec == nil {
				return fmt.Errorf("rule %q action %d: no action type specified", rule.Name, j)
			}
			if action.SCP != nil {
				if _, ok := c.SSHHosts[action.SCP.Host]; !ok {
					return fmt.Errorf("rule %q action %d: unknown SSH host %q", rule.Name, j, action.SCP.Host)
				}
			}
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
