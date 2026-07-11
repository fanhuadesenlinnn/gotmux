package server

import (
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func waitForTestCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition was not satisfied within %s", timeout)
}

func assertPaneCommand(t *testing.T, state *model.Server, command []string) {
	t.Helper()
	for _, session := range snapshotSessions(state) {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if reflect.DeepEqual(pane.Command, command) {
					return
				}
			}
		}
	}
	t.Fatalf("missing pane command %#v", command)
}

func attachTestRuntimeClient(t *testing.T, rt *Runtime, sessionName string) (model.Client, <-chan protocol.Message) {
	t.Helper()
	if rt.clients == nil {
		rt.clients = make(map[int64]*attachedClient)
	}
	client, _, err := rt.state.AttachClient(sessionName, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	clientProtocol := protocol.NewConn(clientConn)
	messages := make(chan protocol.Message, 8)
	go func() {
		defer close(messages)
		for {
			msg, err := clientProtocol.Read()
			if err != nil {
				return
			}
			messages <- msg
		}
	}()
	rt.clients[client.ID] = &attachedClient{id: client.ID, conn: protocol.NewConn(serverConn), done: make(chan struct{})}
	t.Cleanup(func() {
		_ = serverConn.Close()
		_ = clientConn.Close()
		delete(rt.clients, client.ID)
		rt.state.DetachClient(client.ID)
	})
	return *client, messages
}

func expectNoProtocolMessage(t *testing.T, messages <-chan protocol.Message) {
	t.Helper()
	select {
	case msg := <-messages:
		t.Fatalf("unexpected protocol message: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func waitForProtocolState(t *testing.T, messages <-chan protocol.Message, timeout time.Duration, fn func(protocol.Message) bool) protocol.Message {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				t.Fatal("protocol stream closed")
			}
			if fn(msg) {
				return msg
			}
		case <-timer.C:
			t.Fatalf("protocol condition was not satisfied within %s", timeout)
		}
	}
}

func readProtocolMessage(t *testing.T, conn net.Conn, reader *protocol.Conn) protocol.Message {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	msg, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func waitFileContains(t *testing.T, path string, needle string) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), needle) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func assertPanesFormat(t *testing.T, rt *Runtime, sessionName string, want string) {
	t.Helper()
	got := listPanesFormat(rt.state, sessionName, "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}")
	if got != want {
		t.Fatalf("pane geometry = %q, want %q", got, want)
	}
}
