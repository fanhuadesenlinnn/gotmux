package server

import (
	"bytes"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

func TestListWindowsAndPanesAllScopes(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lsta", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session lsta failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "lsta", "-n", "second", "/bin/sh"}, "lsta", 80, 24); !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"split-window", "-t", "lsta:0", "-h", "/bin/sh"}, "lsta", 80, 24); !msg.OK {
		t.Fatalf("split-window failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lstb", "-n", "only", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session lstb failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-a", "-F", "#{session_name}:#{window_index}:#{window_name}"}, "", 80, 24)
	if windows.Text != "lsta:0:first\nlsta:1:second\nlstb:0:only" {
		t.Fatalf("list-windows -a = %q", windows.Text)
	}
	panes := rt.execute([]string{"list-panes", "-s", "-t", "lsta", "-F", "#{session_name}:#{window_index}:#{pane_index}"}, "", 80, 24)
	if panes.Text != "lsta:0:0\nlsta:0:1\nlsta:1:0" {
		t.Fatalf("list-panes -s = %q", panes.Text)
	}
	panes = rt.execute([]string{"list-panes", "-a", "-F", "#{session_name}:#{window_index}:#{pane_index}"}, "", 80, 24)
	if panes.Text != "lsta:0:0\nlsta:0:1\nlsta:1:0\nlstb:0:0" {
		t.Fatalf("list-panes -a = %q", panes.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "lsta", "-f", "#{window_active}", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "", 80, 24)
	if windows.Text != "1:second:1" {
		t.Fatalf("list-windows -f active = %q", windows.Text)
	}
	panes = rt.execute([]string{"list-panes", "-t", "lsta", "-f", "#{pane_active}", "-F", "#{pane_index}:#{pane_active}"}, "", 80, 24)
	if panes.Text != "0:1" {
		t.Fatalf("list-panes -f active = %q", panes.Text)
	}
	sessions := rt.execute([]string{"list-sessions", "-f", "#{session_attached}", "-F", "#{session_name}:#{session_attached}"}, "", 80, 24)
	if sessions.Text != "" {
		t.Fatalf("list-sessions -f attached = %q", sessions.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "lsta"}, "lsta", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "lstb"}, "lstb", 80, 24)
}

func TestPromptHistoryCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"show-prompt-history"}, "", 80, 24)
	want := "History for command:\n\n\nHistory for search:\n\n\nHistory for target:\n\n\nHistory for window-target:\n\n"
	if !msg.OK || msg.Text != want {
		t.Fatalf("show-prompt-history = %#v", msg)
	}
	msg = rt.execute([]string{"showphist", "-T", "command"}, "", 80, 24)
	if !msg.OK || msg.Text != "History for command:\n\n" {
		t.Fatalf("showphist -T command = %#v", msg)
	}
	msg = rt.execute([]string{"clearphist", "-T", "command"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("clearphist = %#v", msg)
	}
	msg = rt.execute([]string{"show-prompt-history", "-T", "nope"}, "", 80, 24)
	if msg.OK || msg.Text != "invalid type: nope" {
		t.Fatalf("show-prompt-history invalid = %#v", msg)
	}
}

