package main

import "testing"

func TestAttachAndNewSessionDetachFlags(t *testing.T) {
	if !hasShortFlag([]string{"-d", "-t", "work"}, 'd') {
		t.Fatal("attach -d flag not detected")
	}
	if hasShortFlag([]string{"-D", "-s", "work"}, 'd') {
		t.Fatal("new-session -D should not be treated as detached")
	}
	if !hasShortFlag([]string{"-AD", "-s", "work"}, 'D') {
		t.Fatal("new-session -D flag not detected")
	}
	if detached([]string{"new-session", "-D", "-s", "work"}) {
		t.Fatal("new-session -D should still attach")
	}
}
