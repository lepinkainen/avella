package actions

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/lepinkainen/avella/template"
)

// NotifyAction sends a macOS notification via osascript.
type NotifyAction struct {
	Message string
}

func (a *NotifyAction) String() string { return fmt.Sprintf("notify → %s", a.Message) }

// Describe returns the resolved notification message for a specific file.
func (a *NotifyAction) Describe(path string) string {
	msg, err := template.ResolveDest(a.Message, path)
	if err != nil {
		return a.String()
	}
	return fmt.Sprintf("notify → %s", msg)
}

// Execute sends a macOS notification with the expanded message.
func (a *NotifyAction) Execute(_ context.Context, path string) error {
	msg, err := template.ResolveDest(a.Message, path)
	if err != nil {
		return fmt.Errorf("resolve notify message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	script := fmt.Sprintf(`display notification %q with title "Avella"`, msg)
	if err := exec.CommandContext(ctx, "osascript", "-e", script).Run(); err != nil {
		slog.Warn("notification failed", "error", err)
	}

	return nil
}