func TestDisplayMessageTargetsPane(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "displayt", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"split-window", "-t", "displayt", "-h", "/bin/sh"}, "displayt", 80, 24); !msg.OK {
		t.Fatalf("split-window failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"display-message", "-p", "hello #{session_name}"}, "displayt", 80, 24)
	if msg.Text != "hello displayt" {
		t.Fatalf("display-message message = %q", msg.Text)
	}
	msg = rt.execute([]string{"display-message", "-p", "-t", "displayt:.0", "-F", "#{pane_index}:#{pane_active}"}, "displayt", 80, 24)
	if msg.Text != "0:0" {
		t.Fatalf("targeted display-message = %q", msg.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "displayt"}, "displayt", 80, 24)
}

func TestSplitWindowHonorsTargetWindow(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "split", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "split", "-n", "second", "/bin/sh"}, "split", 80, 24); !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"split-window", "-t", "split:0", "-h", "/bin/sh"}, "split", 80, 24)
	if !msg.OK {
		t.Fatalf("split-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "split", "-F", "#{window_index}:#{window_active}:#{window_panes}"}, "split", 80, 24)
	if windows.Text != "0:0:2\n1:1:1" {
		t.Fatalf("windows after targeted split = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "split"}, "split", 80, 24)
}

func TestRespawnPaneAndWindow(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "respawn", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"respawn-pane", "-t", "respawn:0.0", "/bin/sh"}, "respawn", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "still active") {
		t.Fatalf("respawn-pane without -k = %#v", msg)
	}
	msg = rt.execute([]string{"respawnp", "-k", "-t", "respawn:0.0", "/bin/sh"}, "respawn", 80, 24)
	if !msg.OK {
		t.Fatalf("respawn-pane -k failed: %s", msg.Text)
	}
	panes := rt.execute([]string{"list-panes", "-t", "respawn", "-F", "#{pane_index}:#{pane_active}"}, "respawn", 80, 24)
	if panes.Text != "0:1" {
		t.Fatalf("panes after respawn-pane = %q", panes.Text)
	}
	if msg = rt.execute([]string{"split-window", "-t", "respawn", "-h", "/bin/sh"}, "respawn", 80, 24); !msg.OK {
		t.Fatalf("split-window failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"respawnw", "-k", "-t", "respawn:0", "/bin/sh"}, "respawn", 80, 24)
	if !msg.OK {
		t.Fatalf("respawn-window -k failed: %s", msg.Text)
	}
	panes = rt.execute([]string{"list-panes", "-t", "respawn", "-F", "#{pane_index}:#{pane_active}"}, "respawn", 80, 24)
	if panes.Text != "0:1" {
		t.Fatalf("panes after respawn-window = %q", panes.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "respawn"}, "respawn", 80, 24)
}

func TestSplitWindowDetachedAndPrint(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "splitd", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"split-window", "-d", "-h", "-t", "splitd", "/bin/sh"}, "splitd", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("split-window -d = %#v, want empty success", msg)
	}
	panes := rt.execute([]string{"list-panes", "-t", "splitd", "-F", "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_active}"}, "splitd", 80, 24)
	if panes.Text != "0:0:40:1\n1:41:39:0" {
		t.Fatalf("panes after split-window -d = %q", panes.Text)
	}
	msg = rt.execute([]string{"split-window", "-P", "-F", "#{pane_index}:#{pane_active}", "-t", "splitd", "/bin/sh"}, "splitd", 80, 24)
	if !msg.OK || msg.Text != "1:1" {
		t.Fatalf("split-window -P output = %#v, want 1:1", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "splitd"}, "splitd", 80, 24)
}

func TestNewPaneCreatesFloatingPaneAndPrints(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "newp", "-n", "first", "-x", "80", "-y", "24", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"new-pane", "-P", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}", "-t", "newp", "/bin/sh"}, "newp", 80, 24)
	if !msg.OK || msg.Text != "1:4:2:40:6:1" {
		t.Fatalf("new-pane -P output = %#v, want floating geometry", msg)
	}
	panes := rt.execute([]string{"list-panes", "-t", "newp", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"}, "newp", 80, 24)
	if panes.Text != "0:0:0:80:24:0\n1:4:2:40:6:1" {
		t.Fatalf("panes after new-pane = %q", panes.Text)
	}
	msg = rt.execute([]string{"newp", "-d", "-x", "20", "-y", "5", "-X", "3", "-Y", "4", "-t", "newp", "/bin/sh"}, "newp", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("newp -d = %#v, want empty success", msg)
	}
	panes = rt.execute([]string{"list-panes", "-t", "newp", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"}, "newp", 80, 24)
	if panes.Text != "0:0:0:80:24:0\n1:4:2:40:6:1\n2:3:4:20:5:0" {
		t.Fatalf("panes after newp -d = %q", panes.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "newp"}, "newp", 80, 24)
}

func TestPaneExitClosesPaneAndSession(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "exitclose", "/bin/sh", "-c", "exit 0"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session exit command failed: %s", msg.Text)
	}
	waitForTestCondition(t, time.Second, func() bool {
		return !sessionExists(rt.state, "exitclose")
	})

	msg = rt.execute([]string{"new-session", "-d", "-s", "exitpane", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session live command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"split-window", "-t", "exitpane", "-h", "/bin/sh", "-c", "exit 0"}, "exitpane", 80, 24)
	if !msg.OK {
		t.Fatalf("split-window exit command failed: %s", msg.Text)
	}
	waitForTestCondition(t, time.Second, func() bool {
		panes := rt.execute([]string{"list-panes", "-t", "exitpane", "-F", "#{pane_index}:#{pane_active}"}, "exitpane", 80, 24)
		return panes.OK && panes.Text == "0:1"
	})
	_ = rt.execute([]string{"kill-session", "-t", "exitpane"}, "exitpane", 80, 24)
}

func TestAttachRedrawsContentStatusAndSplits(t *testing.T) {
	rt := &Runtime{
		state:   model.NewServer("/tmp/gotmux-test.sock"),
		clients: make(map[int64]*attachedClient),
		screens: make(map[int]*terminal.Screen),
	}
	msg := rt.execute([]string{"new-session", "-d", "-s", "attachdraw", "/bin/sh", "-c", "printf 'attached\n'; exec cat"}, "", 20, 6)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	clientProtocol := protocol.NewConn(clientConn)
	messages := make(chan protocol.Message, 64)
	readErrs := make(chan error, 1)
	go func() {
		for {
			msg, err := clientProtocol.Read()
			if err != nil {
				readErrs <- err
				close(messages)
				return
			}
			messages <- msg
		}
	}()
	done := make(chan struct{})
	go func() {
		rt.handleAttach(protocol.NewConn(serverConn), protocol.Message{Type: protocol.TypeAttach, Session: "attachdraw", Width: 20, Height: 6})
		close(done)
	}()
	first := waitForProtocolState(t, messages, time.Second, func(msg protocol.Message) bool {
		return msg.Type == protocol.TypeResult
	})
	if first.Type != protocol.TypeResult || !first.OK {
		t.Fatalf("attach result = %#v", first)
	}
	sawContent := false
	sawStatus := false
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		if next.Type == protocol.TypeOutput && bytes.Contains(next.Data, []byte("attached")) {
			sawContent = true
		}
		if next.Type == protocol.TypeStatus && bytes.Contains(next.Data, []byte("attachdraw")) {
			sawStatus = true
		}
		return sawContent && sawStatus
	})
	if !sawContent || !sawStatus {
		t.Fatalf("attach redraw content=%v status=%v", sawContent, sawStatus)
	}
	if err := clientProtocol.Write(protocol.Message{Type: protocol.TypeCommand, Command: []string{"split-window", "-h", "/bin/sh"}}); err != nil {
		t.Fatal(err)
	}
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeOutput && bytes.Contains(next.Data, []byte("|"))
	})
	if err := clientProtocol.Write(protocol.Message{Type: protocol.TypeCommand, Command: []string{"display-message", "command #{session_name}"}}); err != nil {
		t.Fatal(err)
	}
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeStatus && bytes.Contains(next.Data, []byte("command attachdraw"))
	})
	if err := clientProtocol.Write(protocol.Message{Type: protocol.TypeCommand, Command: []string{"detach-client"}}); err != nil {
		t.Fatal(err)
	}
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeExit
	})
	select {
	case <-done:
	case err := <-readErrs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("attach handler did not exit")
	}
	_ = rt.execute([]string{"kill-session", "-t", "attachdraw"}, "attachdraw", 80, 24)
}

