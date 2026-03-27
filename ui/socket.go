package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const defaultSocketDir = ".cache/avella"
const socketFileName = "avella.sock"

// ProtocolVersion is the current socket protocol version.
// Bump this when the protocol changes in incompatible ways.
const ProtocolVersion = 1

// errDaemonRunning is returned when another daemon instance owns the socket.
var errDaemonRunning = errors.New("another avella daemon is already running")

// socketMessage is the envelope for all messages on the wire.
type socketMessage struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data,omitempty"`
	Command string          `json:"command,omitempty"`
}

const maxRecentFiles = 10

// state is the full snapshot pushed to clients.
type state struct {
	Status      string       `json:"status"`
	Processed   int          `json:"processed"`
	DryRun      bool         `json:"dry_run"`
	ConfigPath  string       `json:"config_path"`
	Rules       []RuleInfo   `json:"rules"`
	Version     string       `json:"version"`
	RecentFiles []RecentFile `json:"recent_files"`
}

// SocketUI implements UI over a Unix domain socket.
type SocketUI struct {
	mu       sync.Mutex
	st       state
	clients  map[net.Conn]struct{}
	listener net.Listener
	cancel   context.CancelFunc
	sockPath string

	onDryToggle func(bool)
}

// NewSocket returns a socket-backed UI implementation.
func NewSocket() *SocketUI {
	return &SocketUI{
		clients: make(map[net.Conn]struct{}),
	}
}

// SetRules stores rule info for display. Call before Run.
func (u *SocketUI) SetRules(rules []RuleInfo) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.Rules = rules
}

// SetDryRunToggle configures the dry-run toggle. Call before Run.
func (u *SocketUI) SetDryRunToggle(initial bool, onToggle func(bool)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.DryRun = initial
	u.onDryToggle = onToggle
}

// SetConfigPath stores the config file path. Call before Run.
func (u *SocketUI) SetConfigPath(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.ConfigPath = path
}

// SetVersion stores the daemon version. Call before Run.
func (u *SocketUI) SetVersion(version string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.Version = version
}

// SetStatus updates the current status and broadcasts to clients.
func (u *SocketUI) SetStatus(status string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.Status = status
	u.broadcastLocked()
}

// IncProcessed increments the processed file counter and broadcasts.
func (u *SocketUI) IncProcessed() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.Processed++
	u.broadcastLocked()
}

// AddRecentFile prepends a file to the recent files list and broadcasts.
func (u *SocketUI) AddRecentFile(file RecentFile) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.st.RecentFiles = append([]RecentFile{file}, u.st.RecentFiles...)
	if len(u.st.RecentFiles) > maxRecentFiles {
		u.st.RecentFiles = u.st.RecentFiles[:maxRecentFiles]
	}
	u.broadcastLocked()
}

// Run starts the socket server and daemon, blocking until ctx is done.
func (u *SocketUI) Run(ctx context.Context, cancel context.CancelFunc, daemon func(context.Context)) {
	u.cancel = cancel

	sockPath, resolveErr := u.resolveSocketPath()
	if resolveErr != nil {
		slog.Error("failed to resolve socket path", "error", resolveErr)
		daemon(ctx)
		return
	}
	u.sockPath = sockPath

	// Clean up stale socket file or detect a running daemon.
	if staleErr := removeStaleSocket(sockPath); staleErr != nil {
		if errors.Is(staleErr, errDaemonRunning) {
			slog.Error("cannot start: another daemon is already running", "path", sockPath)
			cancel()
			return
		}
		slog.Error("failed to remove stale socket", "path", sockPath, "error", staleErr)
		daemon(ctx)
		return
	}

	ln, listenErr := net.Listen("unix", sockPath)
	if listenErr != nil {
		slog.Error("failed to listen on socket", "path", sockPath, "error", listenErr)
		daemon(ctx)
		return
	}
	u.listener = ln

	// Restrict socket permissions.
	if chmodErr := os.Chmod(sockPath, 0o600); chmodErr != nil {
		slog.Warn("failed to chmod socket", "path", sockPath, "error", chmodErr)
	}

	slog.Info("UI socket listening", "path", sockPath)

	// Set initial status.
	u.mu.Lock()
	if u.st.Status == "" {
		u.st.Status = "Idle"
	}
	u.mu.Unlock()

	go u.acceptLoop(ctx)
	go daemon(ctx)

	<-ctx.Done()
	u.cleanup()
}

func (u *SocketUI) resolveSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, defaultSocketDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create socket dir: %w", err)
	}
	return filepath.Join(dir, socketFileName), nil
}

func removeStaleSocket(path string) error {
	// Check if something is already listening.
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("socket %s is active: %w", path, errDaemonRunning)
	}
	// Socket file exists but nothing is listening — remove it.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func (u *SocketUI) acceptLoop(ctx context.Context) {
	for {
		conn, err := u.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("socket accept error", "error", err)
			continue
		}
		slog.Debug("tray client connected", "remote", conn.RemoteAddr())
		u.mu.Lock()
		u.clients[conn] = struct{}{}
		u.mu.Unlock()

		// Send hello handshake and initial state outside the lock.
		u.sendHelloAndState(conn)

		go u.handleClient(ctx, conn)
	}
}

