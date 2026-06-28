package terminal

import "testing"

func TestScreenTracksCarriageReturnAndClearLine(t *testing.T) {
	screen := NewScreen(6, 3)
	screen.Write([]byte("hello"))
	screen.Write([]byte("\rHEY"))
	screen.Write([]byte("\x1b[2;3HZ\x1b[K"))

	want := []string{
		"HEYlo ",
		"  Z   ",
		"      ",
	}
	assertLines(t, screen.Lines(), want)
}

func TestScreenHandlesSplitCSIAndClearScreen(t *testing.T) {
	screen := NewScreen(6, 2)
	screen.Write([]byte("abcdef"))
	screen.Write([]byte("\x1b["))
	screen.Write([]byte("2J\x1b[Hdone"))

	want := []string{
		"done  ",
		"      ",
	}
	assertLines(t, screen.Lines(), want)
}

func TestScreenScrollsOnLineFeedAtBottom(t *testing.T) {
	screen := NewScreen(5, 2)
	screen.Write([]byte("one\r\ntwo\r\nthree"))

	want := []string{
		"two  ",
		"three",
	}
	assertLines(t, screen.Lines(), want)
}

func TestScreenInsertAndDeleteCharacters(t *testing.T) {
	screen := NewScreen(8, 1)
	screen.Write([]byte("abcd"))
	screen.Write([]byte("\x1b[1;3H\x1b[@Z"))
	screen.Write([]byte("\x1b[1;2H\x1b[P"))

	want := []string{"aZcd    "}
	assertLines(t, screen.Lines(), want)
}

func TestScreenAlternateScreenRestoresMainBuffer(t *testing.T) {
	screen := NewScreen(8, 3)
	screen.Write([]byte("shell\r\nprompt"))
	screen.Write([]byte("\x1b[?1049hALT"))

	assertLines(t, screen.Lines(), []string{
		"ALT     ",
		"        ",
		"        ",
	})

	screen.Write([]byte("\x1b[?1049l!"))

	want := []string{
		"shell   ",
		"prompt! ",
		"        ",
	}
	assertLines(t, screen.Lines(), want)
}

func TestScreenAlternateScreenRestoresMainAfterResize(t *testing.T) {
	screen := NewScreen(6, 2)
	screen.Write([]byte("main"))
	screen.Write([]byte("\x1b[?1049halt"))
	screen.Resize(8, 3)
	screen.Write([]byte("\x1b[?1049l"))

	want := []string{
		"main    ",
		"        ",
		"        ",
	}
	assertLines(t, screen.Lines(), want)
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q (all lines %#v)", i, got[i], want[i], got)
		}
	}
}
