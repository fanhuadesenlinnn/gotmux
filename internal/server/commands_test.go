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

func TestRenameWindowHonorsTargetWindow(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "ren", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "ren", "-n", "second", "/bin/sh"}, "ren", 80, 24); !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"rename-window", "-t", "ren:0", "primary"}, "ren", 80, 24)
	if !msg.OK {
		t.Fatalf("rename-window failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-windows", "-t", "ren", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "ren", 80, 24)
	if got.Text != "0:primary:0\n1:second:1" {
		t.Fatalf("windows after rename = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "ren"}, "ren", 80, 24)
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

func TestBufferCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"set-buffer", "-b", "named", "hello world"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "named"}, "", 80, 24)
	if msg.Text != "hello world" {
		t.Fatalf("show-buffer = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if msg.Text != "named:11:hello world" {
		t.Fatalf("list-buffers named = %q", msg.Text)
	}
	msg = rt.execute([]string{"set-buffer", "plain"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set-buffer auto failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if !strings.Contains(msg.Text, "buffer0:5:plain") || !strings.Contains(msg.Text, "named:11:hello world") {
		t.Fatalf("list-buffers auto = %q", msg.Text)
	}
	msg = rt.execute([]string{"delete-buffer", "-b", "named"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("delete-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "named"}, "", 80, 24)
	if msg.OK || msg.Text != "no buffer named" {
		t.Fatalf("show deleted buffer = %#v", msg)
	}
}

func TestLoadAndSaveBufferCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	dir := t.TempDir()
	input := dir + "/input.txt"
	output := dir + "/output.txt"
	if err := os.WriteFile(input, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"load-buffer", input}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("load-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if msg.Text != `buffer0:11:alpha\nbeta\n` {
		t.Fatalf("list after load = %q", msg.Text)
	}
	msg = rt.execute([]string{"save-buffer", output}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("save-buffer failed: %s", msg.Text)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alpha\nbeta\n" {
		t.Fatalf("saved data = %q", string(got))
	}
	if err := os.WriteFile(output, []byte("prefix:"), 0o600); err != nil {
		t.Fatal(err)
	}
	if msg := rt.execute([]string{"set-buffer", "-b", "named", "tail"}, "", 80, 24); !msg.OK {
		t.Fatalf("set-buffer named failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"save-buffer", "-a", "-b", "named", output}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("save-buffer append failed: %s", msg.Text)
	}
	got, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix:tail" {
		t.Fatalf("appended data = %q", string(got))
	}
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
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	session, _, pane, err := rt.state.NewSession("clear", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	pane.History.Write([]byte("old\nhistory\n"))

	msg := rt.execute([]string{"clear-history", "-t", "clear"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("clear-history failed: %s", msg.Text)
	}
	if got := string(pane.History.Bytes()); got != "" {
		t.Fatalf("history after clear-history = %q", got)
	}

	pane.History.Write([]byte("again\n"))
	msg = rt.execute([]string{"clearhist", "-t", "clear"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("clearhist failed: %s", msg.Text)
	}
	if got := string(pane.History.Bytes()); got != "" {
		t.Fatalf("history after clearhist = %q", got)
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

func TestKillWindowTargetsWindowAndDropsScreens(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, firstPane, err := rt.state.NewSession("killw", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	_, secondPane, err := rt.state.NewWindow(session.Name, "second", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[firstPane.ID] = terminal.NewScreen(8, 1)
	rt.screens[secondPane.ID] = terminal.NewScreen(8, 1)

	msg := rt.execute([]string{"kill-window", "-t", "killw:1"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("kill-window failed: %s", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}")
	if windows != "0:first:1" {
		t.Fatalf("windows after kill-window = %q", windows)
	}
	if _, ok := rt.screens[secondPane.ID]; ok {
		t.Fatalf("screen for killed window pane %d still exists", secondPane.ID)
	}
	if _, ok := rt.screens[firstPane.ID]; !ok {
		t.Fatalf("screen for remaining pane %d was removed", firstPane.ID)
	}
	_ = rt.execute([]string{"kill-session", "-t", "killw"}, "killw", 80, 24)
}
