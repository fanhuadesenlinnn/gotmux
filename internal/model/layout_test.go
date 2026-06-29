package model

import "testing"

type paneGeometry struct {
	left, top, width, height int
}

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

func TestSelectBuiltinLayoutsMatchTmuxGeometry(t *testing.T) {
	tests := []struct {
		layout string
		want   []paneGeometry
	}{
		{
			layout: "main-horizontal",
			want: []paneGeometry{
				{0, 0, 80, 22},
				{0, 23, 20, 1},
				{21, 23, 19, 1},
				{41, 23, 19, 1},
				{61, 23, 19, 1},
			},
		},
		{
			layout: "main-horizontal-mirrored",
			want: []paneGeometry{
				{0, 2, 80, 22},
				{0, 0, 20, 1},
				{21, 0, 19, 1},
				{41, 0, 19, 1},
				{61, 0, 19, 1},
			},
		},
		{
			layout: "main-vertical",
			want: []paneGeometry{
				{0, 0, 78, 24},
				{79, 0, 1, 6},
				{79, 7, 1, 5},
				{79, 13, 1, 5},
				{79, 19, 1, 5},
			},
		},
		{
			layout: "main-vertical-mirrored",
			want: []paneGeometry{
				{2, 0, 78, 24},
				{0, 0, 1, 6},
				{0, 7, 1, 5},
				{0, 13, 1, 5},
				{0, 19, 1, 5},
			},
		},
		{
			layout: "tiled",
			want: []paneGeometry{
				{0, 0, 39, 7},
				{40, 0, 40, 7},
				{0, 8, 39, 7},
				{40, 8, 40, 7},
				{0, 16, 80, 8},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.layout, func(t *testing.T) {
			state := NewServer("/tmp/gotmux-layout-test.sock")
			session, _, _, err := state.NewSession("layout-"+tt.layout, "", "first", []string{"/bin/sh"})
			if err != nil {
				t.Fatal(err)
			}
			state.SetActiveWindowSize(session.Name, 80, 24)
			for i := 1; i < 5; i++ {
				if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
					t.Fatal(err)
				}
			}
			if err := state.SelectLayout(session.Name, tt.layout); err != nil {
				t.Fatal(err)
			}
			assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), tt.want)
		})
	}
}

func TestSelectRelativeLayoutsMatchTmuxOrder(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, _, err := state.NewSession("layout-cycle", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	if _, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal"); err != nil {
		t.Fatal(err)
	}

	if err := state.SelectPreviousLayout(session.Name); err != nil {
		t.Fatal(err)
	}
	assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), []paneGeometry{
		{0, 0, 80, 11},
		{0, 12, 80, 12},
	})

	if err := state.SelectNextLayout(session.Name); err != nil {
		t.Fatal(err)
	}
	assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), []paneGeometry{
		{0, 0, 40, 24},
		{41, 0, 39, 24},
	})

	if err := state.SelectLayout(session.Name, "main-horizontal"); err != nil {
		t.Fatal(err)
	}
	if err := state.SelectNextLayout(session.Name); err != nil {
		t.Fatal(err)
	}
	assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), []paneGeometry{
		{0, 2, 80, 22},
		{0, 0, 80, 1},
	})
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

func TestSwapPanesByIDMovesPaneObjectsAndActivePane(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, window, first, err := state.NewSession("swap", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	second, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	third, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}

	if err := state.SwapPanesByID(first.ID, third.ID, false); err != nil {
		t.Fatal(err)
	}
	assertPaneIDs(t, window.Panes, []int{third.ID, second.ID, first.ID})
	if window.Active != 0 {
		t.Fatalf("active pane after swap = %d, want 0", window.Active)
	}
	assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), []paneGeometry{
		{0, 0, 40, 24},
		{41, 0, 19, 24},
		{61, 0, 19, 24},
	})

	if err := state.SwapPanesByID(third.ID, second.ID, true); err != nil {
		t.Fatal(err)
	}
	assertPaneIDs(t, window.Panes, []int{second.ID, third.ID, first.ID})
	if window.Active != 0 {
		t.Fatalf("active pane after detached swap = %d, want 0", window.Active)
	}
}

