package server

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func TestCommandCreatesAndListsSession(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "work", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	list := rt.execute([]string{"list-sessions"}, "", 80, 24)
	if !list.OK || !strings.Contains(list.Text, "work: 1 windows") {
		t.Fatalf("list-sessions = %#v", list)
	}
	_ = rt.execute([]string{"kill-session", "-t", "work"}, "work", 80, 24)
}

func TestCommandRejectsDuplicateSession(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	first := rt.execute([]string{"new-session", "-d", "-s", "work", "/bin/sh"}, "", 80, 24)
	if !first.OK {
		t.Fatalf("new-session failed: %s", first.Text)
	}
	second := rt.execute([]string{"new-session", "-d", "-s", "work", "/bin/sh"}, "", 80, 24)
	if second.OK {
		t.Fatalf("duplicate session unexpectedly succeeded")
	}
	_ = rt.execute([]string{"kill-session", "-t", "work"}, "work", 80, 24)
}

func TestNewSessionAttachExisting(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	first := rt.execute([]string{"new-session", "-d", "-s", "work", "/bin/sh"}, "", 80, 24)
	if !first.OK {
		t.Fatalf("new-session failed: %s", first.Text)
	}
	second := rt.execute([]string{"new-session", "-A", "-d", "-P", "-F", "#{session_name}:#{session_windows}", "-s", "work", "/bin/sh"}, "", 80, 24)
	if !second.OK || second.Session != "work" || second.Text != "work:1" {
		t.Fatalf("new-session -A existing = %#v", second)
	}
	list := rt.execute([]string{"list-sessions", "-F", "#{session_name}:#{session_windows}"}, "", 80, 24)
	if list.Text != "work:1" {
		t.Fatalf("new-session -A created extra session/window: %q", list.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "work"}, "work", 80, 24)
}

func TestKillSessionAllButTarget(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	for _, name := range []string{"keep", "drop1", "drop2"} {
		msg := rt.execute([]string{"new-session", "-d", "-s", name, "/bin/sh"}, "", 80, 24)
		if !msg.OK {
			t.Fatalf("new-session %s failed: %s", name, msg.Text)
		}
	}
	msg := rt.execute([]string{"kill-session", "-a", "-t", "keep"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("kill-session -a = %#v", msg)
	}
	list := rt.execute([]string{"list-sessions", "-F", "#{session_name}"}, "", 80, 24)
	if list.Text != "keep" {
		t.Fatalf("remaining sessions = %q", list.Text)
	}
	msg = rt.execute([]string{"kill-session", "-a", "-t", "missing"}, "", 80, 24)
	if msg.OK || msg.Text != "can't find session: missing" {
		t.Fatalf("kill-session -a missing = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "keep"}, "keep", 80, 24)
}

func TestNewSessionPrintFlag(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "newsout", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK || msg.Text != "" || msg.Session != "newsout" {
		t.Fatalf("new-session default output = %#v, want empty text and session newsout", msg)
	}
	msg = rt.execute([]string{"new-session", "-d", "-P", "-F", "#{session_name}:#{window_index}.#{pane_index}", "-s", "newsp", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK || msg.Text != "newsp:0.0" || msg.Session != "newsp" {
		t.Fatalf("new-session -P output = %#v, want newsp:0.0", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "newsout"}, "newsout", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "newsp"}, "newsp", 80, 24)
}

func TestRefreshClientRequiresCurrentClient(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"refresh-client"}, "", 80, 24)
	if msg.OK || msg.Text != "no current client" {
		t.Fatalf("refresh-client without client = %#v", msg)
	}
	msg = rt.executeWithClient([]string{"refresh"}, "", 80, 24, 42)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("refresh with client = %#v", msg)
	}
}

func TestBasicModeAndClientEntryCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	for _, args := range [][]string{
		{"clock-mode"},
		{"copy-mode"},
		{"choose-buffer"},
		{"choose-client"},
		{"choose-tree"},
		{"customize-mode"},
		{"findw", "anything"},
	} {
		msg := rt.execute(args, "", 80, 24)
		if !msg.OK || msg.Text != "" {
			t.Fatalf("%v = %#v", args, msg)
		}
	}
	for _, args := range [][]string{
		{"command-prompt"},
		{"confirm-before", "true"},
		{"menu", "item", "i", "true"},
		{"displayp"},
		{"popup"},
		{"suspend-client"},
	} {
		msg := rt.execute(args, "", 80, 24)
		if msg.OK || msg.Text != "no current client" {
			t.Fatalf("%v without client = %#v", args, msg)
		}
		msg = rt.executeWithClient(args, "", 80, 24, 42)
		if !msg.OK || msg.Text != "" {
			t.Fatalf("%v with client = %#v", args, msg)
		}
	}
}