func TestPipePaneWritesPaneOutputAndToggleCloses(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	dir := t.TempDir()
	output := dir + "/pipe.txt"
	paneScript := dir + "/pane.sh"
	if err := os.WriteFile(paneScript, []byte("#!/bin/sh\nstty -echo\nexec cat\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "pipep", "-n", "first", paneScript}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"pipep", "-o", "-t", "pipep:0.0", "cat > " + shellQuote(output)}, "pipep", 80, 24)
	if !msg.OK {
		t.Fatalf("pipe-pane failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"send-keys", "-t", "pipep:0.0", "pipe-alpha", "Enter"}, "pipep", 80, 24)
	if !msg.OK {
		t.Fatalf("send-keys failed: %s", msg.Text)
	}
	if !waitFileContains(t, output, "pipe-alpha") {
		t.Fatalf("pipe output did not contain alpha")
	}
	msg = rt.execute([]string{"pipep", "-o", "-t", "pipep:0.0", "cat > " + shellQuote(output)}, "pipep", 80, 24)
	if !msg.OK {
		t.Fatalf("pipe-pane toggle failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"send-keys", "-t", "pipep:0.0", "pipe-beta", "Enter"}, "pipep", 80, 24)
	if !msg.OK {
		t.Fatalf("send-keys beta failed: %s", msg.Text)
	}
	time.Sleep(150 * time.Millisecond)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "pipe-beta") {
		t.Fatalf("pipe output was still open: %q", string(data))
	}
	_ = rt.execute([]string{"kill-session", "-t", "pipep"}, "pipep", 80, 24)
}

func TestCapturePaneUsesScreenSnapshot(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, pane, err := rt.state.NewSession("cap", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	screen := terminal.NewScreen(8, 3)
	screen.Write([]byte("one\r\ntwo\r\nthree"))
	rt.screens[pane.ID] = screen

	msg := rt.execute([]string{"capture-pane", "-p", "-t", "cap"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("capture-pane failed: %s", msg.Text)
	}
	if msg.Text != "one\ntwo\nthree" {
		t.Fatalf("capture-pane = %q", msg.Text)
	}

	msg = rt.execute([]string{"capture-pane", "-p", "-S", "1", "-E", "1", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "two" {
		t.Fatalf("capture-pane range = %q", msg.Text)
	}

	screen = terminal.NewScreen(8, 2)
	screen.Write([]byte("one  \r\ntwo"))
	rt.screens[pane.ID] = screen
	msg = rt.execute([]string{"capture-pane", "-p", "-N", "-S", "0", "-E", "1", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "one     \ntwo " {
		t.Fatalf("capture-pane -N = %q", msg.Text)
	}
	msg = rt.execute([]string{"capture-pane", "-p", "-N", "-T", "-S", "0", "-E", "1", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "one  \ntwo" {
		t.Fatalf("capture-pane -N -T = %q", msg.Text)
	}

	screen = terminal.NewScreen(5, 3)
	screen.Write([]byte("abcdefgh\r\nxy"))
	rt.screens[pane.ID] = screen
	msg = rt.execute([]string{"capture-pane", "-p", "-S", "0", "-E", "2", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "abcde\nfgh\nxy" {
		t.Fatalf("capture-pane wrapped lines = %q", msg.Text)
	}
	msg = rt.execute([]string{"capture-pane", "-p", "-J", "-S", "0", "-E", "2", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "abcdefgh\nxy" {
		t.Fatalf("capture-pane -J = %q", msg.Text)
	}
	msg = rt.execute([]string{"capture-pane", "-p", "-F", "-S", "0", "-E", "2", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "W abcde\n- fgh\n- xy" {
		t.Fatalf("capture-pane -F = %q", msg.Text)
	}
	msg = rt.execute([]string{"capture-pane", "-p", "-L", "-S", "0", "-E", "2", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "0 abcde\n1 fgh\n2 xy" {
		t.Fatalf("capture-pane -L = %q", msg.Text)
	}
	msg = rt.execute([]string{"capture-pane", "-p", "-L", "-F", "-J", "-S", "0", "-E", "2", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "0 W abcde1 - fgh\n2 - xy" {
		t.Fatalf("capture-pane -L -F -J = %q", msg.Text)
	}

	screen = terminal.NewScreen(8, 1)
	screen.Write([]byte(`a\b`))
	rt.screens[pane.ID] = screen
	msg = rt.execute([]string{"capture-pane", "-p", "-C", "-S", "0", "-E", "0", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != `a\\b` {
		t.Fatalf("capture-pane -C = %q", msg.Text)
	}

	screen = terminal.NewScreen(24, 1)
	screen.Write([]byte("\x1b[31mred\x1b[0m plain \x1b[1;44mbold\x1b[0m"))
	rt.screens[pane.ID] = screen
	msg = rt.execute([]string{"capture-pane", "-e", "-p", "-T", "-S", "0", "-E", "0", "-t", "cap"}, session.Name, 80, 24)
	if msg.Text != "\x1b[31mred\x1b[39m plain \x1b[1m\x1b[44mbold" {
		t.Fatalf("capture-pane -e = %q", msg.Text)
	}

	screen = terminal.NewScreen(8, 2)
	screen.Write([]byte("one  \r\ntwo"))
	rt.screens[pane.ID] = screen
	msg = rt.execute([]string{"capture-pane", "-b", "capbuf", "-S", "0", "-E", "1", "-t", "cap"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("capture-pane to buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, session.Name, 80, 24)
	if msg.Text != `capbuf:8:one\ntwo\n` {
		t.Fatalf("capture buffer list = %q", msg.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "cap"}, "cap", 80, 24)
}

func TestClearHistoryClearsPaneHistory(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, pane, err := rt.state.NewSession("clear", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	screen := terminal.NewScreenWithHistory(8, 2, 10)
	screen.Write([]byte("old\r\nhistory\r\nvisible"))
	rt.screens[pane.ID] = screen

	msg := rt.execute([]string{"clear-history", "-t", "clear"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("clear-history failed: %s", msg.Text)
	}
	if got := screen.HistoryLen(); got != 0 {
		t.Fatalf("history size after clear-history = %d", got)
	}

	screen.Write([]byte("\r\nagain\r\nmore"))
	msg = rt.execute([]string{"clearhist", "-t", "clear"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("clearhist failed: %s", msg.Text)
	}
	if got := screen.HistoryLen(); got != 0 {
		t.Fatalf("history size after clearhist = %d", got)
	}
	_ = rt.execute([]string{"kill-session", "-t", "clear"}, "clear", 80, 24)
}

func TestKillPaneTargetsPaneAndDropsScreen(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, first, err := rt.state.NewSession("kill", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[first.ID] = terminal.NewScreen(8, 1)
	rt.screens[second.ID] = terminal.NewScreen(8, 1)

	msg := rt.execute([]string{"kill-pane", "-t", ".0"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("kill-pane failed: %s", msg.Text)
	}
	panes := rt.state.ActiveWindowPanes(session.Name)
	if len(panes) != 1 || panes[0].ID != second.ID {
		t.Fatalf("panes after kill-pane = %#v, want only pane %d", panes, second.ID)
	}
	if _, ok := rt.screens[first.ID]; ok {
		t.Fatalf("screen for killed pane %d still exists", first.ID)
	}
	if _, ok := rt.screens[second.ID]; !ok {
		t.Fatalf("screen for remaining pane %d was removed", second.ID)
	}
	_ = rt.execute([]string{"kill-session", "-t", "kill"}, "kill", 80, 24)
}

func TestKillPaneAllKeepsTargetAndDropsOtherScreens(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, first, err := rt.state.NewSession("killpa", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	third, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[first.ID] = terminal.NewScreen(8, 1)
	rt.screens[second.ID] = terminal.NewScreen(8, 1)
	rt.screens[third.ID] = terminal.NewScreen(8, 1)

	msg := rt.execute([]string{"kill-pane", "-a", "-t", ".1"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("kill-pane -a failed: %s", msg.Text)
	}
	panes := rt.state.ActiveWindowPanes(session.Name)
	if len(panes) != 1 || panes[0].ID != second.ID {
		t.Fatalf("panes after kill-pane -a = %#v, want only pane %d", panes, second.ID)
	}
	if _, ok := rt.screens[second.ID]; !ok {
		t.Fatalf("screen for kept pane %d was removed", second.ID)
	}
	for _, paneID := range []int{first.ID, third.ID} {
		if _, ok := rt.screens[paneID]; ok {
			t.Fatalf("screen for killed pane %d still exists", paneID)
		}
	}
	_ = rt.execute([]string{"kill-session", "-t", "killpa"}, "killpa", 80, 24)
}

func TestSelectPaneTargetsPane(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("selp", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"select-pane", "-t", ".0"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-pane failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-panes", "-t", "selp", "-F", "#{pane_index}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:1\n1:0" {
		t.Fatalf("panes after select-pane = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "selp"}, "selp", 80, 24)
}

func TestSelectPaneDirectionsAndLastPane(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("selpdir", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"select-pane", "-L"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-pane -L failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-panes", "-t", "selpdir", "-F", "#{pane_index}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:1\n1:0" {
		t.Fatalf("panes after select-pane -L = %q", got.Text)
	}
	if msg := rt.execute([]string{"select-pane", "-l"}, session.Name, 80, 24); !msg.OK {
		t.Fatalf("select-pane -l failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "selpdir", "-F", "#{pane_index}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:0\n1:1" {
		t.Fatalf("panes after select-pane -l = %q", got.Text)
	}
	if msg := rt.execute([]string{"last-pane"}, session.Name, 80, 24); !msg.OK {
		t.Fatalf("last-pane failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "selpdir", "-F", "#{pane_index}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:1\n1:0" {
		t.Fatalf("panes after last-pane = %q", got.Text)
	}
	if msg := rt.execute([]string{"lastp"}, session.Name, 80, 24); !msg.OK {
		t.Fatalf("lastp failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "selpdir", "-F", "#{pane_index}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:0\n1:1" {
		t.Fatalf("panes after lastp = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "selpdir"}, "selpdir", 80, 24)
}

func TestResizePaneZoomTogglesWindowZoom(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("zoom", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 20, 6)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"resize-pane", "-Z", "-t", "zoom:.0"}, session.Name, 20, 6)
	if !msg.OK {
		t.Fatalf("resize-pane -Z failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-panes", "-t", "zoom", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if got.Text != "0:0:0:20:6:1:1\n1:11:0:9:6:0:1" {
		t.Fatalf("panes after zoom = %q", got.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "zoom", "-F", "#{window_index}:#{window_flags}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if windows.Text != "0:*Z:1" {
		t.Fatalf("windows after zoom = %q", windows.Text)
	}
	panes := rt.state.ActiveWindowPanes(session.Name)
	if len(panes) != 1 || panes[0].Index != 0 {
		t.Fatalf("visible panes after zoom = %#v", panes)
	}
	msg = rt.execute([]string{"resize-pane", "-Z", "-t", "zoom:.1"}, session.Name, 20, 6)
	if !msg.OK {
		t.Fatalf("resize-pane -Z other target failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "zoom", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if got.Text != "0:0:0:10:6:1:0\n1:11:0:9:6:0:0" {
		t.Fatalf("panes after other-target unzoom = %q", got.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "zoom", "-F", "#{window_index}:#{window_flags}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if windows.Text != "0:*:0" {
		t.Fatalf("windows after other-target unzoom = %q", windows.Text)
	}
	msg = rt.execute([]string{"resize-pane", "-Z", "-t", "zoom:.1"}, session.Name, 20, 6)
	if !msg.OK {
		t.Fatalf("resize-pane -Z second pane failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "zoom", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if got.Text != "0:0:0:10:6:0:1\n1:0:0:20:6:1:1" {
		t.Fatalf("panes after zoom second pane = %q", got.Text)
	}
	msg = rt.execute([]string{"resize-pane", "-Z", "-t", "zoom:.1"}, session.Name, 20, 6)
	if !msg.OK {
		t.Fatalf("resize-pane -Z unzoom second pane failed: %s", msg.Text)
	}
	got = rt.execute([]string{"list-panes", "-t", "zoom", "-F", "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}:#{window_zoomed_flag}"}, session.Name, 20, 6)
	if got.Text != "0:0:0:10:6:0:0\n1:11:0:9:6:1:0" {
		t.Fatalf("panes after unzoom second pane = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "zoom"}, "zoom", 80, 24)
}

func TestResizePaneTargetsPane(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, first, err := rt.state.NewSession("resize", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[first.ID] = terminal.NewScreen(40, 24)
	rt.screens[second.ID] = terminal.NewScreen(39, 24)

	msg := rt.execute([]string{"resize-pane", "-t", ".0", "-R", "5"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("resize-pane failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-panes", "-t", "resize", "-F", "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_active}"}, session.Name, 80, 24)
	if got.Text != "0:0:45:0\n1:46:34:1" {
		t.Fatalf("panes after resize-pane = %q", got.Text)
	}
	if lines := rt.screens[first.ID].Lines(); len(lines[0]) != 45 {
		t.Fatalf("screen for resized pane width = %d, want 45", len(lines[0]))
	}
	_ = rt.execute([]string{"kill-session", "-t", "resize"}, "resize", 80, 24)
}

func TestSelectLayoutTargetsWindow(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("layoutt", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rt.state.NewWindow(session.Name, "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"select-layout", "-t", "layoutt:0", "even-vertical"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-layout failed: %s", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_active}")
	if windows != "0:0\n1:1" {
		t.Fatalf("windows after targeted layout = %q", windows)
	}
	sessions := snapshotSessions(rt.state)
	firstWindow := sessions[0].Windows[0]
	if firstWindow.Panes[0].Height != 12 || firstWindow.Panes[1].Top != 13 || firstWindow.Panes[1].Height != 11 {
		t.Fatalf("target layout pane geometry = %d,%d,%d",
			firstWindow.Panes[0].Height, firstWindow.Panes[1].Top, firstWindow.Panes[1].Height)
	}
	_ = rt.execute([]string{"kill-session", "-t", "layoutt"}, "layoutt", 80, 24)
}

func TestLayoutCycleCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("layoutcycle", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}

	msg := rt.execute([]string{"select-layout"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-layout no-arg failed: %s", msg.Text)
	}
	assertPanesFormat(t, rt, session.Name, "0:0:0:40:24\n1:41:0:39:24")

	msg = rt.execute([]string{"previous-layout", "-t", session.Name}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("previous-layout failed: %s", msg.Text)
	}
	assertPanesFormat(t, rt, session.Name, "0:0:0:80:11\n1:0:12:80:12")

	msg = rt.execute([]string{"next-layout", "-t", session.Name}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("next-layout failed: %s", msg.Text)
	}
	assertPanesFormat(t, rt, session.Name, "0:0:0:40:24\n1:41:0:39:24")

	msg = rt.execute([]string{"select-layout", "-p"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-layout -p failed: %s", msg.Text)
	}
	assertPanesFormat(t, rt, session.Name, "0:0:0:80:11\n1:0:12:80:12")

	msg = rt.execute([]string{"select-layout", "-n"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("select-layout -n failed: %s", msg.Text)
	}
	assertPanesFormat(t, rt, session.Name, "0:0:0:40:24\n1:41:0:39:24")
	_ = rt.execute([]string{"kill-session", "-t", "layoutcycle"}, "layoutcycle", 80, 24)
}

func TestSwapPaneCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("swapcmd", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}

	msg := rt.execute([]string{"swap-pane", "-U", "-t", session.Name}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("swap-pane -U failed: %s", msg.Text)
	}
	got := listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_active}")
	if got != "0:%0:0:0\n1:%2:41:1\n2:%1:61:0" {
		t.Fatalf("panes after swap-pane -U = %q", got)
	}

	msg = rt.execute([]string{"swap-pane", "-d", "-s", "swapcmd:.0", "-t", "swapcmd:.1"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("swap-pane -d failed: %s", msg.Text)
	}
	got = listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_active}")
	if got != "0:%2:0:0\n1:%0:41:1\n2:%1:61:0" {
		t.Fatalf("panes after swap-pane -d = %q", got)
	}
	_ = rt.execute([]string{"kill-session", "-t", "swapcmd"}, "swapcmd", 80, 24)
}

func TestRotateWindowCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("rotatecmd", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}

	msg := rt.execute([]string{"rotate-window", "-t", session.Name}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("rotate-window failed: %s", msg.Text)
	}
	got := listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_active}")
	if got != "0:%1:0:0\n1:%2:41:0\n2:%0:61:1" {
		t.Fatalf("panes after rotate-window = %q", got)
	}

	msg = rt.execute([]string{"rotate-window", "-D", "-t", session.Name}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("rotate-window -D failed: %s", msg.Text)
	}
	got = listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_active}")
	if got != "0:%0:0:0\n1:%1:41:0\n2:%2:61:1" {
		t.Fatalf("panes after rotate-window -D = %q", got)
	}
	_ = rt.execute([]string{"kill-session", "-t", "rotatecmd"}, "rotatecmd", 80, 24)
}

func TestBreakPaneCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, _, err := rt.state.NewSession("breakcmd", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}

	msg := rt.execute([]string{"break-pane", "-s", "breakcmd:.1", "-n", "broken", "-P", "-F", "#{session_name}:#{window_index}.#{pane_index}:#{pane_id}:#{window_name}"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("break-pane failed: %s", msg.Text)
	}
	if msg.Text != "breakcmd:1.0:%1:broken" {
		t.Fatalf("break-pane output = %q", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}:#{window_panes}")
	if windows != "0:first:0:1\n1:broken:1:1" {
		t.Fatalf("windows after break-pane = %q", windows)
	}
	panes := listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_width}:#{pane_active}")
	if panes != "0:%1:80:1" {
		t.Fatalf("active panes after break-pane = %q", panes)
	}
	_ = rt.execute([]string{"kill-session", "-t", "breakcmd"}, "breakcmd", 80, 24)
}

func TestJoinPaneCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, firstWindow, _, err := rt.state.NewSession("joincmd", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, _, err := rt.state.NewWindow(session.Name, "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	firstWindow.Width = 80
	firstWindow.Height = 24

	msg := rt.execute([]string{"join-pane", "-s", "joincmd:1.0", "-t", "joincmd:0.0", "-h"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("join-pane failed: %s", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}:#{window_panes}")
	if windows != "0:first:1:2" {
		t.Fatalf("windows after join-pane = %q", windows)
	}
	panes := listPanesFormat(rt.state, session.Name, "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_width}:#{pane_active}")
	if panes != "0:%0:0:40:0\n1:%1:41:39:1" {
		t.Fatalf("panes after join-pane = %q", panes)
	}
	targeted := rt.execute([]string{"list-panes", "-t", "joincmd:0", "-F", "#{pane_index}:#{pane_id}"}, session.Name, 80, 24)
	if targeted.Text != "0:%0\n1:%1" {
		t.Fatalf("targeted list-panes = %q", targeted.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "joincmd"}, "joincmd", 80, 24)
}
