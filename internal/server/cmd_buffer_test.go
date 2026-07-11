package server

import (
	"os"
	"strings"
	"testing"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

func TestChooseBufferAttachedStatus(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	if _, _, _, err := rt.state.NewSession("choosebuffer", "", "first", []string{"/bin/sh"}); err != nil {
		t.Fatalf("new-session failed: %s", err)
	}
	msg := rt.execute([]string{"choose-buffer"}, "choosebuffer", 80, 24)
	if !msg.OK || msg.Text != "" || msg.StatusText != "" {
		t.Fatalf("detached choose-buffer = %#v", msg)
	}
	msg = rt.executeWithClient([]string{"choose-buffer"}, "choosebuffer", 80, 24, 1)
	if !msg.OK || msg.StatusText != "choose-buffer: empty" {
		t.Fatalf("attached empty choose-buffer = %#v", msg)
	}
	rt.state.SetBuffer("one", "alpha", false)
	rt.state.SetBuffer("two", "beta", false)
	msg = rt.executeWithClient([]string{"choose-buffer"}, "choosebuffer", 80, 24, 1)
	if !msg.OK || msg.StatusText != "choose-buffer: two:4:\"beta\" one:5:\"alpha\"" {
		t.Fatalf("attached choose-buffer = %#v", msg)
	}
	_ = rt.execute([]string{"kill-session", "-t", "choosebuffer"}, "choosebuffer", 80, 24)
}

func TestBufferCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	msg := rt.execute([]string{"set-buffer", "-b", "named", "hello world"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "named"}, "", 80, 24)
	if msg.Text != "hello world" {
		t.Fatalf("show-buffer = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if msg.Text != "named:11:hello world" {
		t.Fatalf("list-buffers named = %q", msg.Text)
	}
	msg = rt.execute([]string{"set-buffer", "-b", "named", "-n", "renamed"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("rename buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "renamed"}, "", 80, 24)
	if msg.Text != "hello world" {
		t.Fatalf("show renamed buffer = %q", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "named"}, "", 80, 24)
	if msg.OK || msg.Text != "no buffer named" {
		t.Fatalf("show old buffer name = %#v", msg)
	}
	msg = rt.execute([]string{"set-buffer", "plain"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("set-buffer auto failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if !strings.Contains(msg.Text, "buffer0:5:plain") || !strings.Contains(msg.Text, "renamed:11:hello world") {
		t.Fatalf("list-buffers auto = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-O", "name", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "buffer0\nrenamed" {
		t.Fatalf("list-buffers -O name = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-f", "#{buffer_name}", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "buffer0\nrenamed" {
		t.Fatalf("list-buffers truthy filter = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-f", "0", "-F", "#{buffer_name}"}, "", 80, 24)
	if msg.Text != "" {
		t.Fatalf("list-buffers false filter = %q", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-O", "nope"}, "", 80, 24)
	if msg.OK || msg.Text != "invalid sort order" {
		t.Fatalf("list-buffers invalid order = %#v", msg)
	}
	msg = rt.execute([]string{"delete-buffer", "-b", "renamed"}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("delete-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"show-buffer", "-b", "renamed"}, "", 80, 24)
	if msg.OK || msg.Text != "no buffer renamed" {
		t.Fatalf("show deleted buffer = %#v", msg)
	}
}

func TestLoadAndSaveBufferCommands(t *testing.T) {
	rt := &Runtime{state: model.NewServer("/tmp/gotmux-test.sock")}
	dir := t.TempDir()
	input := dir + "/input.txt"
	output := dir + "/output.txt"
	if err := os.WriteFile(input, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	msg := rt.execute([]string{"load-buffer", input}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("load-buffer failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"list-buffers", "-F", "#{buffer_name}:#{buffer_size}:#{buffer_sample}"}, "", 80, 24)
	if msg.Text != `buffer0:11:alpha\nbeta\n` {
		t.Fatalf("list after load = %q", msg.Text)
	}
	msg = rt.execute([]string{"save-buffer", output}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("save-buffer failed: %s", msg.Text)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alpha\nbeta\n" {
		t.Fatalf("saved data = %q", string(got))
	}
	if err := os.WriteFile(output, []byte("prefix:"), 0o600); err != nil {
		t.Fatal(err)
	}
	if msg := rt.execute([]string{"set-buffer", "-b", "named", "tail"}, "", 80, 24); !msg.OK {
		t.Fatalf("set-buffer named failed: %s", msg.Text)
	}
	msg = rt.execute([]string{"save-buffer", "-a", "-b", "named", output}, "", 80, 24)
	if !msg.OK {
		t.Fatalf("save-buffer append failed: %s", msg.Text)
	}
	got, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "prefix:tail" {
		t.Fatalf("appended data = %q", string(got))
	}
}
