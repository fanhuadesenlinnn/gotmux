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

func TestScreenCaptureLinesPreserveOnlyUsedTrailingSpaces(t *testing.T) {
	screen := NewScreen(8, 3)
	screen.Write([]byte("one  \r\ntwo\r\nthree"))

	assertLines(t, screen.CaptureLines(false), []string{
		"one",
		"two",
		"three",
	})
	assertLines(t, screen.CaptureLines(true), []string{
		"one  ",
		"two",
		"three",
	})
}

func TestScreenCaptureLinesClearsUsedCells(t *testing.T) {
	screen := NewScreen(8, 2)
	screen.Write([]byte("abcdef"))
	screen.Write([]byte("\x1b[1;4H\x1b[K"))

	assertLines(t, screen.Lines(), []string{
		"abc     ",
		"        ",
	})
	assertLines(t, screen.CaptureLines(true), []string{
		"abc",
		"",
	})
}

func TestScreenCaptureRowsWithEmptyCellOptions(t *testing.T) {
	screen := NewScreen(8, 2)
	screen.Write([]byte("one  \r\ntwo"))

	assertCaptureRows(t, screen.CaptureRowsWithOptions(true, false), []CaptureRow{
		{Text: "one     "},
		{Text: "two "},
	})
	assertCaptureRows(t, screen.CaptureRowsWithOptions(false, false), []CaptureRow{
		{Text: "one  "},
		{Text: "two"},
	})
	assertCaptureRows(t, screen.CaptureRowsWithOptions(true, true), []CaptureRow{
		{Text: "one"},
		{Text: "two"},
	})

	wide := NewScreen(20, 1)
	wide.Write([]byte("one  "))
	assertCaptureRows(t, wide.CaptureRowsWithOptions(true, false), []CaptureRow{
		{Text: "one       "},
	})
}

func TestScreenCaptureRowsMarksAutoWrappedLines(t *testing.T) {
	screen := NewScreen(5, 3)
	screen.Write([]byte("abcdefgh\r\nxy"))

	assertCaptureRows(t, screen.CaptureRows(false), []CaptureRow{
		{Text: "abcde", Wrapped: true},
		{Text: "fgh", Wrapped: false},
		{Text: "xy", Wrapped: false},
	})
}

func TestScreenTracksSGRStyles(t *testing.T) {
	screen := NewScreen(24, 2)
	screen.Write([]byte("\x1b[31mR\x1b[38;5;202mP\x1b[48;2;1;2;3mT\x1b[1;2;3;4;5;7;8;9mA\x1b[22;23;24;25;27;28;29mO"))

	rows := screen.StyledRows()
	if len(rows) != 2 || len(rows[0].Cells) != 24 {
		t.Fatalf("styled rows shape = %#v", rows)
	}
	want := []StyledCell{
		{Rune: 'R', Used: true, Width: 1, Style: Style{Fg: Color{Mode: ColorANSI, Value: 1}}},
		{Rune: 'P', Used: true, Width: 1, Style: Style{Fg: Color{Mode: Color256, Value: 202}}},
		{Rune: 'T', Used: true, Width: 1, Style: Style{Fg: Color{Mode: Color256, Value: 202}, Bg: Color{Mode: ColorRGB, R: 1, G: 2, B: 3}}},
		{Rune: 'A', Used: true, Width: 1, Style: Style{Fg: Color{Mode: Color256, Value: 202}, Bg: Color{Mode: ColorRGB, R: 1, G: 2, B: 3}, Attrs: AttrBold | AttrDim | AttrItalic | AttrUnderline | AttrBlink | AttrReverse | AttrHidden | AttrStrikethrough}},
		{Rune: 'O', Used: true, Width: 1, Style: Style{Fg: Color{Mode: Color256, Value: 202}, Bg: Color{Mode: ColorRGB, R: 1, G: 2, B: 3}}},
	}
	for i := range want {
		if rows[0].Cells[i] != want[i] {
			t.Fatalf("cell %d = %#v, want %#v", i, rows[0].Cells[i], want[i])
		}
	}
}

func TestScreenEraseUsesCurrentBackground(t *testing.T) {
	screen := NewScreen(8, 1)
	screen.Write([]byte("\x1b[41mabc\x1b[K"))

	row := screen.StyledRows()[0]
	for i := 3; i < 8; i++ {
		want := StyledCell{Rune: ' ', Width: 1, Style: Style{Bg: Color{Mode: ColorANSI, Value: 1}}}
		if row.Cells[i] != want {
			t.Fatalf("erased cell %d = %#v, want %#v", i, row.Cells[i], want)
		}
	}
}

