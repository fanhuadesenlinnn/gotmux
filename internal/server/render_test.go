package server

import (
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

func TestRenderPaneCanvasDrawsSplitBorderAndContent(t *testing.T) {
	left := &model.Pane{Left: 0, Top: 0, Width: 4, Height: 3, History: model.NewRing(1024)}
	right := &model.Pane{Left: 5, Top: 0, Width: 4, Height: 3, History: model.NewRing(1024)}
	left.History.Write([]byte("left\n"))
	right.History.Write([]byte("right\n"))

	got := renderPaneCanvas(9, 3, []*model.Pane{left, right})
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
