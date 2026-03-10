package ui

import "context"

// UI abstracts the optional system tray interface.
// The daemon function contains the core application logic.
// Run decides how to schedule it (inline for headless, in a goroutine for systray).
type UI interface {
	Run(ctx context.Context, cancel context.CancelFunc, daemon func(context.Context))
	SetStatus(status string)
}
