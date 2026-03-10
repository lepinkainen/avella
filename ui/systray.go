//go:build systray

package ui

import (
	"context"
	_ "embed"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed icon.png
var iconData []byte

// SystrayUI provides a macOS menu bar icon with status display.
type SystrayUI struct {
	mu         sync.Mutex
	statusItem *systray.MenuItem
	cancel     context.CancelFunc
}

// New returns a systray-backed UI implementation.
func New() UI { return &SystrayUI{} }

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

		u.mu.Lock()
		u.statusItem = systray.AddMenuItem("Idle", "Current status")
		u.statusItem.Disable()
		u.mu.Unlock()

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop Avella")

		go func() {
			select {
			case <-quit.ClickedCh:
				u.cancel()
				systray.Quit()
			case <-ctx.Done():
				systray.Quit()
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
