//go:build !systray

package ui

import "context"

// NoopUI runs the daemon directly with no UI overhead.
type NoopUI struct{}

// New returns a headless UI implementation.
func New() UI { return &NoopUI{} }

// Run executes the daemon function directly on the current goroutine.
func (u *NoopUI) Run(ctx context.Context, _ context.CancelFunc, daemon func(context.Context)) {
	daemon(ctx)
}

// SetStatus is a no-op in headless mode.
func (u *NoopUI) SetStatus(_ string) {}
