package server

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func TestSendKeysTargetsPaneAndRepeats(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, first, err := rt.state.NewSession("sendt", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	firstRead, firstWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	secondRead, secondWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	first.PTY = firstWrite
	second.PTY = secondWrite
	msg := rt.execute([]string{"send-keys", "-N", "2", "-t", "sendt:.0", "A", "Enter"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("send-keys failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"send-keys", "-t", "sendt:.1", "C-c", "Up", "Delete", "F1", "Escape"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("send-keys special keys failed: %s", msg.Text)
	}
	_ = firstWrite.Close()
	_ = secondWrite.Close()
	firstData, _ := io.ReadAll(firstRead)
	secondData, _ := io.ReadAll(secondRead)
	if string(firstData) != "A\rA\r" {
		t.Fatalf("target pane data = %q, want repeated A enter", firstData)
	}
	if string(secondData) != "\x03\x1b[A\x1b[3~\x1bOP\x1b" {
		t.Fatalf("special key pane data = %q", secondData)
	}
	_ = firstRead.Close()
	_ = secondRead.Close()
}

func TestRootKeyBindingDispatch(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	msg := rt.execute([]string{"new-session", "-d", "-s", "root", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("root", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	msg = rt.execute([]string{"bind-key", "-n", "C-a", "new-window", "-n", "rooted", "/bin/sh"}, "root", 80, 24)
	if !msg.OK {
		t.Fatalf("bind-key -n failed: %s", msg.Text)
	}
	rt.handleInput(client.ID, []byte{0x01})
	msg = rt.execute([]string{"list-windows", "-t", "root", "-F", "#{window_index}:#{window_name}"}, "root", 80, 24)
	if !strings.Contains(msg.Text, "1:rooted") {
		t.Fatalf("root binding did not create window: %q", msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "-n", "F1", "new-window", "-n", "fkey", "/bin/sh"}, "root", 80, 24)
	if !msg.OK {
		t.Fatalf("bind-key F1 failed: %s", msg.Text)
	}
	rt.handleInput(client.ID, []byte("\x1bOP"))
	msg = rt.execute([]string{"list-windows", "-t", "root", "-F", "#{window_index}:#{window_name}"}, "root", 80, 24)
	if !strings.Contains(msg.Text, "2:fkey") {
		t.Fatalf("F1 root binding did not create window: %q", msg.Text)
	}
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "root"}, "root", 80, 24)
}

func TestDisplayMessageBindingShowsStatus(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	msg := rt.execute([]string{"new-session", "-d", "-s", "displaybind", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("displaybind", 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	clientProtocol := protocol.NewConn(clientConn)
	messages := make(chan protocol.Message, 64)
	go func() {
		for {
			msg, err := clientProtocol.Read()
			if err != nil {
				close(messages)
				return
			}
			messages <- msg
		}
	}()
	rt.clients[client.ID] = &attachedClient{id: client.ID, conn: protocol.NewConn(serverConn), done: make(chan struct{})}
	msg = rt.execute([]string{"bind-key", "-n", "C-a", "display-message", "hello #{session_name}"}, "displaybind", 80, 24)
	if !msg.OK {
		t.Fatalf("bind-key failed: %s", msg.Text)
	}
	rt.handleInput(client.ID, []byte{0x01})
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeStatus && bytes.Contains(next.Data, []byte("hello displaybind"))
	})
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "displaybind"}, "displaybind", 80, 24)
}

func TestPrefixKeyBindingsDispatch(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	msg := rt.execute([]string{"new-session", "-d", "-s", "prefixkeys", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("prefixkeys", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	rt.handleInput(client.ID, []byte{0x02, 'c'})
	windows := rt.execute([]string{"list-windows", "-t", "prefixkeys", "-F", "#{window_index}:#{window_active}"}, "prefixkeys", 80, 24)
	if windows.Text != "0:0\n1:1" {
		t.Fatalf("prefix c windows = %q", windows.Text)
	}
	messages := rt.execute([]string{"show-messages"}, "prefixkeys", 80, 24)
	if !strings.Contains(messages.Text, "client-1 key c: new-window") {
		t.Fatalf("prefix key message log = %q", messages.Text)
	}
	rt.handleInput(client.ID, []byte{0x02, '%'})
	panes := rt.execute([]string{"list-panes", "-t", "prefixkeys:1", "-F", "#{pane_index}:#{pane_active}"}, "prefixkeys", 80, 24)
	if panes.Text != "0:0\n1:1" {
		t.Fatalf("prefix %% panes = %q", panes.Text)
	}
	msg = rt.executeWithClient([]string{"display-panes"}, "prefixkeys", 80, 24, client.ID)
	if !msg.OK || msg.Text != "panes: 0 1" {
		t.Fatalf("display-panes = %#v", msg)
	}
	msg = rt.execute([]string{"set", "-g", "prefix", "C-a"}, "prefixkeys", 80, 24)
	if !msg.OK {
		t.Fatalf("set prefix failed: %s", msg.Text)
	}
	rt.handleInput(client.ID, []byte{0x01, 'c'})
	windows = rt.execute([]string{"list-windows", "-t", "prefixkeys", "-F", "#{window_index}:#{window_active}"}, "prefixkeys", 80, 24)
	if windows.Text != "0:0\n1:0\n2:1" {
		t.Fatalf("custom prefix c windows = %q", windows.Text)
	}
	msg = rt.execute([]string{"set", "-g", "prefix2", "C-z"}, "prefixkeys", 80, 24)
	if !msg.OK {
		t.Fatalf("set prefix2 failed: %s", msg.Text)
	}
	rt.handleInput(client.ID, []byte{0x1a, 'c'})
	windows = rt.execute([]string{"list-windows", "-t", "prefixkeys", "-F", "#{window_index}:#{window_active}"}, "prefixkeys", 80, 24)
	if windows.Text != "0:0\n1:0\n2:0\n3:1" {
		t.Fatalf("prefix2 c windows = %q", windows.Text)
	}
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "prefixkeys"}, "prefixkeys", 80, 24)
}

func TestPrefixDetachBindingSendsExit(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	msg := rt.execute([]string{"new-session", "-d", "-s", "prefixdetach", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("prefixdetach", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	rt.clients[client.ID] = &attachedClient{id: client.ID, conn: protocol.NewConn(serverConn), done: make(chan struct{})}
	reader := protocol.NewConn(clientConn)
	done := make(chan struct{})
	go func() {
		rt.handleInput(client.ID, []byte{0x02, 'd'})
		close(done)
	}()
	result, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	exit, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if result.Type != protocol.TypeResult || exit.Type != protocol.TypeExit {
		t.Fatalf("detach messages = %#v %#v", result, exit)
	}
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "prefixdetach"}, "prefixdetach", 80, 24)
}

func TestPrefixKillPaneBindingClosesSession(t *testing.T) {
	var once sync.Once
	stopped := make(chan struct{})
	rt := &Runtime{
		state:   model.NewServer("/tmp/gotmux-test.sock"),
		clients: make(map[int64]*attachedClient),
		stopServer: func() {
			once.Do(func() { close(stopped) })
		},
	}
	msg := rt.execute([]string{"new-session", "-d", "-s", "prefixkill", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("prefixkill", 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	clientProtocol := protocol.NewConn(clientConn)
	messages := make(chan protocol.Message, 64)
	go func() {
		for {
			msg, err := clientProtocol.Read()
			if err != nil {
				close(messages)
				return
			}
			messages <- msg
		}
	}()
	rt.clients[client.ID] = &attachedClient{id: client.ID, conn: protocol.NewConn(serverConn), done: make(chan struct{})}

	rt.handleInput(client.ID, []byte{0x02, 'x'})
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "session closed"
	})
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("server did not stop after prefix kill-pane removed the last session")
	}
}

func TestSelectLayoutSupportsBuiltinPrefix(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("layoutprefix", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	for i := 1; i < 5; i++ {
		if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
			t.Fatal(err)
		}
	}
	msg := rt.execute([]string{"select-layout", "til"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-layout prefix failed: %s", msg.Text)
	}
	got := listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}")
	want := "0:0:0:39:7\n1:40:0:40:7\n2:0:8:39:7\n3:40:8:40:7\n4:0:16:80:8"
	if got != want {
		t.Fatalf("tiled prefix geometry = %q", got)
	}
	_ = rt.execute([]string{"kill-session", "-t", "layoutprefix"}, "layoutprefix", 80, 24)
}
