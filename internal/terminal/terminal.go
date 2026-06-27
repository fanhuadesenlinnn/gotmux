package terminal

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

type State = term.State

func MakeRaw(fd int) (*State, error) {
	return term.MakeRaw(fd)
}

func Restore(fd int, state *State) error {
	if state == nil {
		return nil
	}
	return term.Restore(fd, state)
}

func Size(fd int) (int, int) {
	width, height, err := term.GetSize(fd)
	if err != nil || width <= 0 || height <= 0 {
		return 80, 24
	}
	return width, height
}

func ClearScreen() []byte {
	return []byte("\x1b[?25l\x1b[2J\x1b[H\x1b[?25h")
}

func StatusLine(width, height int, text string) []byte {
	if width <= 0 || height <= 0 {
		return nil
	}
	text = sanitize(text)
	if displayWidth(text) > width {
		text = trimDisplay(text, width)
	}
	padding := width - displayWidth(text)
	if padding > 0 {
		text += strings.Repeat(" ", padding)
	}
	return []byte(fmt.Sprintf("\x1b7\x1b[%d;1H\x1b[7m%s\x1b[0m\x1b8", height, text))
}

func RestoreScreen() []byte {
	return []byte("\x1b[0m\x1b[?25h\r\n")
}

func WriteRestoreScreen() {
	_, _ = os.Stdout.Write(RestoreScreen())
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\x1b' {
			b.WriteRune(' ')
			continue
		}
		if r < 0x20 && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r == '\t' {
			w += 4
			continue
		}
		w++
	}
	return w
}

func trimDisplay(s string, max int) string {
	if max <= 0 {
		return ""
	}
	var b strings.Builder
	w := 0
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 0 {
			break
		}
		if w+1 > max {
			break
		}
		b.WriteRune(r)
		w++
		s = s[size:]
	}
	return b.String()
}
