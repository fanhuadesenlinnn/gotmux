package server

import (
	"strings"
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
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
