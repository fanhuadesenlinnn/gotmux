package server

import (
	osuser "os/user"
	"strings"
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

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
	msg = rt.execute([]string{"show", "-sqv", "message-limit"}, "", 80, 24)
	if msg.Text != "1000" {
		t.Fatalf("show server message-limit = %q", msg.Text)
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
	msg = rt.execute([]string{"show", "-gqv", "prefix2"}, "", 80, 24)
	if msg.Text != "None" {
		t.Fatalf("default prefix2 = %q", msg.Text)
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
