package server

import (
	"bytes"
	"io"
	"net"
	"os"
	osuser "os/user"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
	"github.com/fanhuadesenlinnn/gotmux/internal/version"
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
	msg = rt.execute([]string{"show", "-gqv", "status-left"}, "", 80, 24)
	if msg.Text != model.DefaultStatusLeft {
		t.Fatalf("show status-left = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "status-right"}, "", 80, 24)
	if msg.Text != model.DefaultStatusRight {
		t.Fatalf("show status-right = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-go", "status", "on"}, "", 80, 24)
	if msg.OK || msg.Text != "already set: status" {
		t.Fatalf("set-once global status = %#v", msg)
	}
	msg = rt.execute([]string{"show", "-sqv", "escape-time"}, "", 80, 24)
	if msg.Text != "10" {
		t.Fatalf("show server escape-time = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "escape-time"}, "", 80, 24)
	if msg.Text != "10" {
		t.Fatalf("show global escape-time = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-s", "escape-time", "123"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set server escape-time failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-sqv", "escape-time"}, "", 80, 24)
	if msg.Text != "123" {
		t.Fatalf("updated server escape-time = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-su", "escape-time"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unset server escape-time failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-sqv", "escape-time"}, "", 80, 24)
	if msg.Text != "10" {
		t.Fatalf("unset server escape-time = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-s", "prefix", "C-a"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set server prefix failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-sqv", "prefix"}, "", 80, 24)
	if msg.Text != "C-a" {
		t.Fatalf("server prefix = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-so", "prefix", "C-c"}, "", 80, 24)
	if msg.OK || msg.Text != "already set: prefix" {
		t.Fatalf("set-once server prefix = %#v", msg)
	}
	msg = rt.execute([]string{"show", "-gqv", "prefix"}, "", 80, 24)
	if msg.Text != "C-b" {
		t.Fatalf("global prefix after server set = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-gu", "status"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unset global status failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "status"}, "", 80, 24)
	if msg.Text != "on" {
		t.Fatalf("unset global status = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-g", "default-command", "foo"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set global default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"set", "-ga", "default-command", "bar"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("append global default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "default-command"}, "", 80, 24)
	if msg.Text != "foobar" {
		t.Fatalf("append global default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-gu", "default-command"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unset global default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gqv", "default-command"}, "", 80, 24)
	if msg.Text != "" {
		t.Fatalf("unset global default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"new-session", "-d", "-s", "opts", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session opts failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"new-session", "-d", "-s", "once", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session once failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"set", "-o", "status", "off"}, "once", 80, 24)
	if !msg.OK {
		t.Fatalf("set-once local status failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"set", "-o", "status", "on"}, "once", 80, 24)
	if msg.OK || msg.Text != "already set: status" {
		t.Fatalf("repeat set-once local status = %#v", msg)
	}
	msg = rt.execute([]string{"show", "-v", "status"}, "once", 80, 24)
	if msg.Text != "off" {
		t.Fatalf("set-once local status value = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-g", "default-command", "base"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("set inherited default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-v", "default-command"}, "opts", 80, 24)
	if msg.Text != "" {
		t.Fatalf("local inherited default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-Av", "default-command"}, "opts", 80, 24)
	if msg.Text != "base" {
		t.Fatalf("inherited default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-a", "default-command", "plus"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("append local default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-v", "default-command"}, "opts", 80, 24)
	if msg.Text != "plus" {
		t.Fatalf("append local default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"set", "-u", "default-command"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("unset local default-command failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-v", "default-command"}, "opts", 80, 24)
	if msg.Text != "" {
		t.Fatalf("unset local default-command = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-Av", "default-command"}, "opts", 80, 24)
	if msg.Text != "base" {
		t.Fatalf("inherited after local unset = %q", msg.Text)
	}
	msg = rt.execute([]string{"new-session", "-d", "-s", "opttarget", "-n", "first", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session opttarget failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"new-window", "-t", "opttarget", "-n", "second", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-window opttarget failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"set", "-t", "opttarget", "status", "off"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("set target session option failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show", "-t", "opttarget", "-v", "status"}, "opts", 80, 24)
	if msg.Text != "off" {
		t.Fatalf("target session status = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-t", "opts", "-v", "status"}, "opts", 80, 24)
	if msg.Text != "" {
		t.Fatalf("untouched session status = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-gwqv", "main-pane-width"}, "", 80, 24)
	if msg.Text != "80" {
		t.Fatalf("show main-pane-width = %q", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-gv", "mode-keys"}, "", 80, 24)
	if msg.Text != "emacs" {
		t.Fatalf("showw mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"setw", "-g", "mode-keys", "vi"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("set global window mode-keys failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"setw", "mode-keys", "emacs"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("set local window mode-keys failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-v", "mode-keys"}, "opts", 80, 24)
	if msg.Text != "emacs" {
		t.Fatalf("local window mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"setw", "-u", "mode-keys"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("unset local window mode-keys failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-v", "mode-keys"}, "opts", 80, 24)
	if msg.Text != "" {
		t.Fatalf("unset local window mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-Awv", "mode-keys"}, "opts", 80, 24)
	if msg.Text != "vi" {
		t.Fatalf("inherited window mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"setw", "-t", "opttarget:1", "mode-keys", "vi"}, "opts", 80, 24)
	if !msg.OK {
		t.Fatalf("set target window option failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-t", "opttarget:1", "-v", "mode-keys"}, "opts", 80, 24)
	if msg.Text != "vi" {
		t.Fatalf("target window mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-t", "opttarget:0", "-v", "mode-keys"}, "opts", 80, 24)
	if msg.Text != "" {
		t.Fatalf("untouched window mode-keys = %q", msg.Text)
	}
	msg = rt.execute([]string{"showw", "-t", "opttarget:9", "-v", "mode-keys"}, "opts", 80, 24)
	if msg.OK || msg.Text != "no such window: opttarget:9" {
		t.Fatalf("missing target window = ok %v text %q", msg.OK, msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "C-a", "send-prefix"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("bind failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "-N", "reload config", "C-r", "source-file", "~/.tmux.conf"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("bind note failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-T", "prefix"}, "", 80, 24)
	if !strings.Contains(msg.Text, "C-a send-prefix") {
		t.Fatalf("list-keys missing binding: %q", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-N"}, "", 80, 24)
	if !strings.Contains(msg.Text, "C-b C-r") || !strings.Contains(msg.Text, "reload config") {
		t.Fatalf("list-keys -N missing note: %q", msg.Text)
	}
	msg = rt.execute([]string{"unbind-key", "C-a"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unbind key failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-T", "prefix"}, "", 80, 24)
	if strings.Contains(msg.Text, "C-a send-prefix") {
		t.Fatalf("list-keys still has unbound key: %q", msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "C-a", "send-prefix"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("rebind C-a failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"bind-key", "C-c", "send-prefix"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("bind C-c failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"unbind-key", "-a"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unbind table failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-T", "prefix"}, "", 80, 24)
	if msg.OK || msg.Text != "table prefix doesn't exist" {
		t.Fatalf("prefix table after unbind -a = %#v", msg)
	}
	msg = rt.execute([]string{"bind-key", "-T", "root", "F1", "display-message", "root"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("bind root failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"unbind-key", "-a", "-T", "root"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("unbind root table failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-keys", "-T", "root"}, "", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("root table after unbind -a = %#v", msg)
	}
}

func TestSetAndShowHooks(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "hooks", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session hooks failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"show-hooks", "-g", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window" {
		t.Fatalf("empty global hook = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-H", "-g", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window" {
		t.Fatalf("empty global hook through show-options = %q", msg.Text)
	}
	msg = rt.execute([]string{"show-hooks", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "" {
		t.Fatalf("empty local hook = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-H", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "" {
		t.Fatalf("empty local hook through show-options = %q", msg.Text)
	}
	msg = rt.execute([]string{"set-hook", "-g", "after-new-window", "display-message hi"}, "hooks", 80, 24)
	if !msg.OK {
		t.Fatalf("set-hook -g failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"set-hook", "-ga", "after-new-window", "display-message there"}, "hooks", 80, 24)
	if !msg.OK {
		t.Fatalf("set-hook -ga failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-hooks", "-g", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window[0] display-message hi\nafter-new-window[1] display-message there" {
		t.Fatalf("global hook values = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-H", "-g", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window[0] display-message hi\nafter-new-window[1] display-message there" {
		t.Fatalf("global hook values through show-options = %q", msg.Text)
	}
	msg = rt.execute([]string{"show", "-H", "-g", "-v", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "display-message hi\ndisplay-message there" {
		t.Fatalf("global hook commands through show-options = %q", msg.Text)
	}
	msg = rt.execute([]string{"set-hook", "after-new-window", "display-message local"}, "hooks", 80, 24)
	if !msg.OK {
		t.Fatalf("set-hook local failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-hooks", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window[0] display-message local" {
		t.Fatalf("local hook value = %q", msg.Text)
	}
	msg = rt.execute([]string{"set-hook", "-gu", "after-new-window"}, "hooks", 80, 24)
	if !msg.OK {
		t.Fatalf("set-hook -gu failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-hooks", "-g", "after-new-window"}, "hooks", 80, 24)
	if msg.Text != "after-new-window" {
		t.Fatalf("unset global hook = %q", msg.Text)
	}
	msg = rt.execute([]string{"show-hooks", "-g", "missing-hook"}, "hooks", 80, 24)
	if msg.OK || msg.Text != "invalid option: missing-hook" {
		t.Fatalf("show-hooks invalid = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "hooks"}, "hooks", 80, 24)
}

func TestServerAccessBasicACL(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	current, err := osuser.Current()
	if err != nil {
		t.Fatal(err)
	}
	owner := current.Username
	if idx := strings.LastIndexAny(owner, `\`); idx >= 0 {
		owner = owner[idx+1:]
	}
	msg := rt.execute([]string{"server-access", "-l"}, "", 80, 24)
	if !msg.OK || msg.Text != owner+" (W)" {
		t.Fatalf("server-access -l = %#v", msg)
	}
	msg = rt.execute([]string{"server-access"}, "", 80, 24)
	if msg.OK || msg.Text != "missing user argument" {
		t.Fatalf("server-access missing user = %#v", msg)
	}
	msg = rt.execute([]string{"server-access", owner}, "", 80, 24)
	if msg.OK || msg.Text != owner+" owns the server, can't change access" {
		t.Fatalf("server-access owner = %#v", msg)
	}
	msg = rt.execute([]string{"server-access", "gotmux-no-such-user"}, "", 80, 24)
	if msg.OK || msg.Text != "unknown user: gotmux-no-such-user" {
		t.Fatalf("server-access unknown user = %#v", msg)
	}
	if _, err := osuser.Lookup("nobody"); err != nil {
		return
	}
	if msg = rt.execute([]string{"server-access", "-a", "nobody"}, "", 80, 24); !msg.OK {
		t.Fatalf("server-access -a nobody = %#v", msg)
	}
	msg = rt.execute([]string{"server-access", "-l"}, "", 80, 24)
	if !strings.Contains(msg.Text, "nobody (W)") {
		t.Fatalf("server-access list after add = %q", msg.Text)
	}
	if msg = rt.execute([]string{"server-access", "-r", "nobody"}, "", 80, 24); !msg.OK {
		t.Fatalf("server-access -r nobody = %#v", msg)
	}
	msg = rt.execute([]string{"server-access", "-l"}, "", 80, 24)
	if !strings.Contains(msg.Text, "nobody (R)") {
		t.Fatalf("server-access list after read-only = %q", msg.Text)
	}
	if msg = rt.execute([]string{"server-access", "-d", "nobody"}, "", 80, 24); !msg.OK {
		t.Fatalf("server-access -d nobody = %#v", msg)
	}
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

func TestSelectWindowHonorsTargetSession(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "aaa", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session aaa failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "src", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session src failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "src", "-n", "second", "/bin/sh"}, "src", 80, 24); !msg.OK {
		t.Fatalf("new-window failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"select-window", "-t", "src:0"}, "aaa", 80, 24); !msg.OK {
		t.Fatalf("select-window failed: %s", msg.Text)
	}
	got := rt.execute([]string{"list-windows", "-t", "src", "-F", "#{window_index}:#{window_active}"}, "aaa", 80, 24)
	if got.Text != "0:1\n1:0" {
		t.Fatalf("src windows after select = %q", got.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "aaa"}, "aaa", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "src"}, "src", 80, 24)
}

func TestSelectWindowFlagsAndRelativeTargets(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "selwflags", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "selwflags", "-n", "second", "/bin/sh"}, "selwflags", 80, 24); !msg.OK {
		t.Fatalf("new-window second failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "selwflags", "-n", "third", "/bin/sh"}, "selwflags", 80, 24); !msg.OK {
		t.Fatalf("new-window third failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"select-window", "-p", "-t", "selwflags"}, "", 80, 24); !msg.OK {
		t.Fatalf("select-window -p failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "selwflags", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "", 80, 24)
	if windows.Text != "0:first:0\n1:second:1\n2:third:0" {
		t.Fatalf("windows after select-window -p = %q", windows.Text)
	}
	if msg := rt.execute([]string{"select-window", "-n", "-t", "selwflags"}, "", 80, 24); !msg.OK {
		t.Fatalf("select-window -n failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"previous-window", "-t", "selwflags"}, "", 80, 24); !msg.OK {
		t.Fatalf("previous-window -t failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "selwflags", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "", 80, 24)
	if windows.Text != "0:first:0\n1:second:1\n2:third:0" {
		t.Fatalf("windows after previous-window -t = %q", windows.Text)
	}
	if msg := rt.execute([]string{"next-window", "-t", "selwflags"}, "", 80, 24); !msg.OK {
		t.Fatalf("next-window -t failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "selwflags", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "", 80, 24)
	if windows.Text != "0:first:0\n1:second:0\n2:third:1" {
		t.Fatalf("windows after next-window -t = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "selwflags"}, "selwflags", 80, 24)
}

func TestNewWindowDetachedAndPrint(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "newwd", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"new-window", "-d", "-t", "newwd", "-n", "second", "/bin/sh"}, "newwd", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("new-window -d = %#v, want empty success", msg)
	}
	windows := rt.execute([]string{"list-windows", "-t", "newwd", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "newwd", 80, 24)
	if windows.Text != "0:first:1\n1:second:0" {
		t.Fatalf("windows after new-window -d = %q", windows.Text)
	}
	if msg := rt.execute([]string{"last-window", "-t", "newwd"}, "newwd", 80, 24); msg.OK {
		t.Fatal("last-window unexpectedly succeeded after detached new-window")
	}
	msg = rt.execute([]string{"new-window", "-P", "-F", "#{window_index}:#{window_name}", "-t", "newwd", "-n", "third", "/bin/sh"}, "newwd", 80, 24)
	if !msg.OK || msg.Text != "2:third" {
		t.Fatalf("new-window -P output = %#v, want 2:third", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "newwd"}, "newwd", 80, 24)
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

func TestLastWindowCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lastw", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "lastw", "-n", "second", "/bin/sh"}, "lastw", 80, 24); !msg.OK {
		t.Fatalf("new-window second failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "lastw", "-n", "third", "/bin/sh"}, "lastw", 80, 24); !msg.OK {
		t.Fatalf("new-window third failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"last-window", "-t", "lastw"}, "lastw", 80, 24); !msg.OK {
		t.Fatalf("last-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "lastw", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "lastw", 80, 24)
	if windows.Text != "0:first:0\n1:second:1\n2:third:0" {
		t.Fatalf("windows after last-window = %q", windows.Text)
	}
	if msg := rt.execute([]string{"select-window", "-l", "-t", "lastw"}, "lastw", 80, 24); !msg.OK {
		t.Fatalf("select-window -l failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "lastw", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "lastw", 80, 24)
	if windows.Text != "0:first:0\n1:second:0\n2:third:1" {
		t.Fatalf("windows after select-window -l = %q", windows.Text)
	}
	if msg := rt.execute([]string{"last", "-t", "lastw"}, "lastw", 80, 24); !msg.OK {
		t.Fatalf("last alias failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "lastw", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "lastw", 80, 24)
	if windows.Text != "0:first:0\n1:second:1\n2:third:0" {
		t.Fatalf("windows after last alias = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "lastw"}, "lastw", 80, 24)
}

func TestSwapWindowCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "swapw", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "swapw", "-n", "second", "/bin/sh"}, "swapw", 80, 24); !msg.OK {
		t.Fatalf("new-window second failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "swapw", "-n", "third", "/bin/sh"}, "swapw", 80, 24); !msg.OK {
		t.Fatalf("new-window third failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"swap-window", "-s", "swapw:0", "-t", "swapw:2"}, "swapw", 80, 24)
	if !msg.OK {
		t.Fatalf("swap-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "swapw", "-F", "#{window_index}:#{window_name}:#{window_id}:#{window_active}"}, "swapw", 80, 24)
	if windows.Text != "0:third:@2:0\n1:second:@1:0\n2:first:@0:1" {
		t.Fatalf("windows after swap-window = %q", windows.Text)
	}
	msg = rt.execute([]string{"swapw", "-d", "-s", "swapw:0", "-t", "swapw:2"}, "swapw", 80, 24)
	if !msg.OK {
		t.Fatalf("swapw -d failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "swapw", "-F", "#{window_index}:#{window_name}:#{window_id}:#{window_active}"}, "swapw", 80, 24)
	if windows.Text != "0:first:@0:0\n1:second:@1:0\n2:third:@2:1" {
		t.Fatalf("windows after swapw -d = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "swapw"}, "swapw", 80, 24)
}

func TestMoveWindowCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "movew", "-n", "first", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "movew", "-n", "second", "/bin/sh"}, "movew", 80, 24); !msg.OK {
		t.Fatalf("new-window second failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-window", "-t", "movew", "-n", "third", "/bin/sh"}, "movew", 80, 24); !msg.OK {
		t.Fatalf("new-window third failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"move-window", "-s", "movew:0", "-t", "movew:5"}, "movew", 80, 24)
	if !msg.OK {
		t.Fatalf("move-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "movew", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "movew", 80, 24)
	if windows.Text != "1:second:0\n2:third:0\n5:first:1" {
		t.Fatalf("windows after move-window = %q", windows.Text)
	}
	msg = rt.execute([]string{"movew", "-r", "-t", "movew"}, "movew", 80, 24)
	if !msg.OK {
		t.Fatalf("movew -r failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "movew", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "movew", 80, 24)
	if windows.Text != "0:second:0\n1:third:0\n2:first:1" {
		t.Fatalf("windows after movew -r = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "movew"}, "movew", 80, 24)
}

func TestEnvironmentCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "env", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"new-session", "-d", "-s", "envtarget", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session envtarget failed: %s", msg.Text)
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
	msg = rt.execute([]string{"setenv", "-t", "envtarget", "TARGET", "yes"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("setenv target failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-t", "envtarget", "TARGET"}, "env", 80, 24)
	if msg.Text != "TARGET=yes" {
		t.Fatalf("showenv target = %q", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-t", "env", "TARGET"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("showenv untouched target = %#v", msg)
	}
	msg = rt.execute([]string{"showenv", "-t", "envtarget", "-s", "TARGET"}, "env", 80, 24)
	if msg.Text != `TARGET="yes"; export TARGET;` {
		t.Fatalf("showenv target shell = %q", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-h", "SECRET", "shh"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("set hidden env failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "SECRET"}, "env", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("show hidden env without -h = %#v", msg)
	}
	msg = rt.execute([]string{"showenv", "-h", "SECRET"}, "env", 80, 24)
	if msg.Text != "SECRET=shh" {
		t.Fatalf("show hidden env = %q", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-hs", "SECRET"}, "env", 80, 24)
	if msg.Text != `SECRET="shh"; export SECRET;` {
		t.Fatalf("show hidden env shell = %q", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-g", "-h", "GSECRET", "gshh"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("set global hidden env failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-g", "GSECRET"}, "env", 80, 24)
	if !msg.OK || msg.Text != "" {
		t.Fatalf("show global hidden env without -h = %#v", msg)
	}
	msg = rt.execute([]string{"showenv", "-gh", "GSECRET"}, "env", 80, 24)
	if msg.Text != "GSECRET=gshh" {
		t.Fatalf("show global hidden env = %q", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-g", "REMOVE_ME", "keep"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("set global env for removal failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-r", "REMOVE_ME"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("set remove marker failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "REMOVE_ME"}, "env", 80, 24)
	if msg.Text != "-REMOVE_ME" {
		t.Fatalf("show remove marker = %q", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-s", "REMOVE_ME"}, "env", 80, 24)
	if msg.Text != "unset REMOVE_ME;" {
		t.Fatalf("show remove marker shell = %q", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-g", "-r", "GREMOVE"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("set global remove marker failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-g", "GREMOVE"}, "env", 80, 24)
	if msg.Text != "-GREMOVE" {
		t.Fatalf("show global remove marker = %q", msg.Text)
	}
	msg = rt.execute([]string{"setenv", "-t", "missing", "TARGET", "no"}, "env", 80, 24)
	if msg.OK || msg.Text != "no such session: missing" {
		t.Fatalf("setenv missing target = %#v", msg)
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
	for _, session := range snapshotSessions(rt.state) {
		if session.Name != "env" {
			continue
		}
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				for _, item := range pane.Env {
					if item == "SECRET=shh" || item == "GSECRET=gshh" || item == "REMOVE_ME=keep" {
						t.Fatalf("new pane inherited hidden environment: %s", item)
					}
				}
			}
		}
	}
	msg = rt.execute([]string{"setenv", "-u", "FOO"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("setenv -u failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "FOO"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("showenv after unset = %#v", msg)
	}
	msg = rt.execute([]string{"setenv", "-u", "REMOVE_ME"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("unset remove marker failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "REMOVE_ME"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("show remove marker after unset = %#v", msg)
	}
	msg = rt.execute([]string{"setenv", "-u", "SECRET"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("unset hidden env failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-h", "SECRET"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("show hidden env after unset = %#v", msg)
	}
	msg = rt.execute([]string{"setenv", "-t", "envtarget", "-u", "TARGET"}, "env", 80, 24)
	if !msg.OK {
		t.Fatalf("unset target env failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"showenv", "-t", "envtarget", "TARGET"}, "env", 80, 24)
	if msg.OK || !strings.Contains(msg.Text, "unknown variable") {
		t.Fatalf("showenv target after unset = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "env"}, "env", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "envtarget"}, "envtarget", 80, 24)
}

func TestPaneEnvironmentOverrides(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"new-session", "-d", "-s", "envopts", "-e", "NEWS=one", "/bin/sh"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session -e failed: %s", msg.Text)
	}
	assertPaneHasEnv := func(name, value string) {
		t.Helper()
		expected := name + "=" + value
		for _, session := range snapshotSessions(rt.state) {
			for _, window := range session.Windows {
				for _, pane := range window.Panes {
					for _, item := range pane.Env {
						if item == expected {
							return
						}
					}
				}
			}
		}
		t.Fatalf("missing pane env %s", expected)
	}
	assertPaneHasEnv("NEWS", "one")
	msg = rt.execute([]string{"new-window", "-t", "envopts", "-n", "winenv", "-e", "WINENV=two", "/bin/sh"}, "envopts", 80, 24)
	if !msg.OK {
		t.Fatalf("new-window -e failed: %s", msg.Text)
	}
	assertPaneHasEnv("WINENV", "two")
	msg = rt.execute([]string{"split-window", "-t", "envopts:1", "-h", "-e", "SPLITENV=three", "/bin/sh"}, "envopts", 80, 24)
	if !msg.OK {
		t.Fatalf("split-window -e failed: %s", msg.Text)
	}
	assertPaneHasEnv("SPLITENV", "three")
	_ = rt.execute([]string{"kill-session", "-t", "envopts"}, "envopts", 80, 24)
}

func TestShellCommandOptionsStayInTrailingCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	shellCommand := []string{"/bin/sh", "-c", "sleep 1"}
	msg := rt.execute(append([]string{"new-session", "-d", "-s", "shellopts"}, shellCommand...), "", 80, 24)
	if !msg.OK {
		t.Fatalf("new-session shell command failed: %s", msg.Text)
	}
	assertPaneCommand(t, rt.state, shellCommand)
	msg = rt.execute(append([]string{"new-window", "-t", "shellopts", "-n", "second"}, shellCommand...), "shellopts", 80, 24)
	if !msg.OK {
		t.Fatalf("new-window shell command failed: %s", msg.Text)
	}
	assertPaneCommand(t, rt.state, shellCommand)
	msg = rt.execute(append([]string{"split-window", "-t", "shellopts:1", "-h"}, shellCommand...), "shellopts", 80, 24)
	if !msg.OK {
		t.Fatalf("split-window shell command failed: %s", msg.Text)
	}
	assertPaneCommand(t, rt.state, shellCommand)
	msg = rt.execute(append([]string{"respawn-pane", "-k", "-t", "shellopts:0.0"}, shellCommand...), "shellopts", 80, 24)
	if !msg.OK {
		t.Fatalf("respawn-pane shell command failed: %s", msg.Text)
	}
	assertPaneCommand(t, rt.state, shellCommand)
	msg = rt.execute(append([]string{"respawn-window", "-k", "-t", "shellopts:1"}, shellCommand...), "shellopts", 80, 24)
	if !msg.OK {
		t.Fatalf("respawn-window shell command failed: %s", msg.Text)
	}
	assertPaneCommand(t, rt.state, shellCommand)
	msg = rt.execute([]string{"run-shell", "printf", "%s", "-c"}, "shellopts", 80, 24)
	if !msg.OK || msg.Text != "-c" {
		t.Fatalf("run-shell trailing option output = %#v, want -c", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "shellopts"}, "shellopts", 80, 24)
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
	rt.handleInput(client.ID, []byte{0x02, '%'})
	panes := rt.execute([]string{"list-panes", "-t", "prefixkeys:1", "-F", "#{pane_index}:#{pane_active}"}, "prefixkeys", 80, 24)
	if panes.Text != "0:0\n1:1" {
		t.Fatalf("prefix %% panes = %q", panes.Text)
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
	msg = rt.execute([]string{"set-buffer", "-b", "named", "-n", "renamed"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("rename buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "renamed"}, "", 80, 24)
	if msg.Text != "hello world" {
		t.Fatalf("show renamed buffer = %q", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "named"}, "", 80, 24)
	if msg.OK || msg.Text != "no buffer named" {
		t.Fatalf("show old buffer name = %#v", msg)
	}
	msg = rt.execute([]string{"set-buffer", "plain"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set-buffer auto failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if !strings.Contains(msg.Text, "buffer0:5:plain") || !strings.Contains(msg.Text, "renamed:11:hello world") {
		t.Fatalf("list-buffers auto = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-O", "name", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "buffer0\nrenamed" {
		t.Fatalf("list-buffers -O name = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-f", "#{buffer_name}", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "buffer0\nrenamed" {
		t.Fatalf("list-buffers truthy filter = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-f", "0", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "" {
		t.Fatalf("list-buffers false filter = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-O", "nope"}, "", 80, 24)
	if msg.OK || msg.Text != "invalid sort order" {
		t.Fatalf("list-buffers invalid order = %#v", msg)
	}
	msg = rt.execute([]string{"delete-buffer", "-b", "renamed"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("delete-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "renamed"}, "", 80, 24)
	if msg.OK || msg.Text != "no buffer renamed" {
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

func TestResizeWindowCommand(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, _, err := rt.state.NewSession("resizew", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := rt.state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"resize-window", "-x", "100", "-y", "30", "-t", "resizew"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("resize-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-t", "resizew", "-F", "#{window_width}:#{window_height}"}, session.Name, 80, 24)
	if windows.Text != "100:30" {
		t.Fatalf("window size after resize-window = %q", windows.Text)
	}
	panes := rt.execute([]string{"list-panes", "-t", "resizew", "-F", "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_height}"}, session.Name, 80, 24)
	if panes.Text != "0:0:50:30\n1:51:49:30" {
		t.Fatalf("panes after resize-window = %q", panes.Text)
	}
	msg = rt.execute([]string{"resizew", "-L", "-t", "resizew", "10"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("resizew -L failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "resizew", "-F", "#{window_width}:#{window_height}"}, session.Name, 80, 24)
	if windows.Text != "90:30" {
		t.Fatalf("window size after resizew -L = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "resizew"}, "resizew", 80, 24)
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

func TestKillWindowAllKeepsTargetAndDropsOtherScreens(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, firstPane, err := rt.state.NewSession("killwa", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	_, secondPane, err := rt.state.NewWindow(session.Name, "second", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	_, thirdPane, err := rt.state.NewWindow(session.Name, "third", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[firstPane.ID] = terminal.NewScreen(8, 1)
	rt.screens[secondPane.ID] = terminal.NewScreen(8, 1)
	rt.screens[thirdPane.ID] = terminal.NewScreen(8, 1)

	msg := rt.execute([]string{"kill-window", "-a", "-t", "killwa:1"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("kill-window -a failed: %s", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}")
	if windows != "1:second:1" {
		t.Fatalf("windows after kill-window -a = %q", windows)
	}
	if _, ok := rt.screens[secondPane.ID]; !ok {
		t.Fatalf("screen for kept window pane %d was removed", secondPane.ID)
	}
	for _, paneID := range []int{firstPane.ID, thirdPane.ID} {
		if _, ok := rt.screens[paneID]; ok {
			t.Fatalf("screen for killed window pane %d still exists", paneID)
		}
	}
	_ = rt.execute([]string{"kill-session", "-t", "killwa"}, "killwa", 80, 24)
}

func TestUnlinkWindowRequiresKillFlagForSingleLinkAndDropsScreens(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	session, _, firstPane, err := rt.state.NewSession("unlinkw", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	_, secondPane, err := rt.state.NewWindow(session.Name, "second", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	rt.screens[firstPane.ID] = terminal.NewScreen(8, 1)
	rt.screens[secondPane.ID] = terminal.NewScreen(8, 1)

	rejected := rt.execute([]string{"unlink-window", "-t", "unlinkw:1"}, session.Name, 80, 24)
	if rejected.OK || rejected.Text != "window only linked to one session" {
		t.Fatalf("unlink-window without -k = %#v", rejected)
	}
	before := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}")
	if before != "0:first:0\n1:second:1" {
		t.Fatalf("windows after rejected unlink-window = %q", before)
	}

	msg := rt.execute([]string{"unlinkw", "-k", "-t", "unlinkw:1"}, session.Name, 80, 24)
	if !msg.OK {
		t.Fatalf("unlink-window -k failed: %s", msg.Text)
	}
	windows := listWindowsFormat(rt.state, session.Name, "#{window_index}:#{window_name}:#{window_active}")
	if windows != "0:first:1" {
		t.Fatalf("windows after unlink-window -k = %q", windows)
	}
	if _, ok := rt.screens[secondPane.ID]; ok {
		t.Fatalf("screen for unlinked window pane %d still exists", secondPane.ID)
	}
	if _, ok := rt.screens[firstPane.ID]; !ok {
		t.Fatalf("screen for remaining pane %d was removed", firstPane.ID)
	}
	_ = rt.execute([]string{"kill-session", "-t", "unlinkw"}, "unlinkw", 80, 24)
}

func TestLinkWindowLinksUnlinksAndReplacesTargets(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock"), screens: make(map[int]*terminal.Screen)}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lna", "-n", "one", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session lna failed: %s", msg.Text)
	}
	if msg := rt.execute([]string{"new-session", "-d", "-s", "lnb", "-n", "two", "/bin/sh"}, "", 80, 24); !msg.OK {
		t.Fatalf("new-session lnb failed: %s", msg.Text)
	}
	msg := rt.execute([]string{"link-window", "-s", "lna:0", "-t", "lnb:2"}, "lna", 80, 24)
	if !msg.OK {
		t.Fatalf("link-window failed: %s", msg.Text)
	}
	windows := rt.execute([]string{"list-windows", "-a", "-F", "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"}, "", 80, 24)
	if windows.Text != "lna:0:one:@0:1\nlnb:0:two:@1:0\nlnb:2:one:@0:1" {
		t.Fatalf("windows after link-window = %q", windows.Text)
	}
	msg = rt.execute([]string{"unlink-window", "-t", "lnb:2"}, "lnb", 80, 24)
	if !msg.OK {
		t.Fatalf("unlink linked window failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-a", "-F", "#{session_name}:#{window_index}:#{window_name}:#{window_id}"}, "", 80, 24)
	if windows.Text != "lna:0:one:@0\nlnb:0:two:@1" {
		t.Fatalf("windows after unlink-window = %q", windows.Text)
	}
	msg = rt.execute([]string{"linkw", "-d", "-s", "lna:0", "-t", "lnb:2"}, "lna", 80, 24)
	if !msg.OK {
		t.Fatalf("link-window -d failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "lnb", "-F", "#{window_index}:#{window_name}:#{window_active}"}, "lnb", 80, 24)
	if windows.Text != "0:two:1\n2:one:0" {
		t.Fatalf("windows after link-window -d = %q", windows.Text)
	}
	msg = rt.execute([]string{"link-window", "-k", "-s", "lna:0", "-t", "lnb:0"}, "lna", 80, 24)
	if !msg.OK {
		t.Fatalf("link-window -k failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-a", "-F", "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"}, "", 80, 24)
	if windows.Text != "lna:0:one:@0:1\nlnb:0:one:@0:1\nlnb:2:one:@0:0" {
		t.Fatalf("windows after link-window -k = %q", windows.Text)
	}
	msg = rt.execute([]string{"link-window", "-s", "lna:0", "-t", "lna:2"}, "lna", 80, 24)
	if !msg.OK {
		t.Fatalf("same-session link-window failed: %s", msg.Text)
	}
	windows = rt.execute([]string{"list-windows", "-t", "lna", "-F", "#{window_index}:#{window_name}:#{window_id}:#{window_active}"}, "lna", 80, 24)
	if windows.Text != "0:one:@0:0\n2:one:@0:1" {
		t.Fatalf("same-session linked active window = %q", windows.Text)
	}
	_ = rt.execute([]string{"kill-session", "-t", "lna"}, "lna", 80, 24)
	_ = rt.execute([]string{"kill-session", "-t", "lnb"}, "lnb", 80, 24)
}

func assertPanesFormat(t *testing.T, rt *Runtime, sessionName string, want string) {
	t.Helper()
	got := listPanesFormat(rt.state, sessionName, "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}")
	if got != want {
		t.Fatalf("pane geometry = %q, want %q", got, want)
	}
}