func TestSwitchClientTargetsAndRelativeSessions(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "sw1", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session sw1 failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "sw2", "-n", "second", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session sw2 failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("sw1", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"switch-client", "-t", "sw2"}, "sw1", 80, 24)
	if msg.OK || msg.Text != "no current client" {
		t.Fatalf("switch-client without current client = %#v", msg)
	}
	msg = rt.executeWithClient([]string{"switch-client", "-t", "sw2"}, "sw1", 80, 24, client.ID)
	if !msg.OK || rt.state.ActiveSessionName(client.ID) != "sw2" {
		t.Fatalf("switch-client -t = %#v active=%s", msg, rt.state.ActiveSessionName(client.ID))
	}
	msg = rt.executeWithClient([]string{"switch-client", "-l"}, "sw2", 80, 24, client.ID)
	if !msg.OK || rt.state.ActiveSessionName(client.ID) != "sw1" {
		t.Fatalf("switch-client -l = %#v active=%s", msg, rt.state.ActiveSessionName(client.ID))
	}
	msg = rt.executeWithClient([]string{"switch-client", "-n"}, "sw1", 80, 24, client.ID)
	if !msg.OK || rt.state.ActiveSessionName(client.ID) != "sw2" {
		t.Fatalf("switch-client -n = %#v active=%s", msg, rt.state.ActiveSessionName(client.ID))
	}
	msg = rt.executeWithClient([]string{"switchc", "-p"}, "sw2", 80, 24, client.ID)
	if !msg.OK || rt.state.ActiveSessionName(client.ID) != "sw1" {
		t.Fatalf("switch-client -p = %#v active=%s", msg, rt.state.ActiveSessionName(client.ID))
	}
	msg = rt.execute([]string{"switch-client", "-c", "client-1", "-t", "sw2"}, "sw1", 80, 24)
	if !msg.OK || rt.state.ActiveSessionName(client.ID) != "sw2" {
		t.Fatalf("switch-client -c = %#v active=%s", msg, rt.state.ActiveSessionName(client.ID))
	}
	msg = rt.execute([]string{"switch-client", "-c", "missing", "-t", "sw1"}, "sw2", 80, 24)
	if msg.OK || msg.Text != "can't find client: missing" {
		t.Fatalf("switch-client missing client = %#v", msg)
	}
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "sw1"}, "sw1", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "sw2"}, "sw2", 80, 24)
}

