package rules

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/lepinkainen/avella/actions"
	"github.com/lepinkainen/avella/config"
	"github.com/lepinkainen/avella/ssh"
)

type compiledRule struct {
	name    string
	match   config.MatchRule
	regex   *regexp.Regexp
	actions []actions.Action
}

// Engine evaluates files against configured rules.
type Engine struct {
	rules  []compiledRule
	dryRun bool
}

// NewEngine creates an Engine with pre-compiled regexes and actions for all rules.
// The sshPool may be nil if no SSH hosts are configured.
func NewEngine(rules []config.Rule, sshPool *ssh.Pool, dryRun bool) (*Engine, error) {
	compiled := make([]compiledRule, len(rules))
	for i, r := range rules {
		cr := compiledRule{
			name:  r.Name,
			match: r.Match,
		}

		if r.Match.FilenameRegex != "" {
			re, err := regexp.Compile(r.Match.FilenameRegex)
			if err != nil {
				return nil, fmt.Errorf("rule %q: compile regex: %w", r.Name, err)
			}
			cr.regex = re
		}

		for j, ac := range r.Actions {
			action, err := actions.FromConfig(ac, sshPool)
			if err != nil {
				return nil, fmt.Errorf("rule %q action %d: %w", r.Name, j, err)
			}
			cr.actions = append(cr.actions, action)
		}

		compiled[i] = cr
	}
	return &Engine{rules: compiled, dryRun: dryRun}, nil
}

// Process evaluates a file against all rules. First match wins.
func (e *Engine) Process(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	for _, cr := range e.rules {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !Matches(path, info, cr.match, cr.regex) {
			continue
		}

		slog.Info("rule matched", "rule", cr.name, "path", path)

		if e.dryRun {
			for _, action := range cr.actions {
				slog.Info("[dry-run] would execute action", "rule", cr.name, "action", action, "path", path)
			}
			return nil
		}

		for _, action := range cr.actions {
			if err := action.Execute(ctx, path); err != nil {
				slog.Error("action failed", "rule", cr.name, "path", path, "error", err)
				return fmt.Errorf("rule %q action failed: %w", cr.name, err)
			}
		}

		return nil // first match wins
	}

	slog.Debug("no rule matched", "path", path)
	return nil
}
