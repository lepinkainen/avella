package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startTestServer creates a SocketUI listening on a temp socket and returns a
// cleanup function. The daemon callback is a no-op that blocks on ctx.
func startTestServer(t *testing.T) (*SocketUI, string, context.CancelFunc) {
	t.Helper()
	// Use /tmp directly to keep path under macOS 104-char Unix socket limit.
	dir, err := os.MkdirTemp("/tmp", "avella-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "test.sock")

	u := NewSocket()
	u.sockPath = sockPath
	u.st = state{
		Status:     "Idle",
		Processed:  0,
		DryRun:     false,
		ConfigPath: "/tmp/test.yaml",
		Rules:      []RuleInfo{{Name: "TestRule", ActionType: "move"}},
		Version:    "test",
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	u.listener = ln

	ctx, cancel := context.WithCancel(context.Background())
	go u.acceptLoop(ctx)

	t.Cleanup(func() {
		cancel()
		ln.Close()
		os.Remove(sockPath)
	})

	return u, sockPath, cancel
}

func dial(t *testing.T, sockPath string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readMessage(t *testing.T, conn net.Conn) socketMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("expected message, got none:", scanner.Err())
	}
	var msg socketMessage
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		t.Fatal("unmarshal:", err)
	}
	return msg
}

func readState(t *testing.T, conn net.Conn) state {
	t.Helper()
	msg := readMessage(t, conn)
	if msg.Type != "state" {
		t.Fatalf("expected type=state, got %q", msg.Type)
	}
	var st state
	if err := json.Unmarshal(msg.Data, &st); err != nil {
		t.Fatal("unmarshal data:", err)
	}
	return st
}

func TestSnapshotOnConnect(t *testing.T) {
	_, sockPath, _ := startTestServer(t)
	conn := dial(t, sockPath)

	st := readState(t, conn)
	if st.Status != "Idle" {
		t.Errorf("status = %q, want Idle", st.Status)
	}
	if st.ConfigPath != "/tmp/test.yaml" {
		t.Errorf("config_path = %q, want /tmp/test.yaml", st.ConfigPath)
	}
	if len(st.Rules) != 1 || st.Rules[0].Name != "TestRule" {
		t.Errorf("rules = %v, want [{TestRule move}]", st.Rules)
	}
}

func TestStateUpdateBroadcast(t *testing.T) {
	u, sockPath, _ := startTestServer(t)
	conn := dial(t, sockPath)

	// Consume initial snapshot.
	_ = readState(t, conn)

	// Trigger a state change.
	u.SetStatus("Processing test.txt")

	st := readState(t, conn)
	if st.Status != "Processing test.txt" {
		t.Errorf("status = %q, want 'Processing test.txt'", st.Status)
	}
}

func TestIncProcessedBroadcast(t *testing.T) {
	u, sockPath, _ := startTestServer(t)
	conn := dial(t, sockPath)

	_ = readState(t, conn)

	u.IncProcessed()
	st := readState(t, conn)
	if st.Processed != 1 {
		t.Errorf("processed = %d, want 1", st.Processed)
	}

	u.IncProcessed()
	st = readState(t, conn)
	if st.Processed != 2 {
		t.Errorf("processed = %d, want 2", st.Processed)
	}
}

func TestToggleDryRunCommand(t *testing.T) {
	u, sockPath, _ := startTestServer(t)

	var toggled bool
	var toggledTo bool
	u.onDryToggle = func(enabled bool) {
		toggled = true
		toggledTo = enabled
	}

	conn := dial(t, sockPath)
	_ = readState(t, conn)

	// Send toggle command.
	cmd := `{"type":"command","command":"toggle_dry_run"}` + "\n"
	if _, err := conn.Write([]byte(cmd)); err != nil {
		t.Fatal(err)
	}

	// Read the state update triggered by the toggle.
	st := readState(t, conn)
	if !st.DryRun {
		t.Error("expected dry_run=true after toggle")
	}
	// Give the handler goroutine a moment to invoke the callback.
	time.Sleep(50 * time.Millisecond)
	if !toggled || !toggledTo {
		t.Error("expected onDryToggle callback to be called with true")
	}
}

func TestClientDisconnectDoesNotCrash(t *testing.T) {
	u, sockPath, _ := startTestServer(t)

	conn := dial(t, sockPath)
	_ = readState(t, conn)
	conn.Close()

	// Give the server a moment to process the disconnect.
	time.Sleep(50 * time.Millisecond)

	// Server should still work — set status shouldn't panic.
	u.SetStatus("still running")

	// A new client should be able to connect.
	conn2 := dial(t, sockPath)
	st := readState(t, conn2)
	if st.Status != "still running" {
		t.Errorf("status = %q, want 'still running'", st.Status)
	}
}

func TestMultipleClients(t *testing.T) {
	u, sockPath, _ := startTestServer(t)

	conn1 := dial(t, sockPath)
	conn2 := dial(t, sockPath)

	_ = readState(t, conn1)
	_ = readState(t, conn2)

	u.SetStatus("multi-test")

	st1 := readState(t, conn1)
	st2 := readState(t, conn2)
	if st1.Status != "multi-test" {
		t.Errorf("client1 status = %q, want 'multi-test'", st1.Status)
	}
	if st2.Status != "multi-test" {
		t.Errorf("client2 status = %q, want 'multi-test'", st2.Status)
	}
}

func TestAddRecentFileBroadcast(t *testing.T) {
	u, sockPath, _ := startTestServer(t)
	conn := dial(t, sockPath)

	_ = readState(t, conn)

	u.AddRecentFile(RecentFile{
		Filename: "test.mp4",
		Rule:     "videos",
		Action:   "move → ~/Videos",
		DryRun:   false,
		Time:     "2026-03-25T12:00:00Z",
	})

	st := readState(t, conn)
	if len(st.RecentFiles) != 1 {
		t.Fatalf("recent_files len = %d, want 1", len(st.RecentFiles))
	}
	rf := st.RecentFiles[0]
	if rf.Filename != "test.mp4" {
		t.Errorf("filename = %q, want test.mp4", rf.Filename)
	}
	if rf.Rule != "videos" {
		t.Errorf("rule = %q, want videos", rf.Rule)
	}
	if rf.Action != "move → ~/Videos" {
		t.Errorf("action = %q, want 'move → ~/Videos'", rf.Action)
	}
}

func TestRecentFilesCapsAtTen(t *testing.T) {
	u, sockPath, _ := startTestServer(t)
	conn := dial(t, sockPath)

	_ = readState(t, conn)

	// Add 12 files.
	for i := range 12 {
		u.AddRecentFile(RecentFile{
			Filename: fmt.Sprintf("file%d.txt", i),
			Rule:     "test",
			Action:   "move",
			Time:     "2026-03-25T12:00:00Z",
		})
		_ = readState(t, conn) // consume broadcast
	}

	// Check final state has exactly 10, most recent first.
	u.SetStatus("check") // trigger one more broadcast
	st := readState(t, conn)
	if len(st.RecentFiles) != 10 {
		t.Fatalf("recent_files len = %d, want 10", len(st.RecentFiles))
	}
	// Most recent (file11) should be first.
	if st.RecentFiles[0].Filename != "file11.txt" {
		t.Errorf("first recent = %q, want file11.txt", st.RecentFiles[0].Filename)
	}
	// Oldest kept (file2) should be last.
	if st.RecentFiles[9].Filename != "file2.txt" {
		t.Errorf("last recent = %q, want file2.txt", st.RecentFiles[9].Filename)
	}
}
