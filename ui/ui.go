package ui

import "context"

// RuleInfo describes a rule for display in the UI.
type RuleInfo struct {
	Name       string
	ActionType string // e.g. "move", "scp", "move+exec"
}

// UI abstracts the optional system tray interface.
// The daemon function contains the core application logic.
// Run decides how to schedule it (inline for headless, in a goroutine for systray).
type UI interface {
	Run(ctx context.Context, cancel context.CancelFunc, daemon func(context.Context))
	SetStatus(status string)
	SetRules(rules []RuleInfo)
	SetDryRunToggle(initial bool, onToggle func(bool))
	IncProcessed()
}

// NoopUI runs the daemon directly with no UI overhead.
type NoopUI struct{}

// NewNoop returns a headless UI implementation.
func NewNoop() UI { return &NoopUI{} }

// Run executes the daemon function directly on the current goroutine.
func (u *NoopUI) Run(ctx context.Context, _ context.CancelFunc, daemon func(context.Context)) {
	daemon(ctx)
}

// SetStatus is a no-op in headless mode.
func (u *NoopUI) SetStatus(_ string) {}

// SetRules is a no-op in headless mode.
func (u *NoopUI) SetRules(_ []RuleInfo) {}

// SetDryRunToggle is a no-op in headless mode.
func (u *NoopUI) SetDryRunToggle(_ bool, _ func(bool)) {}

// IncProcessed is a no-op in headless mode.
func (u *NoopUI) IncProcessed() {}