func TestScreenCaptureRowsWithSGRSequencesMatchesTmux(t *testing.T) {
	screen := NewScreen(32, 3)
	screen.Write([]byte("\x1b[31mred\x1b[0m plain \x1b[1;44mboldblue\x1b[0m\r\n" +
		"\x1b[38;5;202m256\x1b[39m \x1b[48;2;1;2;3mRGBBG\x1b[0m\r\n" +
		"\x1b[1;2;3;4;5;7;8;9mall\x1b[22;23;24;25;27;28;29moff\x1b[0m"))

	rows := screen.CaptureRowsWithSequences(false, true)
	want := []CaptureRow{
		{Text: "\x1b[31mred\x1b[39m plain \x1b[1m\x1b[44mboldblue"},
		{Text: "\x1b[0m\x1b[38;5;202m256\x1b[39m \x1b[48;2;1;2;3mRGBBG"},
		{Text: "\x1b[1;2;3;4;5;7;8;9m\x1b[49mall\x1b[0moff"},
	}
	assertCaptureRows(t, rows, want)
}

func TestScreenStoresHistoryRowsWithinLimit(t *testing.T) {
	screen := NewScreenWithHistory(5, 2, 3)
	screen.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\nfive"))

	if got := screen.HistoryLen(); got != 3 {
		t.Fatalf("history length = %d, want 3", got)
	}
	assertCaptureRows(t, screen.HistoryRows(false, true), []CaptureRow{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
	})
	assertCaptureRows(t, screen.CaptureAllRowsWithOptions(false, true, false), []CaptureRow{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
		{Text: "four"},
		{Text: "five"},
	})
}

func TestScreenAlternateScreenDoesNotAddHistory(t *testing.T) {
	screen := NewScreenWithHistory(5, 2, 10)
	screen.Write([]byte("main\r\nview"))
	screen.Write([]byte("\x1b[?1049halt1\r\nalt2\r\nalt3\x1b[?1049l"))

	if got := screen.HistoryLen(); got != 0 {
		t.Fatalf("alternate screen history length = %d, want 0", got)
	}
}

func TestScreenClearHistory(t *testing.T) {
	screen := NewScreenWithHistory(5, 2, 10)
	screen.Write([]byte("one\r\ntwo\r\nthree"))
	if got := screen.HistoryLen(); got != 1 {
		t.Fatalf("history length before clear = %d, want 1", got)
	}
	screen.ClearHistory()
	if got := screen.HistoryLen(); got != 0 {
		t.Fatalf("history length after clear = %d, want 0", got)
	}
}

func TestScreenWideRuneWrapsBeforeLastColumn(t *testing.T) {
	screen := NewScreen(4, 2)
	screen.Write([]byte("abc中"))

	rows := screen.StyledRows()
	if !rows[0].Wrapped {
		t.Fatal("line before wide rune was not marked wrapped")
	}
	if rows[1].Cells[0].Rune != '中' || rows[1].Cells[0].Width != 2 || rows[1].Cells[1].Width != 0 {
		t.Fatalf("wide rune cells = %#v", rows[1].Cells[:2])
	}
}

func TestScreenOverwritingWideRuneHalfClearsOtherHalf(t *testing.T) {
	screen := NewScreen(6, 1)
	screen.Write([]byte("中文"))
	screen.Write([]byte("\x1b[1;2HX"))

	cells := screen.StyledRows()[0].Cells
	if cells[0].Rune != ' ' || cells[0].Width != 1 || cells[1].Rune != 'X' || cells[1].Width != 1 {
		t.Fatalf("overwritten wide cells = %#v", cells[:2])
	}
	if cells[2].Rune != '文' || cells[2].Width != 2 || cells[3].Width != 0 {
		t.Fatalf("following wide cells = %#v", cells[2:4])
	}
}

func TestScreenInsertDeleteLeaveNoOrphanWideCells(t *testing.T) {
	screen := NewScreen(8, 1)
	screen.Write([]byte("A中文B"))
	screen.Write([]byte("\x1b[1;2H\x1b[P\x1b[2@"))

	cells := screen.StyledRows()[0].Cells
	for i, current := range cells {
		if current.Width == 0 && (i == 0 || cells[i-1].Width != 2) {
			t.Fatalf("orphan wide placeholder at %d: %#v", i, cells)
		}
		if current.Width == 2 && (i+1 >= len(cells) || cells[i+1].Width != 0) {
			t.Fatalf("wide main cell without placeholder at %d: %#v", i, cells)
		}
	}
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

func assertCaptureRows(t *testing.T, got, want []CaptureRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %#v, want %#v (all rows %#v)", i, got[i], want[i], got)
		}
	}
}
