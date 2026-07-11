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
	"github.com/fanhuadesenlinnn/gotmux/internal/version"
)

func TestVersionCommandUsesSharedVersion(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"version"}, "", 80, 24)
	if !msg.OK || msg.Text != version.String {
		t.Fatalf("version command = %#v, want %q", msg, version.String)
	}
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

func TestListCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	format := "#{command_list_name}:#{command_list_alias}:#{command_list_usage}"
	msg := rt.execute([]string{"list-commands", "-F", format, "new-session"}, "", 80, 24)
	want := "new-session:new:[-AdDEPX] [-c start-directory] [-e environment] [-F format] [-f flags] [-n window-name] [-s session-name] [-t target-session] [-x width] [-y height] [shell-command [argument ...]]"
	if msg.Text != want {
		t.Fatalf("list-commands new-session = %q", msg.Text)
	}
	msg = rt.execute([]string{"lscm", "-F", format, "display"}, "", 80, 24)
	want = "display-message:display:[-aCIlNpv] [-c target-client] [-d delay] [-F format] [-t target-pane] [message]"
	if msg.Text != want {
		t.Fatalf("lscm display = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "-F", format, "showmsgs"}, "", 80, 24)
	want = "show-messages:showmsgs:[-JT] [-t target-client]"
	if msg.Text != want {
		t.Fatalf("list-commands showmsgs = %q", msg.Text)
	}
	for _, tc := range []struct {
		query string
		want  string
	}{
		{"copy-mode", "copy-mode::[-deHMqSu] [-s src-pane] [-t target-pane]"},
		{"clock-mode", "clock-mode::[-t target-pane]"},
		{"choose-tree", "choose-tree::[-GNrswZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
		{"choose-buffer", "choose-buffer::[-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
		{"choose-client", "choose-client::[-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
		{"customize-mode", "customize-mode::[-NZ] [-F format] [-f filter] [-t target-pane]"},
		{"findw", "find-window:findw:[-CiNrTZ] [-t target-pane] match-string"},
		{"confirm", "confirm-before:confirm:[-by] [-c confirm-key] [-p prompt] [-t target-client] command"},
		{"menu", "display-menu:menu:[-MO] [-b border-lines] [-c target-client] [-C starting-choice] [-H selected-style] [-s style] [-S border-style] [-t target-pane] [-T title] [-x position] [-y position] name [key] [command] ..."},
		{"displayp", "display-panes:displayp:[-bN] [-d duration] [-t target-client] [template]"},
		{"popup", "display-popup:popup:[-BCEkN] [-b border-lines] [-c target-client] [-d start-directory] [-e environment] [-h height] [-s style] [-S border-style] [-t target-pane] [-T title] [-w width] [-x position] [-y position] [shell-command [argument ...]]"},
		{"command-prompt", "command-prompt::[-1CbeFiklN] [-I inputs] [-p prompts] [-t target-client] [-T prompt-type] [template]"},
		{"suspendc", "suspend-client:suspendc:[-t target-client]"},
		{"server-access", "server-access::[-adlrw] [-t target-pane] [user]"},
		{"set-hook", "set-hook::[-agpRuw] [-t target-pane] hook [command]"},
		{"show-hooks", "show-hooks::[-gpw] [-t target-pane] [hook]"},
		{"lsb", "list-buffers:lsb:[-F format] [-f filter] [-O order]"},
	} {
		msg = rt.execute([]string{"list-commands", "-F", format, tc.query}, "", 80, 24)
		if msg.Text != tc.want {
			t.Fatalf("list-commands %s = %q", tc.query, msg.Text)
		}
	}
	msg = rt.execute([]string{"list-commands", "-F", format, "lock"}, "", 80, 24)
	want = "lock-server:lock:"
	if msg.Text != want {
		t.Fatalf("list-commands lock = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "-F", format, "refresh"}, "", 80, 24)
	want = "refresh-client:refresh:[-cDlLRSU] [-A pane:state] [-B name:what:format] [-C XxY] [-f flags] [-r pane:report] [-t target-client] [adjustment]"
	if msg.Text != want {
		t.Fatalf("list-commands refresh = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "-F", format, "linkw"}, "", 80, 24)
	want = "link-window:linkw:[-abdk] [-s src-window] [-t dst-window]"
	if msg.Text != want {
		t.Fatalf("list-commands linkw = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "-F", format, "switchc"}, "", 80, 24)
	want = "switch-client:switchc:[-ElnprZ] [-c target-client] [-t target-session] [-T key-table] [-O order]"
	if msg.Text != want {
		t.Fatalf("list-commands switchc = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "new-sess"}, "", 80, 24)
	if msg.Text != "new-session (new) [-AdDEPX] [-c start-directory] [-e environment] [-F format] [-f flags] [-n window-name] [-s session-name] [-t target-session] [-x width] [-y height] [shell-command [argument ...]]" {
		t.Fatalf("list-commands prefix = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-commands", "list"}, "", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "ambiguous command: list") {
		t.Fatalf("list-commands ambiguous = %#v", msg)
	}
	msg = rt.execute([]string{"start-server"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("start-server = %#v", msg)
	}
}

func TestLockCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lockp", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	for _, args := range [][]string{
		{"lock-server"},
		{"lock"},
		{"lock-session", "-t", "lockp"},
		{"locks", "-t", "lockp"},
	} {
		msg := rt.execute(args, "lockp", 80, 24)
		if !msg.OK || msg.Text != "" {
			t.Fatalf("%v = %#v", args, msg)
		}
	}
	msg := rt.execute([]string{"lock-session", "-t", "missing"}, "lockp", 80, 24)
	if msg.OK || msg.Text != "can't find session: missing" {
		t.Fatalf("lock-session missing = %#v", msg)
	}
	msg = rt.execute([]string{"lock-client"}, "lockp", 80, 24)
	if msg.OK || msg.Text != "no current client" {
		t.Fatalf("lock-client = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "lockp"}, "lockp", 80, 24)
}

func TestRunShell(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"run-shell", "printf alpha"}, "", 80, 24)
	if !msg.OK || msg.Text != "alpha" {
		t.Fatalf("run-shell stdout = %#v", msg)
	}
	msg = rt.execute([]string{"run", "-E", "printf err >&2"}, "", 80, 24)
	if !msg.OK || msg.Text != "err" {
		t.Fatalf("run alias stderr = %#v", msg)
	}
	msg = rt.execute([]string{"run-shell", "exit 7"}, "", 80, 24)
	if msg.OK || msg.Code != 7 || msg.Text != "'exit 7' returned 7" {
		t.Fatalf("run-shell exit status = %#v", msg)
	}
	msg = rt.execute([]string{"run-shell", "-b", "printf beta"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("run-shell background = %#v", msg)
	}
	if msg = rt.execute([]string{"new-session", "-d", "-s", "runsc", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"run-shell", "-C", "display-message -p -F '#{session_name}'"}, "runsc", 80, 24)
	if !msg.OK || msg.Text != "runsc" {
		t.Fatalf("run-shell -C = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "runsc"}, "runsc", 80, 24)
}

func TestIfShell(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"if-shell", "true", "display-message -p yes", "display-message -p no"}, "", 80, 24)
	if !msg.OK || msg.Text != "yes" {
		t.Fatalf("if-shell true branch = %#v", msg)
	}
	msg = rt.execute([]string{"if-shell", "false", "display-message -p yes", "display-message -p no"}, "", 80, 24)
	if !msg.OK || msg.Text != "no" {
		t.Fatalf("if-shell false branch = %#v", msg)
	}
	msg = rt.execute([]string{"if", "-F", "1", "display-message -p fmt-yes", "display-message -p fmt-no"}, "", 80, 24)
	if !msg.OK || msg.Text != "fmt-yes" {
		t.Fatalf("if -F true branch = %#v", msg)
	}
	msg = rt.execute([]string{"if-shell", "-F", "0", "display-message -p fmt-yes", "display-message -p fmt-no"}, "", 80, 24)
	if !msg.OK || msg.Text != "fmt-no" {
		t.Fatalf("if-shell -F false branch = %#v", msg)
	}
	msg = rt.execute([]string{"if-shell", "false", "display-message -p yes"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("if-shell false without else = %#v", msg)
	}
}

func TestWaitFor(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"wait-for", "-S", "ready"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("wait-for -S failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"wait", "ready"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("wait alias failed: %s", msg.Text)
	}

	done := make(chan protocol.Message, 1)
	go func() {
		done <- rt.execute([]string{"wait-for", "async"}, "", 80, 24)
	}()
	select {
	case msg := <-done:
		t.Fatalf("wait-for returned before signal: %#v", msg)
	case <-time.After(20 * time.Millisecond):
	}
	if msg = rt.execute([]string{"wait-for", "-S", "async"}, "", 80, 24); !msg.OK {
		t.Fatalf("wait-for signal failed: %s", msg.Text)
	}
	select {
	case msg := <-done:
		if !msg.OK {
			t.Fatalf("wait-for waiter failed: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("wait-for waiter did not wake")
	}

	if msg = rt.execute([]string{"wait-for", "-L", "lock"}, "", 80, 24); !msg.OK {
		t.Fatalf("wait-for lock failed: %s", msg.Text)
	}
	lockDone := make(chan protocol.Message, 1)
	go func() {
		lockDone <- rt.execute([]string{"wait-for", "-L", "lock"}, "", 80, 24)
	}()
	select {
	case msg := <-lockDone:
		t.Fatalf("second lock returned before unlock: %#v", msg)
	case <-time.After(20 * time.Millisecond):
	}
	if msg = rt.execute([]string{"wait-for", "-U", "lock"}, "", 80, 24); !msg.OK {
		t.Fatalf("wait-for unlock failed: %s", msg.Text)
	}
	select {
	case msg := <-lockDone:
		if !msg.OK {
			t.Fatalf("wait-for second lock failed: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("wait-for lock waiter did not wake")
	}
	if msg = rt.execute([]string{"wait-for", "-U", "lock"}, "", 80, 24); !msg.OK {
		t.Fatalf("wait-for final unlock failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"wait-for", "-U", "missing"}, "", 80, 24)
	if msg.OK || msg.Text != "channel missing not locked" {
		t.Fatalf("wait-for missing unlock = %#v", msg)
	}
}

func TestShowMessagesEmptyJobsAndTerminals(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	for _, args := range [][]string{
		{"show-messages", "-J"},
		{"showmsgs", "-T"},
		{"show-messages", "-J", "-T"},
	} {
		msg := rt.execute(args, "", 80, 24)
		if !msg.OK || msg.Text != "" {
			t.Fatalf("%v = %#v", args, msg)
		}
	}
}

func TestShowMessagesCommandLog(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.executeMessage(protocol.Message{Command: []string{"new-session", "-d", "-s", "msglog", "/bin/sh"}}, "")
	if !msg.OK {
		t.Fatalf("new-session = %#v", msg)
	}
	msg = rt.executeMessage(protocol.Message{Command: []string{"display-message", "-p", "hello"}}, "msglog")
	if !msg.OK || msg.Text != "hello" {
		t.Fatalf("display-message = %#v", msg)
	}
	msg = rt.executeMessage(protocol.Message{Command: []string{"show-messages"}}, "msglog")
	if !msg.OK {
		t.Fatalf("show-messages = %#v", msg)
	}
	lines := strings.Split(msg.Text, "\n")
	if len(lines) < 3 {
		t.Fatalf("show-messages lines = %q", msg.Text)
	}
	for _, line := range lines[:3] {
		if len(line) < len("15:04: command: x") || line[2] != ':' || line[5] != ':' {
			t.Fatalf("message line time shape = %q", line)
		}
	}
	for index, want := range []string{
		"command: show-messages",
		"command: display-message -p hello",
		"command: new-session -d -s msglog /bin/sh",
	} {
		if !strings.Contains(lines[index], want) {
			t.Fatalf("message line %d = %q, want %q", index, lines[index], want)
		}
	}

	if msg := rt.execute([]string{"set", "-s", "message-limit", "2"}, "msglog", 80, 24); !msg.OK {
		t.Fatalf("set message-limit = %#v", msg)
	}
	rt.state.AddMessage("manual-one")
	rt.state.AddMessage("manual-two")
	rt.state.AddMessage("manual-three")
	msg = rt.execute([]string{"show-messages"}, "msglog", 80, 24)
	if !msg.OK || !strings.Contains(msg.Text, "manual-three") || !strings.Contains(msg.Text, "manual-two") || strings.Contains(msg.Text, "manual-one") {
		t.Fatalf("message-limit output = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "msglog"}, "msglog", 80, 24)
}

func TestDisplayMessagePrintFlagControlsCommandOutput(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "displayout", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"display-message", "hello #{session_name}"}, "displayout", 80, 24)
	if !msg.OK || msg.Text != "" || msg.StatusText != "hello displayout" {
		t.Fatalf("display-message default output = %#v", msg)
	}
	msg = rt.execute([]string{"display-message", "-p", "hello #{session_name}"}, "displayout", 80, 24)
	if !msg.OK || msg.Text != "hello displayout" || msg.StatusText != "" {
		t.Fatalf("display-message -p output = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "displayout"}, "displayout", 80, 24)
}

func TestCommonTmuxCommandAliases(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "aliases", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"display", "-p", "-F", "#{session_name}"}, "aliases", 80, 24)
	if !msg.OK || msg.Text != "aliases" {
		t.Fatalf("display alias = %#v, want aliases", msg)
	}
	if msg := rt.execute([]string{"rename", "-t", "aliases", "renamed"}, "aliases", 80, 24); !msg.OK {
		t.Fatalf("rename alias failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"renamew", "-t", "renamed:0", "primary"}, "renamed", 80, 24); !msg.OK {
		t.Fatalf("renamew alias failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-windows", "-t", "renamed", "-F", "#{window_index}:#{window_name}"}, "renamed", 80, 24)
	if got.Text != "0:primary" {
		t.Fatalf("windows after aliases = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "renamed"}, "renamed", 80, 24)
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

func TestStatusOffSuppressesStatusLine(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	if _, _, _, err := rt.state.NewSession("statusoff", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	client, _, err := rt.state.AttachClient("statusoff", 40, 6)
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
	if got := rt.clientContentHeight(client.ID); got != 5 {
		t.Fatalf("content height with status on = %d, want 5", got)
	}
	if msg := rt.execute([]string{"set", "-g", "status", "off"}, "statusoff", 80, 24); !msg.OK {
		t.Fatalf("set status off failed: %s", msg.Text)
	}
	if got := rt.clientContentHeight(client.ID); got != 6 {
		t.Fatalf("content height with status off = %d, want 6", got)
	}
	rt.redrawStatus(client.ID)
	select {
	case msg := <-messages:
		t.Fatalf("unexpected status message with status off: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}
	if msg := rt.execute([]string{"set", "-g", "status", "on"}, "statusoff", 80, 24); !msg.OK {
		t.Fatalf("set status on failed: %s", msg.Text)
	}
	rt.redrawStatus(client.ID)
	waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeStatus && bytes.Contains(next.Data, []byte("statusoff"))
	})
	rt.state.DetachClient(client.ID)
	_ = rt.execute([]string{"kill-session", "-t", "statusoff"}, "statusoff", 80, 24)
}

func TestStatusLineUsesConfiguredFormats(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), clients: make(map[int64]*attachedClient)}
	if _, _, _, err := rt.state.NewSession("statusfmt", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	if _, _, err := rt.state.NewWindow("statusfmt", "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-window failed: %s", err)
	}
	if msg := rt.execute([]string{"set", "-g", "status-left", "#[bold]#S "}, "statusfmt", 80, 24); !msg.OK {
		t.Fatalf("set status-left failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"set", "-g", "status-right", "#h #{session_windows}w"}, "statusfmt", 80, 24); !msg.OK {
		t.Fatalf("set status-right failed: %s", msg.Text)
	}
	client, messages := attachTestRuntimeClient(t, rt, "statusfmt")
	rt.redrawStatus(client.ID)
	status := waitForProtocolState(t, messages, time.Second, func(next protocol.Message) bool {
		return next.Type == protocol.TypeStatus
	})
	host, _ := os.Hostname()
	if idx := strings.Index(host, "."); idx > 0 {
		host = host[:idx]
	}
	text := string(status.Data)
	for _, want := range []string{"statusfmt 0:first 1:second*", host + " 2w"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status line missing %q in %q", want, text)
		}
	}
}
