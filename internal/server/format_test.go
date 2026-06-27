package server

import (
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

func TestFormatSessionWindowPaneFields(t *testing.T) {
	session := &model.Session{ID: 2, Name: "work", Active: 0, Attached: 1}
	window := &model.Window{ID: 3, Index: 4, Name: "build", Active: 5}
	pane := &model.Pane{ID: 6, Index: 5, Width: 80, Height: 23, Command: []string{"/bin/sh"}}
	session.Windows = []*model.Window{window}
	window.Panes = []*model.Pane{pane}

	got := formatString("#{session_name}:#{session_id}:#{session_windows}:#{session_attached}:#{window_id}:#{window_index}:#{window_name}:#{window_panes}:#{window_active}:#{pane_id}:#{pane_index}:#{pane_active}:#{pane_current_command}", formatContext{
		session: session,
		window:  window,
		pane:    pane,
	})
	want := "work:$2:1:1:@3:4:build:1:0:%6:5:1:sh"
	if got != want {
		t.Fatalf("formatString() = %q, want %q", got, want)
	}
}

func TestFormatShorthandFields(t *testing.T) {
	session := &model.Session{Name: "work"}
	window := &model.Window{Index: 1, Name: "edit"}
	pane := &model.Pane{Index: 2}

	got := formatString("#S:#I:#W:#P", formatContext{session: session, window: window, pane: pane})
	if got != "work:1:edit:2" {
		t.Fatalf("formatString() = %q", got)
	}
}
