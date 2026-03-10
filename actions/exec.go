package actions

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// ExecAction runs a command with the file path as an argument.
type ExecAction struct {
	Command string
}

func (a *ExecAction) String() string { return fmt.Sprintf("exec → %s", a.Command) }

// Execute runs the configured command with the file path as an argument.
func (a *ExecAction) Execute(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, a.Command, path)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		slog.Info("command output", "command", a.Command, "path", path, "output", string(output))
	}

	if err != nil {
		return fmt.Errorf("exec %s %s: %w", a.Command, path, err)
	}

	slog.Info("executed command", "command", a.Command, "path", path)
	return nil
}
