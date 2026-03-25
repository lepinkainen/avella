package rules

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/lepinkainen/avella/actions"
	"github.com/lepinkainen/avella/config"
	"github.com/lepinkainen/avella/ssh"
)

type compiledRule struct {
	name      string
	match     config.MatchRule
	regex     *regexp.Regexp
	minAge    time.Duration
	actions   []actions.Action
	onSuccess []actions.Action
	onFail    []actions.Action
}

type compiledIgnore struct {
	name   string
	match  config.MatchRule
	regex  *regexp.Regexp
	minAge time.Duration
}

// Engine evaluates files against configured rules.
type Engine struct {
	mu      sync.Mutex
	rules   []compiledRule
	ignores []compiledIgnore
	dryRun  bool
}

// SetDryRun enables or disables dry-run mode at runtime.
func (e *Engine) SetDryRun(enabled bool) {
	e.mu.Lock()
	e.dryRun = enabled
	e.mu.Unlock()
}

// DryRun returns whether dry-run mode is currently enabled.
func (e *Engine) DryRun() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dryRun
}

// compileMinAge parses min_age string; falls back to min_age_seconds.
func compileMinAge(m config.MatchRule) time.Duration {
	if m.MinAge != "" {
		if d, err := time.ParseDuration(m.MinAge); err == nil {
			return d
		}
	}
	if m.MinAgeSeconds > 0 {
		return time.Duration(m.MinAgeSeconds) * time.Second
	}
	return 0
}

// NewEngine creates an Engine with pre-compiled regexes and actions for all rules.
// The sshPool may be nil if no SSH hosts are configured.
func NewEngine(cfgRules []config.Rule, ignored map[string]config.IgnoreRule, sshPool *ssh.Pool, dryRun bool) (*Engine, error) {
	compiled := make([]compiledRule, len(cfgRules))
	for i, r := range cfgRules {
		cr := compiledRule{
			name:   r.Name,
			match:  r.Match,
			minAge: compileMinAge(r.Match),
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

		for j, ac := range r.OnSuccess {
			action, err := actions.FromConfig(ac, sshPool)
			if err != nil {
				return nil, fmt.Errorf("rule %q on_success %d: %w", r.Name, j, err)
			}
			cr.onSuccess = append(cr.onSuccess, action)
		}

		for j, ac := range r.OnFail {
			action, err := actions.FromConfig(ac, sshPool)
			if err != nil {
				return nil, fmt.Errorf("rule %q on_fail %d: %w", r.Name, j, err)
			}
			cr.onFail = append(cr.onFail, action)
		}

		compiled[i] = cr
	}

	var compiledIgnores []compiledIgnore
	for name, ig := range ignored {
		ci := compiledIgnore{
			name:   name,
			match:  ig.Match,
			minAge: compileMinAge(ig.Match),
		}
		if ig.Match.FilenameRegex != "" {
			re, err := regexp.Compile(ig.Match.FilenameRegex)
			if err != nil {
				return nil, fmt.Errorf("ignored %q: compile regex: %w", name, err)
			}
			ci.regex = re
		}
		compiledIgnores = append(compiledIgnores, ci)
	}

	return &Engine{rules: compiled, ignores: compiledIgnores, dryRun: dryRun}, nil
}

// ShouldIgnore returns true if the file matches any configured ignore rule.
func (e *Engine) ShouldIgnore(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	for _, ci := range e.ignores {
		if Matches(path, info, ci.match, ci.regex, ci.minAge) {
			slog.Debug("ignoring file", "rule", ci.name, "path", path)
			return true
		}
	}
	return false
}

// ProcessResult describes what happened when a file was processed.
type ProcessResult struct {
	Matched  bool     // true if a rule matched
	RuleName string   // name of the matched rule
	Actions  []string // human-readable description of each action
	DryRun   bool     // true if dry-run mode was active
}

// Process evaluates a file against all rules. First match wins.
func (e *Engine) Process(ctx context.Context, path string) (ProcessResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("stat %s: %w", path, err)
	}

	dryRun := e.DryRun()

	for _, cr := range e.rules {
		select {
		case <-ctx.Done():
			return ProcessResult{}, ctx.Err()
		default:
		}

		if !Matches(path, info, cr.match, cr.regex, cr.minAge) {
			continue
		}

		slog.Info("rule matched", "rule", cr.name, "path", path)

		if len(cr.actions) == 0 {
			continue // no actions — pass-through rule (e.g. validation-only)
		}

		// Collect descriptions before executing (file may be moved/gone after).
		descs := describeActions(cr.actions, path)

		result := ProcessResult{
			Matched:  true,
			RuleName: cr.name,
			Actions:  descs,
			DryRun:   dryRun,
		}

		if dryRun {
			logDryRun(cr.name, "action", cr.actions, path)
			logDryRun(cr.name, "on_success", cr.onSuccess, path)
			logDryRun(cr.name, "on_fail", cr.onFail, path)
			return result, nil
		}

		var actionErr error
		for _, action := range cr.actions {
			if execErr := action.Execute(ctx, path); execErr != nil {
				slog.Error("action failed", "rule", cr.name, "path", path, "error", execErr)
				actionErr = fmt.Errorf("rule %q action failed: %w", cr.name, execErr)
				break
			}
		}

		if actionErr != nil {
			runHooks(ctx, cr.name, "on_fail", cr.onFail, path)
			return result, actionErr
		}

		runHooks(ctx, cr.name, "on_success", cr.onSuccess, path)
		return result, nil // first match wins
	}

	slog.Debug("no rule matched", "path", path)
	return ProcessResult{}, nil
}

func describeActions(acts []actions.Action, path string) []string {
	descs := make([]string, len(acts))
	for i, action := range acts {
		desc := fmt.Sprint(action)
		if d, ok := action.(actions.Describer); ok {
			desc = d.Describe(path)
		}
		descs[i] = desc
	}
	return descs
}

func logDryRun(ruleName, list string, acts []actions.Action, path string) {
	for _, action := range acts {
		desc := fmt.Sprint(action)
		if d, ok := action.(actions.Describer); ok {
			desc = d.Describe(path)
		}
		slog.Info("[dry-run] would execute", "rule", ruleName, "list", list, "action", desc, "path", path)
	}
}

func runHooks(ctx context.Context, ruleName, list string, acts []actions.Action, path string) {
	for _, action := range acts {
		if err := action.Execute(ctx, path); err != nil {
			slog.Error("hook action failed", "rule", ruleName, "list", list, "path", path, "error", err)
		}
	}
}
