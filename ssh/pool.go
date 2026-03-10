package ssh

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/lepinkainen/avella/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Pool manages cached SSH connections keyed by host alias.
type Pool struct {
	hosts map[string]config.SSH
	mu    sync.Mutex
	conns map[string]*ssh.Client
}

// NewPool creates a pool from the configured SSH hosts.
// Connections are established lazily on first use.
func NewPool(hosts map[string]config.SSH) *Pool {
	return &Pool{
		hosts: hosts,
		conns: make(map[string]*ssh.Client),
	}
}

// dial establishes an SSH connection to the named host.
// Must be called with p.mu held.
func (p *Pool) dial(name string) (*ssh.Client, error) {
	hostCfg, ok := p.hosts[name]
	if !ok {
		return nil, fmt.Errorf("unknown SSH host %q", name)
	}

	keyData, err := os.ReadFile(hostCfg.Key)
	if err != nil {
		return nil, fmt.Errorf("read SSH key %s: %w", hostCfg.Key, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse SSH key %s: %w", hostCfg.Key, err)
	}

	addr := hostCfg.Host
	// Default to port 22 if no port specified.
	if _, _, splitErr := net.SplitHostPort(addr); splitErr != nil {
		addr = net.JoinHostPort(addr, "22")
	}

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            hostCfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // user-configured hosts
	})
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s@%s: %w", hostCfg.User, addr, err)
	}

	slog.Info("SSH connected", "host", name, "addr", addr)
	return client, nil
}

// getConn returns a cached or new SSH connection for the named host.
func (p *Pool) getConn(name string) (*ssh.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if client, ok := p.conns[name]; ok {
		// Check if connection is still alive.
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return client, nil
		}
		// Stale connection — remove and reconnect.
		slog.Debug("SSH connection stale, reconnecting", "host", name)
		_ = client.Close()
		delete(p.conns, name)
	}

	client, err := p.dial(name)
	if err != nil {
		return nil, err
	}
	p.conns[name] = client
	return client, nil
}

// SFTP returns an SFTP client for the named host.
// The caller should close the SFTP client when done, but the
// underlying SSH connection is retained in the pool.
func (p *Pool) SFTP(name string) (*sftp.Client, error) {
	conn, err := p.getConn(name)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("SFTP session for %q: %w", name, err)
	}

	return sftpClient, nil
}

// Close closes all cached SSH connections.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for name, client := range p.conns {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close SSH %q: %w", name, err)
		}
		delete(p.conns, name)
	}
	return firstErr
}
