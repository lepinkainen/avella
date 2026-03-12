//go:build systray

package ui

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed icon.png
var iconData []byte

// SystrayUI provides a macOS menu bar icon with status display.
type SystrayUI struct {
	mu             sync.Mutex
	statusItem     *systray.MenuItem
	processedItem  *systray.MenuItem
	processedCount int
	cancel         context.CancelFunc

	// Set before Run via SetRules / SetDryRunToggle.
	rules          []RuleInfo
	dryRunEnabled  bool
	onDryRunToggle func(bool)
}

// New returns a systray-backed UI implementation when built with systray support.
func New() UI { return NewSystray() }

// NewSystray returns a systray-backed UI implementation.
func NewSystray() UI { return &SystrayUI{} }

// SetRules stores rule info for display in the menu. Call before Run.
func (u *SystrayUI) SetRules(rules []RuleInfo) {
	u.rules = rules
}

// SetDryRunToggle configures the dry-run toggle. Call before Run.
func (u *SystrayUI) SetDryRunToggle(initial bool, onToggle func(bool)) {
	u.dryRunEnabled = initial
	u.onDryRunToggle = onToggle
}

func (u *SystrayUI) Run(ctx context.Context, cancel context.CancelFunc, daemon func(context.Context)) {
	u.cancel = cancel

	// Start the daemon in a background goroutine — systray needs the main thread.
	go daemon(ctx)

	systray.Run(u.onReady(ctx), u.onExit)
}

func (u *SystrayUI) onReady(ctx context.Context) func() {
	return func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Avella")
		systray.SetTooltip("Avella — file automation daemon")

		// Status line.
		u.mu.Lock()
		u.statusItem = systray.AddMenuItem("Idle", "Current status")
		u.statusItem.Disable()

		u.processedItem = systray.AddMenuItem("Processed: 0 files", "Total files processed")
		u.processedItem.Disable()
		u.mu.Unlock()

		systray.AddSeparator()

		// Dry-run toggle.
		dryRunItem := systray.AddMenuItem("Dry-run mode", "Toggle dry-run mode")
		if u.dryRunEnabled {
			dryRunItem.Check()
		}

		systray.AddSeparator()

		// Rules submenu.
		rulesParent := systray.AddMenuItem("Rules", "Configured rules")
		for _, r := range u.rules {
			label := fmt.Sprintf("%s: %s", r.Name, r.ActionType)
			sub := rulesParent.AddSubMenuItem(label, "")
			sub.Disable()
		}

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop Avella")

		go func() {
			for {
				select {
				case <-dryRunItem.ClickedCh:
					u.mu.Lock()
					u.dryRunEnabled = !u.dryRunEnabled
					enabled := u.dryRunEnabled
					u.mu.Unlock()

					if enabled {
						dryRunItem.Check()
						slog.Info("dry-run mode enabled via menu")
					} else {
						dryRunItem.Uncheck()
						slog.Info("dry-run mode disabled via menu")
					}
					if u.onDryRunToggle != nil {
						u.onDryRunToggle(enabled)
					}
				case <-quit.ClickedCh:
					u.cancel()
					systray.Quit()
					return
				case <-ctx.Done():
					systray.Quit()
					return
				}
			}
		}()
	}
}

func (u *SystrayUI) onExit() {}

func (u *SystrayUI) SetStatus(status string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.statusItem != nil {
		u.statusItem.SetTitle(status)
	}
}

// IncProcessed increments the processed file counter.
func (u *SystrayUI) IncProcessed() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.processedCount++
	if u.processedItem != nil {
		u.processedItem.SetTitle(fmt.Sprintf("Processed: %d files", u.processedCount))
	}
}
