package model

import "testing"

func TestSplitPaneGeometryMatchesTmuxBasics(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, _, err := state.NewSession("layout", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "vertical"); err != nil {
		t.Fatal(err)
	}
	got := state.ActiveWindowPanes(session.Name)
	want := []struct {
		left, top, width, height int
	}{
		{0, 0, 40, 24},
		{41, 0, 39, 12},
		{41, 13, 39, 11},
	}
	if len(got) != len(want) {
		t.Fatalf("panes = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Left != want[i].left || got[i].Top != want[i].top ||
			got[i].Width != want[i].width || got[i].Height != want[i].height {
			t.Fatalf("pane %d geometry = %d,%d %dx%d, want %d,%d %dx%d",
				i, got[i].Left, got[i].Top, got[i].Width, got[i].Height,
				want[i].left, want[i].top, want[i].width, want[i].height)
		}
	}
}

func TestResizeActivePaneGeometry(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, _, err := state.NewSession("layout", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if err := state.ResizeActivePane(session.Name, "L", 5); err != nil {
		t.Fatal(err)
	}
	got := state.ActiveWindowPanes(session.Name)
	if got[0].Width != 35 || got[1].Left != 36 || got[1].Width != 44 {
		t.Fatalf("resize geometry = pane0 %dx%d at %d,%d pane1 %dx%d at %d,%d",
			got[0].Width, got[0].Height, got[0].Left, got[0].Top,
			got[1].Width, got[1].Height, got[1].Left, got[1].Top)
	}
}
