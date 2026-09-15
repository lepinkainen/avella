package ssh

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/avella/config"
)

// listenUnix creates a listening unix socket and returns its path.
func listenUnix(t *testing.T) string {
	t.Helper()
	// Short base dir: unix socket paths are capped near 104 bytes on macOS.
	dir, err := os.MkdirTemp("/tmp", "avella")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sock := filepath.Join(dir, "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen %s: %v", sock, err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return sock
}

func TestAuthMethodAgentSockOverridesEnv(t *testing.T) {
	sock := listenUnix(t)
	t.Setenv("SSH_AUTH_SOCK", "/nonexistent/agent.sock")

	p := NewPool(nil)
	if _, err := p.authMethod(config.SSH{AgentSock: sock}); err != nil {
		t.Fatalf("agent_sock should be used instead of SSH_AUTH_SOCK: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAuthMethodFallsBackToEnv(t *testing.T) {
	sock := listenUnix(t)
	t.Setenv("SSH_AUTH_SOCK", sock)

	p := NewPool(nil)
	if _, err := p.authMethod(config.SSH{}); err != nil {
		t.Fatalf("SSH_AUTH_SOCK fallback: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAuthMethodNoAgentConfigured(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	p := NewPool(nil)
	if _, err := p.authMethod(config.SSH{}); err == nil {
		t.Fatal("expected error when no key, agent_sock or SSH_AUTH_SOCK")
	}
}