func TestRotateWindowMovesPaneObjectsAndActivePane(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, window, first, err := state.NewSession("rotate", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	second, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	third, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}

	if err := state.RotateWindow(session.Name, false); err != nil {
		t.Fatal(err)
	}
	assertPaneIDs(t, window.Panes, []int{second.ID, third.ID, first.ID})
	if window.Active != 2 {
		t.Fatalf("active pane after rotate = %d, want 2", window.Active)
	}
	assertPaneGeometries(t, state.ActiveWindowPanes(session.Name), []paneGeometry{
		{0, 0, 40, 24},
		{41, 0, 19, 24},
		{61, 0, 19, 24},
	})

	if err := state.RotateWindow(session.Name, true); err != nil {
		t.Fatal(err)
	}
	assertPaneIDs(t, window.Panes, []int{first.ID, second.ID, third.ID})
	if window.Active != 2 {
		t.Fatalf("active pane after reverse rotate = %d, want 2", window.Active)
	}
}

func TestBreakPaneByIDMovesPaneToNewWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, firstWindow, first, err := state.NewSession("break", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	second, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}

	_, newWindow, moved, err := state.BreakPaneByID(second.ID, "broken", false)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID != second.ID {
		t.Fatalf("moved pane id = %d, want %d", moved.ID, second.ID)
	}
	if len(session.Windows) != 2 || session.Active != 1 {
		t.Fatalf("windows after break = %d active %d, want 2 active 1", len(session.Windows), session.Active)
	}
	if newWindow.Name != "broken" || len(newWindow.Panes) != 1 || newWindow.Panes[0].ID != second.ID {
		t.Fatalf("new window = %#v", newWindow)
	}
	if len(firstWindow.Panes) != 1 || firstWindow.Panes[0].ID != first.ID {
		t.Fatalf("source panes after break = %#v", firstWindow.Panes)
	}
	if firstWindow.Panes[0].Width != 80 || firstWindow.Panes[0].Height != 24 {
		t.Fatalf("source pane geometry = %dx%d", firstWindow.Panes[0].Width, firstWindow.Panes[0].Height)
	}
	if newWindow.Panes[0].Width != 80 || newWindow.Panes[0].Height != 24 {
		t.Fatalf("new pane geometry = %dx%d", newWindow.Panes[0].Width, newWindow.Panes[0].Height)
	}
}

func TestBreakPaneByIDDetachedKeepsActiveWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, _, _, err := state.NewSession("breakd", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	second, err := state.SplitPaneWithLayout(session.Name, "", []string{"/bin/sh"}, "horizontal")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := state.BreakPaneByID(second.ID, "", true); err != nil {
		t.Fatal(err)
	}
	if session.Active != 0 {
		t.Fatalf("active window after detached break = %d, want 0", session.Active)
	}
}

func TestJoinPaneByIDMovesPaneIntoTargetWindow(t *testing.T) {
	state := NewServer("/tmp/gotmux-layout-test.sock")
	session, firstWindow, firstPane, err := state.NewSession("join", "", "first", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	state.SetActiveWindowSize(session.Name, 80, 24)
	_, secondPane, err := state.NewWindow(session.Name, "second", "", []string{"/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	firstWindow.Width = 80
	firstWindow.Height = 24

	if _, joinedWindow, moved, err := state.JoinPaneByID(secondPane.ID, firstPane.ID, "horizontal", false); err != nil {
		t.Fatal(err)
	} else if joinedWindow.ID != firstWindow.ID || moved.ID != secondPane.ID {
		t.Fatalf("joined window/pane = %d/%d, want %d/%d", joinedWindow.ID, moved.ID, firstWindow.ID, secondPane.ID)
	}
	if len(session.Windows) != 1 || session.Active != 0 {
		t.Fatalf("windows after join = %d active %d, want 1 active 0", len(session.Windows), session.Active)
	}
	assertPaneIDs(t, firstWindow.Panes, []int{firstPane.ID, secondPane.ID})
	if firstWindow.Active != 1 {
		t.Fatalf("active pane after join = %d, want 1", firstWindow.Active)
	}
	assertPaneGeometries(t, firstWindow.Panes, []paneGeometry{
		{0, 0, 40, 24},
		{41, 0, 39, 24},
	})
}

func assertPaneGeometries(t *testing.T, got []*Pane, want []paneGeometry) {
	t.Helper()
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

func assertPaneIDs(t *testing.T, got []*Pane, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("panes = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] || got[i].Index != i {
			t.Fatalf("pane %d = id %d index %d, want id %d index %d", i, got[i].ID, got[i].Index, want[i], i)
		}
	}
}