func (u *SocketUI) handleClient(ctx context.Context, conn net.Conn) {
	defer func() {
		u.mu.Lock()
		delete(u.clients, conn)
		u.mu.Unlock()
		_ = conn.Close()
		slog.Debug("tray client disconnected")
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		var msg socketMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			slog.Warn("invalid message from tray client", "error", err)
			continue
		}
		u.handleCommand(msg.Command)
	}
}

func (u *SocketUI) handleCommand(cmd string) {
	switch cmd {
	case "toggle_dry_run":
		u.mu.Lock()
		u.st.DryRun = !u.st.DryRun
		enabled := u.st.DryRun
		cb := u.onDryToggle
		u.broadcastLocked()
		u.mu.Unlock()
		if enabled {
			slog.Info("dry-run mode enabled via tray")
		} else {
			slog.Info("dry-run mode disabled via tray")
		}
		if cb != nil {
			cb(enabled)
		}

	case "open_config":
		u.mu.Lock()
		path := u.st.ConfigPath
		u.mu.Unlock()
		if path != "" {
			if err := exec.Command("open", path).Start(); err != nil {
				slog.Error("failed to open config file", "path", path, "error", err)
			}
		}

	case "quit":
		slog.Info("quit requested via tray")
		if u.cancel != nil {
			u.cancel()
		}

	default:
		slog.Warn("unknown command from tray client", "command", cmd)
	}
}

// broadcastLocked snapshots state and clients under the lock, then writes
// outside the lock so slow clients cannot block state mutations.
// Must be called with u.mu held; the lock is temporarily released during I/O.
func (u *SocketUI) broadcastLocked() {
	line := u.marshalStateLine()
	if line == nil {
		return
	}
	clients := make([]net.Conn, 0, len(u.clients))
	for conn := range u.clients {
		clients = append(clients, conn)
	}

	u.mu.Unlock()
	failed := writeToClients(clients, line)
	u.mu.Lock()

	for _, conn := range failed {
		delete(u.clients, conn)
		_ = conn.Close()
	}
}

// marshalStateLine returns the JSON-encoded state message with a trailing
// newline, ready to write to a client. Must be called with u.mu held.
func (u *SocketUI) marshalStateLine() []byte {
	data, err := json.Marshal(u.st)
	if err != nil {
		slog.Error("failed to marshal state", "error", err)
		return nil
	}
	msg := socketMessage{Type: "state", Data: data}
	line, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal message", "error", err)
		return nil
	}
	return append(line, '\n')
}

// writeToClients writes line to each conn and returns any that failed.
func writeToClients(clients []net.Conn, line []byte) []net.Conn {
	var failed []net.Conn
	for _, conn := range clients {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, writeErr := conn.Write(line); writeErr != nil {
			slog.Debug("failed to write to tray client", "error", writeErr)
			failed = append(failed, conn)
		}
		_ = conn.SetWriteDeadline(time.Time{})
	}
	return failed
}

// sendHelloAndState writes the hello handshake and initial state to a new
// client. Called without holding u.mu; acquires the lock to read state.
func (u *SocketUI) sendHelloAndState(conn net.Conn) {
	hello := socketMessage{Type: "hello", Data: json.RawMessage(
		fmt.Sprintf(`{"protocol_version":%d}`, ProtocolVersion),
	)}
	helloLine, err := json.Marshal(hello)
	if err != nil {
		slog.Error("failed to marshal hello", "error", err)
		return
	}
	helloLine = append(helloLine, '\n')

	u.mu.Lock()
	stateLine := u.marshalStateLine()
	u.mu.Unlock()

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, writeErr := conn.Write(helloLine); writeErr != nil {
		slog.Debug("failed to write hello to tray client", "error", writeErr)
		u.removeClient(conn)
		return
	}
	if stateLine != nil {
		if _, writeErr := conn.Write(stateLine); writeErr != nil {
			slog.Debug("failed to write state to tray client", "error", writeErr)
			u.removeClient(conn)
			return
		}
	}
	_ = conn.SetWriteDeadline(time.Time{})
}

// removeClient removes a connection from the client set and closes it.
func (u *SocketUI) removeClient(conn net.Conn) {
	u.mu.Lock()
	delete(u.clients, conn)
	u.mu.Unlock()
	_ = conn.Close()
}

func (u *SocketUI) cleanup() {
	if u.listener != nil {
		_ = u.listener.Close()
	}
	u.mu.Lock()
	for conn := range u.clients {
		_ = conn.Close()
	}
	u.clients = make(map[net.Conn]struct{})
	u.mu.Unlock()
	if u.sockPath != "" {
		_ = os.Remove(u.sockPath)
	}
	slog.Debug("UI socket cleaned up")
}