func TestDetachClientTargetsAndSessions(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	if _, _, _, err := rt.state.NewSession("detach1", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := rt.state.NewSession("detach2", "", "second", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	c1, c1Messages := attachTestRuntimeClient(t, rt, "detach1")
	_, c2Messages := attachTestRuntimeClient(t, rt, "detach1")
	_, c3Messages := attachTestRuntimeClient(t, rt, "detach2")

	msg := rt.execute([]string{"detach-client"}, "", 80, 24)
	if msg.OK || msg.Text != "no current client" {
		t.Fatalf("detach-client without target = %#v", msg)
	}
	msg = rt.execute([]string{"detach-client", "-t", "missing"}, "", 80, 24)
	if msg.OK || msg.Text != "can't find client: missing" {
		t.Fatalf("detach-client missing target = %#v", msg)
	}
	msg = rt.execute([]string{"detach-client", "-a", "-t", clientName(c1)}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("detach-client -a -t = %#v", msg)
	}
	waitForProtocolState(t, c2Messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "detached"
	})
	waitForProtocolState(t, c3Messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "detached"
	})
	expectNoProtocolMessage(t, c1Messages)

	rt = &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	if _, _, _, err := rt.state.NewSession("group1", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := rt.state.NewSession("group2", "", "second", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	_, g1aMessages := attachTestRuntimeClient(t, rt, "group1")
	_, g1bMessages := attachTestRuntimeClient(t, rt, "group1")
	_, g2Messages := attachTestRuntimeClient(t, rt, "group2")
	msg = rt.execute([]string{"detach-client", "-s", "group1"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("detach-client -s = %#v", msg)
	}
	waitForProtocolState(t, g1aMessages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "detached"
	})
	waitForProtocolState(t, g1bMessages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "detached"
	})
	expectNoProtocolMessage(t, g2Messages)
}

func TestListClients(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	format := "#{client_name}:#{session_name}:#{client_width}:#{client_height}:#{client_termname}"
	msg := rt.execute([]string{"list-clients", "-F", format}, "", 80, 24)
	if msg.Text != "" {
		t.Fatalf("list-clients empty = %q", msg.Text)
	}
	if msg = rt.execute([]string{"new-session", "-d", "-s", "clients", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("clients", 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	msg = rt.execute([]string{"lsc", "-F", format, "-t", "clients"}, "", 80, 24)
	if msg.Text != "client-1:clients:100:30:screen-256color" {
		t.Fatalf("list-clients format = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-clients", "-F", format, "-t", "missing"}, "", 80, 24)
	if msg.Text != "" {
		t.Fatalf("list-clients target filter = %q", msg.Text)
	}
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "clients"}, "clients", 80, 24)
}

func TestChooseTreeAttachedStatus(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if _, _, _, err := rt.state.NewSession("choosetree", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	if _, _, err := rt.state.NewWindow("choosetree", "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-window failed: %s", err)
	}
	msg := rt.execute([]string{"choose-tree"}, "choosetree", 80, 24)
	if !msg.OK || msg.Text != "" || msg.StatusText != "" {
		t.Fatalf("detached choose-tree = %#v", msg)
	}
	msg = rt.executeWithClient([]string{"choose-tree"}, "choosetree", 80, 24, 1)
	if !msg.OK || msg.Text != "" || msg.StatusText != "choose-tree: choosetree:0:first choosetree:1:second*" {
		t.Fatalf("attached choose-tree = %#v", msg)
	}
	msg = rt.executeWithClient([]string{"choose-tree", "-s"}, "choosetree", 80, 24, 1)
	if !msg.OK || msg.StatusText != "choose-tree: choosetree*" {
		t.Fatalf("attached choose-tree sessions = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "choosetree"}, "choosetree", 80, 24)
}

func TestAttachCanDetachOtherClients(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	if _, _, _, err := rt.state.NewSession("attachd", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	_, oldMessages := attachTestRuntimeClient(t, rt, "attachd")
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	newProtocol := protocol.NewConn(clientConn)
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.handleAttach(protocol.NewConn(serverConn), protocol.Message{
			Type:         protocol.TypeAttach,
			Session:      "attachd",
			Width:        40,
			Height:       6,
			DetachOthers: true,
		})
	}()
	first, err := newProtocol.Read()
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != protocol.TypeResult || !first.OK {
		t.Fatalf("new attach result = %#v", first)
	}
	waitForProtocolState(t, oldMessages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "detached"
	})
	_ = clientConn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("attach handler did not exit")
	}
}

func TestCommandStopsServerWhenLastSessionRemoved(t *testing.T) {
	var once sync.Once
	stopped := make(chan struct{})
	rt := &Runtime{
		state: model.NewServer("/tmp/gotmux-test.sock"),
		stopServer: func() {
			once.Do(func() { close(stopped) })
		},
	}
	msg := rt.execute([]string{"new-session", "-d", "-s", "last", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	done := make(chan struct{})
	go func() {
		rt.handleCommand(protocol.NewConn(serverConn), protocol.Message{
			Type:    protocol.TypeCommand,
			Command: []string{"kill-session", "-t", "last"},
		})
		close(done)
	}()
	result, err := protocol.NewConn(clientConn).Read()
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("kill-session result = %#v", result)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("server did not stop after last session was removed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("command handler did not finish")
	}
}

func TestCommandDetachesClientsForRemovedSession(t *testing.T) {
	rt := &Runtime{
		state:   model.NewServer("/tmp/gotmux-test.sock"),
		clients: make(map[int64]*attachedClient),
	}
	msg := rt.execute([]string{"new-session", "-d", "-s", "removed", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	client, _, err := rt.state.AttachClient("removed", 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	attachedServerConn, attachedClientConn := net.Pipe()
	defer attachedServerConn.Close()
	defer attachedClientConn.Close()
	attachedProtocol := protocol.NewConn(attachedClientConn)
	messages := make(chan protocol.Message, 64)
	go func() {
		for {
			msg, err := attachedProtocol.Read()
			if err != nil {
				close(messages)
				return
			}
			messages <- msg
		}
	}()
	rt.clients[client.ID] = &attachedClient{id: client.ID, conn: protocol.NewConn(attachedServerConn), done: make(chan struct{})}

	commandServerConn, commandClientConn := net.Pipe()
	defer commandServerConn.Close()
	defer commandClientConn.Close()
	go rt.handleCommand(protocol.NewConn(commandServerConn), protocol.Message{
		Type:    protocol.TypeCommand,
		Command: []string{"kill-session", "-t", "removed"},
	})
	result, err := protocol.NewConn(commandClientConn).Read()
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("kill-session result = %#v", result)
	}
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "session closed"
	})
}

func TestKillServerStopsRuntimeAndDetachesClients(t *testing.T) {
	var once sync.Once
	stopped := make(chan struct{})
	rt := &Runtime{
		state:   model.NewServer("/tmp/gotmux-test.sock"),
		clients: make(map[int64]*attachedClient),
		stopServer: func() {
			once.Do(func() { close(stopped) })
		},
	}
	if _, _, _, err := rt.state.NewSession("killserver", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	client, _, err := rt.state.AttachClient("killserver", 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	clientProtocol := protocol.NewConn(clientConn)
	messages := make(chan protocol.Message, 8)
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
	msg := rt.execute([]string{"kill-server"}, "killserver", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("kill-server result = %#v", msg)
	}
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit && next.Text == "server exited"
	})
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("server did not stop after kill-server")
	}
}
