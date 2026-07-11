package server

import (
	"strings"
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

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
