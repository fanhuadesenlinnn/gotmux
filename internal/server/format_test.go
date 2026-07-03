package server

import (
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

func TestFormatSessionWindowPaneFields(t *testing.T) {
	session := &model.Session{ID: 2, Name: "work", Active: 0, Attached: 1}
	window := &model.Window{ID: 3, Index: 4, Name: "build", Active: 0}
	pane := &model.Pane{ID: 6, Index: 5, Width: 80, Height: 23, Command: []string{"/bin/sh"}}
	session.Windows = []*model.Window{window}
	window.Panes = []*model.Pane{pane}

	got := formatString("#{session_name}:#{session_id}:#{session_windows}:#{session_attached}:#{window_id}:#{window_index}:#{window_name}:#{window_panes}:#{window_active}:#{pane_id}:#{pane_index}:#{pane_active}:#{pane_current_command}", formatContext{
		session: session,
		window:  window,
		pane:    pane,
	})
	want := "work:$2:1:1:@3:4:build:1:1:%6:5:1:sh"
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

func TestFormatConditionals(t *testing.T) {
	session := &model.Session{Name: "work"}
	window := &model.Window{Active: 0}
	active := &model.Pane{ID: 1, Index: 1}
	inactive := &model.Pane{ID: 2, Index: 2}
	window.Panes = []*model.Pane{active, inactive}
	ctx := formatContext{session: session, window: window, pane: active}

	tests := map[string]string{
		"#{?pane_active,Y,N}":                   "Y",
		"#{?#{pane_active},#S,N}":               "work",
		"#{?pane_active,#{session_name},no}":    "work",
		"#{?1,Y,N}":                             "N",
		"#{?missing,Y,N}":                       "N",
		"#{?pane_active,#{pane_index},missing}": "1",
	}
	for template, want := range tests {
		if got := formatString(template, ctx); got != want {
			t.Fatalf("formatString(%q) = %q, want %q", template, got, want)
		}
	}

	inactiveCtx := formatContext{session: session, window: window, pane: inactive}
	if got := formatString("#{?pane_active,Y,N}", inactiveCtx); got != "N" {
		t.Fatalf("inactive pane conditional = %q", got)
	}
}

func TestFormatTrimModifier(t *testing.T) {
	session := &model.Session{Name: "work"}
	pane := &model.Pane{Command: []string{"/usr/bin/python3"}}
	ctx := formatContext{session: session, pane: pane}

	tests := map[string]string{
		"#{=2:session_name}":         "wo",
		"#{=4:#{session_name}}":      "work",
		"#{=6:pane_current_command}": "python",
		"#{=4:unknown_format}":       "",
		"#{=2;...:#{session_name}}":  "",
	}
	for template, want := range tests {
		if got := formatString(template, ctx); got != want {
			t.Fatalf("formatString(%q) = %q, want %q", template, got, want)
		}
	}
}
