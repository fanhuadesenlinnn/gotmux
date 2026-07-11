package server

import (
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

func TestRenderPaneCanvasDrawsSplitBorderAndContent(t *testing.T) {
	left := &model.Pane{ID: 1, Left: 0, Top: 0, Width: 4, Height: 3}
	right := &model.Pane{ID: 2, Left: 5, Top: 0, Width: 4, Height: 3}

	got := renderPaneCanvas(9, 3, []*model.Pane{left, right}, map[int][]string{
		1: {"    ", "    ", "left"},
		2: {"    ", "    ", "right"},
	})
	want := []string{
		"    |    ",
		"    |    ",
		"left|righ",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderPaneCanvasUsesScreenLinesFromTop(t *testing.T) {
	pane := &model.Pane{ID: 7, Left: 0, Top: 0, Width: 6, Height: 2}

	got := renderPaneCanvas(6, 2, []*model.Pane{pane}, map[int][]string{
		pane.ID: {"top   ", "next  "},
	})
	want := []string{"top   ", "next  "}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVisibleTextLinesStripsANSIEscapes(t *testing.T) {
	got := visibleTextLines([]byte("\x1b[31mred\x1b[0m\r\nplain\n"), 10)
	want := []string{"red", "plain"}
	if len(got) != len(want) {
		t.Fatalf("lines = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderPanesIncludesScreenStyles(t *testing.T) {
	pane := &model.Pane{ID: 7, Left: 0, Top: 0, Width: 2, Height: 1}
	rows := map[int][]terminal.StyledRow{
		pane.ID: {
			{Cells: []terminal.StyledCell{
				{Rune: 'A', Used: true, Style: terminal.Style{Fg: terminal.Color{Mode: terminal.ColorANSI, Value: 1}}},
				{Rune: 'B', Used: true},
			}},
		},
	}

	got := string(renderPanes(2, 1, []*model.Pane{pane}, rows))
	want := "\x1b[?25l\x1b[2J\x1b[1;1H\x1b[0m\x1b[31mA\x1b[39mB\x1b[?25h"
	if got != want {
		t.Fatalf("rendered panes = %q, want %q", got, want)
	}
}
