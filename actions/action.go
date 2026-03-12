package actions

import (
	"context"
	"fmt"

	"github.com/lepinkainen/avella/config"
	"github.com/lepinkainen/avella/ssh"
)

// Action is the interface for all file actions.
type Action interface {
	Execute(ctx context.Context, path string) error
}

// Describer is optionally implemented by actions that can describe
// their resolved destination for a specific file (e.g. after template expansion).
type Describer interface {
	Describe(path string) string
}

// FromConfig creates an Action from an ActionConfig.
// The sshPool may be nil if no SSH hosts are configured.
func FromConfig(ac config.ActionConfig, sshPool *ssh.Pool) (Action, error) {
	switch {
	case ac.Move != nil:
		return &MoveAction{Dest: ac.Move.Dest}, nil
	case ac.Exec != nil:
		return &ExecAction{Command: ac.Exec.Command}, nil
	case ac.SCP != nil:
		if sshPool == nil {
			return nil, fmt.Errorf("scp action requires ssh_hosts configuration")
		}
		return &SCPAction{Host: ac.SCP.Host, Dest: ac.SCP.Dest, Pool: sshPool, DeleteSource: ac.SCP.DeleteSource}, nil
	case ac.ValidateZip != nil:
		return &ValidateZipAction{Full: ac.ValidateZip.Mode == "full"}, nil
	case ac.Notify != nil:
		return &NotifyAction{Message: ac.Notify.Message}, nil
	default:
		return nil, fmt.Errorf("no action type specified")
	}
}
