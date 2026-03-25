package ui

import "context"

// RuleInfo describes a rule for display in the UI.
type RuleInfo struct {
	Name       string `json:"name"`
	ActionType string `json:"action_type"` // e.g. "move", "scp", "move+exec"
}

// RecentFile describes a recently processed file for display.
type RecentFile struct {
	Filename string `json:"filename"` // base filename
	Rule     string `json:"rule"`     // matched rule name
	Action   string `json:"action"`   // human-readable action description
	DryRun   bool   `json:"dry_run"`  // was this a dry-run match
	Time     string `json:"time"`     // RFC 3339 timestamp
}

// UI abstracts the optional system tray interface.
// The daemon function contains the core application logic.
// Run decides how to schedule it (inline for headless, in a goroutine for systray).
type UI interface {
	Run(ctx context.Context, cancel context.CancelFunc, daemon func(context.Context))
	SetStatus(status string)
	SetRules(rules []RuleInfo)
	SetDryRunToggle(initial bool, onToggle func(bool))
	SetConfigPath(path string)
	SetVersion(version string)
	IncProcessed()
	AddRecentFile(file RecentFile)
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

// SetConfigPath is a no-op in headless mode.
func (u *NoopUI) SetConfigPath(_ string) {}

// SetVersion is a no-op in headless mode.
func (u *NoopUI) SetVersion(_ string) {}

// IncProcessed is a no-op in headless mode.
func (u *NoopUI) IncProcessed() {}

// AddRecentFile is a no-op in headless mode.
func (u *NoopUI) AddRecentFile(_ RecentFile) {}
