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

func TestResizePaneByIDTargetsNonActivePane(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, first, err := state.NewSession("layout", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if err := state.ResizePaneByID(first.ID, "R", 5); err != nil {
		t.Fatal(err)
	}
	got := state.ActiveWindowPanes(session.Name)
	if got[0].Width != 45 || got[1].Left != 46 || got[1].Width != 34 {
		t.Fatalf("targeted resize geometry = pane0 %dx%d at %d,%d pane1 %dx%d at %d,%d",
			got[0].Width, got[0].Height, got[0].Left, got[0].Top,
			got[1].Width, got[1].Height, got[1].Left, got[1].Top)
	}
}

func TestKillPaneByIDRemovesTargetPaneAndLayoutLeaf(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, first, err := state.NewSession("layout", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	second, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	if err := state.KillPaneByID(first.ID); err != nil {
		t.Fatal(err)
	}
	got := state.ActiveWindowPanes(session.Name)
	if len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("panes after kill = %#v, want only pane %d", got, second.ID)
	}
	if got[0].Left != 0 || got[0].Top != 0 || got[0].Width != 80 || got[0].Height != 24 {
		t.Fatalf("remaining pane geometry = %d,%d %dx%d",
			got[0].Left, got[0].Top, got[0].Width, got[0].Height)
	}
	window := session.ActiveWindow()
	if window == nil {
		t.Fatal("session has no active window after kill")
	}
	if window.Layout == nil || !window.Layout.isLeaf() || window.Layout.PaneID != second.ID {
		t.Fatalf("layout after kill = %#v, want leaf pane %d", window.Layout, second.ID)
	}
}

func TestKillWindowRemovesTargetWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, firstWindow, _, err := state.NewSession("windows", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	secondWindow, _, err := state.NewWindow(session.Name, "second", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.KillWindow(session.Name, secondWindow.Index); err != nil {
		t.Fatal(err)
	}
	if len(session.Windows) != 1 || session.Windows[0].ID != firstWindow.ID {
		t.Fatalf("windows after kill = %#v, want only window %d", session.Windows, firstWindow.ID)
	}
	if session.Active != 0 {
		t.Fatalf("active window after kill = %d, want 0", session.Active)
	}
}

func TestSelectEvenLayoutByIndexDoesNotChangeActiveWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, _, err := state.NewSession("layouts", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := state.NewWindow(session.Name, "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	if session.Active != 1 {
		t.Fatalf("active window before layout = %d, want 1", session.Active)
	}
	if err := state.SelectEvenLayoutByIndex(session.Name, 0, "even-vertical"); err != nil {
		t.Fatal(err)
	}
	if session.Active != 1 {
		t.Fatalf("active window after layout = %d, want 1", session.Active)
	}
	first := session.Windows[0].Panes
	if first[0].Height != 12 || first[1].Top != 13 || first[1].Height != 11 {
		t.Fatalf("target window vertical geometry = pane0 %dx%d at %d,%d pane1 %dx%d at %d,%d",
			first[0].Width, first[0].Height, first[0].Left, first[0].Top,
			first[1].Width, first[1].Height, first[1].Left, first[1].Top)
	}
}

func TestSplitPaneWithLayoutByIndexDoesNotChangeActiveWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, firstWindow, _, err := state.NewSession("splits", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := state.NewWindow(session.Name, "second", "", []string{"/bin/sh"}); err != nil {
		t.Fatal(err)
	}
	if session.Active != 1 {
		t.Fatalf("active window before split = %d, want 1", session.Active)
	}
	pane, err := state.SplitPaneWithLayoutByIndex(session.Name, firstWindow.Index, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	if session.Active != 1 {
		t.Fatalf("active window after split = %d, want 1", session.Active)
	}
	if len(firstWindow.Panes) != 2 || firstWindow.Active != pane.Index {
		t.Fatalf("target window panes = %d active %d, want new pane active", len(firstWindow.Panes), firstWindow.Active)
	}
}
