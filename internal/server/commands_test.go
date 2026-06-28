package server

import (
	"os"
	"strings"
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
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

func TestCommandSequence(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.executeCommands([][]string{
		{"new-session", "-d", "-s", "seq", "-n", "first", "/bin/sh"},
		{"new-window", "-t", "seq", "-n", "second", "/bin/sh"},
		{"list-windows", "-t", "seq", "-F", "#{window_index}:#{window_name}"},
	}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("sequence failed: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "0:first\n1:second") {
		t.Fatalf("sequence output = %q", msg.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "seq"}, "seq", 80, 24)
}

func TestOptionsAndKeyBindings(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"set", "-g", "status", "off"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "status"}, "", 80, 24)
	if msg.Text != "off" {
		t.Fatalf("show status = %q", msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "C-a", "send-prefix"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("bind failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-T", "prefix"}, "", 80, 24)
	if !strings.Contains(msg.Text, "C-a send-prefix") {
		t.Fatalf("list-keys missing binding: %q", msg.Text)
	}
}

func TestSourceFile(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "src", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	file, err := os.CreateTemp("", "gotmux-source-*.conf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString("set -g status off\nnew-window -n sourced /bin/sh\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	msg = rt.execute([]string{"source-file", file.Name()}, "src", 80, 24)
	if !msg.OK {
		t.Fatalf("source-file failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "status"}, "src", 80, 24)
	if msg.Text != "off" {
		t.Fatalf("source-file status = %q", msg.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "src"}, "src", 80, 24)
}

func TestNewWindowHonorsTargetSession(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "aaa", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session aaa failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "src", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session src failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"new-window", "-t", "src", "-n", "targeted", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	src := rt.execute([]string{"list-windows", "-t", "src", "-F", "#{window_index}:#{window_name}"}, "", 80, 24)
	if !strings.Contains(src.Text, "1:targeted") {
		t.Fatalf("target session missing new window: %q", src.Text)
	}
	aaa := rt.execute([]string{"list-windows", "-t", "aaa", "-F", "#{window_index}:#{window_name}"}, "", 80, 24)
	if strings.Contains(aaa.Text, "targeted") {
		t.Fatalf("new window created in wrong session: %q", aaa.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "aaa"}, "aaa", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "src"}, "src", 80, 24)
}

func TestEnvironmentCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "env", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "FOO", "bar"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("setenv failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "FOO"}, "env", 80, 24)
	if msg.Text != "FOO=bar" {
		t.Fatalf("showenv = %q", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-s", "FOO"}, "env", 80, 24)
	if msg.Text != `FOO="bar"; export FOO;` {
		t.Fatalf("showenv -s = %q", msg.Text)
	}
	msg = rt.execute([]string{"new-window", "-t", "env", "-n", "usesenv", "/bin/sh"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	found := false
	for _, session := range snapshotSessions(rt.state) {
		if session.Name != "env" {
			continue
		}
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				for _, item := range pane.Env {
					if item == "FOO=bar" {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("new pane did not inherit FOO=bar")
	}
	msg = rt.execute([]string{"setenv", "-u", "FOO"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("setenv -u failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "FOO"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("showenv after unset = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "env"}, "env", 80, 24)
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
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "root"}, "root", 80, 24)
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
	_ = rt.execute([]string{"kill-session", "-t", "cap"}, "cap", 80, 24)
}
