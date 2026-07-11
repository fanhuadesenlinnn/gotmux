package terminal

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	screenNormal = iota
	screenEscape
	screenCSI
	screenIgnoreOne
)

// Screen keeps the visible grid for a pane. It intentionally models the common
// VT sequences used by shells and full-screen programs without inventing tmux
// behavior above the terminal layer.
type Screen struct {
	mu     sync.RWMutex
	width  int
	height int
	cells  [][]cell
	wraps  []bool

	altScreen   bool
	mainCells   [][]cell
	mainWraps   []bool
	mainCursorX int
	mainCursorY int
	mainSavedX  int
	mainSavedY  int

	cursorX  int
	cursorY  int
	savedX   int
	savedY   int
	curStyle Style

	state   int
	csi     []byte
	utf8Buf []byte
}

type cell struct {
	r     rune
	used  bool
	style Style
}

func NewScreen(width, height int) *Screen {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	screen := &Screen{}
	screen.resizeLocked(width, height)
	return screen
}

func (s *Screen) Resize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.width == width && s.height == height && len(s.cells) == height {
		return
	}
	s.resizeLocked(width, height)
}

func (s *Screen) Write(data []byte) {
	if len(data) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range data {
		s.writeByteLocked(b)
	}
}

func (s *Screen) Lines() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lines := make([]string, s.height)
	for y := 0; y < s.height; y++ {
		lines[y] = cellsString(s.cells[y], s.width)
	}
	return lines
}

func (s *Screen) StyledRows() []StyledRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]StyledRow, s.height)
	for y := 0; y < s.height; y++ {
		cells := make([]StyledCell, s.width)
		for x := 0; x < s.width; x++ {
			current := s.cells[y][x]
			r := current.r
			if r == 0 {
				r = ' '
			}
			cells[x] = StyledCell{Rune: r, Used: current.used, Style: current.style}
		}
		rows[y] = StyledRow{Cells: cells, Wrapped: y < len(s.wraps) && s.wraps[y]}
	}
	return rows
}

func (s *Screen) CaptureLines(preserveTrailing bool) []string {
	rows := s.CaptureRows(preserveTrailing)
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.Text
	}
	return lines
}

type CaptureRow struct {
	Text    string
	Wrapped bool
}

func (s *Screen) CaptureRows(preserveTrailing bool) []CaptureRow {
	if preserveTrailing {
		return s.CaptureRowsWithOptions(false, false)
	}
	return s.CaptureRowsWithOptions(true, true)
}

func (s *Screen) CaptureRowsWithOptions(includeEmptyCells bool, trimTrailing bool) []CaptureRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]CaptureRow, s.height)
	for y := 0; y < s.height; y++ {
		line := ""
		last := lastUsedCell(s.cells[y])
		if includeEmptyCells {
			if last >= 0 {
				end := expandedLineSize(s.width, last+1)
				line = cellsString(s.cells[y][:end], end)
			}
		} else {
			if last >= 0 {
				line = cellsString(s.cells[y][:last+1], last+1)
			}
		}
		if trimTrailing {
			line = strings.TrimRight(line, " ")
		}
		rows[y] = CaptureRow{Text: line, Wrapped: y < len(s.wraps) && s.wraps[y]}
	}
	return rows
}

func (s *Screen) CaptureRowsWithSequences(includeEmptyCells bool, trimTrailing bool) []CaptureRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]CaptureRow, s.height)
	lastStyle := Style{}
	for y := 0; y < s.height; y++ {
		end := captureLineEnd(s.cells[y], s.width, includeEmptyCells)
		var line strings.Builder
		for x := 0; x < end; x++ {
			current := s.cells[y][x]
			line.WriteString(styleTransition(lastStyle, current.style))
			lastStyle = current.style
			r := current.r
			if r == 0 {
				r = ' '
			}
			line.WriteRune(r)
		}
		text := line.String()
		if trimTrailing {
			text = strings.TrimRight(text, " ")
		}
		rows[y] = CaptureRow{Text: text, Wrapped: y < len(s.wraps) && s.wraps[y]}
	}
	return rows
}

func (s *Screen) resizeLocked(width, height int) {
	s.cells = resizeCells(s.cells, width, height)
	s.wraps = resizeWraps(s.wraps, height)
	if s.altScreen {
		s.mainCells = resizeCells(s.mainCells, width, height)
		s.mainWraps = resizeWraps(s.mainWraps, height)
		s.mainCursorX = clampInt(s.mainCursorX, 0, width)
		s.mainCursorY = clampInt(s.mainCursorY, 0, height-1)
		s.mainSavedX = clampInt(s.mainSavedX, 0, width)
		s.mainSavedY = clampInt(s.mainSavedY, 0, height-1)
	}
	s.width = width
	s.height = height
	s.cursorX = clampInt(s.cursorX, 0, width)
	s.cursorY = clampInt(s.cursorY, 0, height-1)
	s.savedX = clampInt(s.savedX, 0, width)
	s.savedY = clampInt(s.savedY, 0, height-1)
}

func (s *Screen) writeByteLocked(b byte) {
	switch s.state {
	case screenEscape:
		s.handleEscapeLocked(b)
		return
	case screenCSI:
		s.handleCSIByteLocked(b)
		return
	case screenIgnoreOne:
		s.state = screenNormal
		return
	}

	if b == 0x1b {
		s.utf8Buf = nil
		s.state = screenEscape
		return
	}
	if len(s.utf8Buf) > 0 || b >= utf8.RuneSelf {
		s.utf8Buf = append(s.utf8Buf, b)
		if !utf8.FullRune(s.utf8Buf) {
			return
		}
		r, size := utf8.DecodeRune(s.utf8Buf)
		s.utf8Buf = nil
		if r == utf8.RuneError && size == 1 {
			return
		}
		s.putRuneLocked(r)
		return
	}
	s.handleControlOrASCII(b)
}

func (s *Screen) handleControlOrASCII(b byte) {
	switch b {
	case '\r':
		s.cursorX = 0
	case '\n':
		s.lineFeedLocked()
	case '\b':
		if s.cursorX > 0 {
			s.cursorX--
		}
	case '\t':
		next := ((s.cursorX / 8) + 1) * 8
		for s.cursorX < next {
			s.putRuneLocked(' ')
		}
	case 0x00, 0x07:
		return
	default:
		if b >= 0x20 {
			s.putRuneLocked(rune(b))
		}
	}
}

func (s *Screen) handleEscapeLocked(b byte) {
	switch b {
	case '[':
		s.csi = s.csi[:0]
		s.state = screenCSI
	case 'c':
		s.resetLocked()
		s.state = screenNormal
	case '7':
		s.savedX, s.savedY = s.cursorX, s.cursorY
		s.state = screenNormal
	case '8':
		s.cursorX = clampInt(s.savedX, 0, s.width)
		s.cursorY = clampInt(s.savedY, 0, s.height-1)
		s.state = screenNormal
	case '(', ')', '*', '+', '-', '.', '/', '#':
		s.state = screenIgnoreOne
	default:
		s.state = screenNormal
	}
}

func (s *Screen) handleCSIByteLocked(b byte) {
	s.csi = append(s.csi, b)
	if len(s.csi) > 128 {
		s.state = screenNormal
		s.csi = s.csi[:0]
		return
	}
	if b >= 0x40 && b <= 0x7e {
		s.applyCSILocked(string(s.csi))
		s.csi = s.csi[:0]
		s.state = screenNormal
	}
}

func (s *Screen) applyCSILocked(seq string) {
	if seq == "" {
		return
	}
	final := seq[len(seq)-1]
	raw := seq[:len(seq)-1]
	private := strings.HasPrefix(raw, "?")
	params := parseCSIParams(raw)
	if private && (final == 'h' || final == 'l') {
		s.applyPrivateModeLocked(params, final == 'h')
		return
	}
	switch final {
	case 'H', 'f':
		row := csiParam(params, 0, 1)
		col := csiParam(params, 1, 1)
		s.cursorY = clampInt(row-1, 0, s.height-1)
		s.cursorX = clampInt(col-1, 0, s.width)
	case 'A':
		s.cursorY = clampInt(s.cursorY-csiParam(params, 0, 1), 0, s.height-1)
	case 'B':
		s.cursorY = clampInt(s.cursorY+csiParam(params, 0, 1), 0, s.height-1)
	case 'C':
		s.cursorX = clampInt(s.cursorX+csiParam(params, 0, 1), 0, s.width)
	case 'D':
		s.cursorX = clampInt(s.cursorX-csiParam(params, 0, 1), 0, s.width)
	case 'G':
		s.cursorX = clampInt(csiParam(params, 0, 1)-1, 0, s.width)
	case 'd':
		s.cursorY = clampInt(csiParam(params, 0, 1)-1, 0, s.height-1)
	case 'J':
		s.clearScreenLocked(csiParam(params, 0, 0))
	case 'K':
		s.clearLineLocked(csiParam(params, 0, 0))
	case 'X':
		s.eraseCharsLocked(csiParam(params, 0, 1))
	case 'P':
		s.deleteCharsLocked(csiParam(params, 0, 1))
	case '@':
		s.insertBlankCharsLocked(csiParam(params, 0, 1))
	case 'L':
		s.insertLinesLocked(csiParam(params, 0, 1))
	case 'M':
		s.deleteLinesLocked(csiParam(params, 0, 1))
	case 'S':
		s.scrollUpLocked(csiParam(params, 0, 1))
	case 'T':
		s.scrollDownLocked(csiParam(params, 0, 1))
	case 's':
		s.savedX, s.savedY = s.cursorX, s.cursorY
	case 'u':
		s.cursorX = clampInt(s.savedX, 0, s.width)
		s.cursorY = clampInt(s.savedY, 0, s.height-1)
	case 'm':
		s.applySGRLocked(params)
	case 'h', 'l', 'r':
		return
	}
}

func (s *Screen) applySGRLocked(params []int) {
	if len(params) == 0 {
		s.curStyle = Style{}
		return
	}
	for i := 0; i < len(params); i++ {
		n := params[i]
		if n == 38 || n == 48 {
			if i+2 < len(params) && params[i+1] == 5 && validColorComponent(params[i+2]) {
				color := Color{Mode: Color256, Value: uint8(params[i+2])}
				if n == 38 {
					s.curStyle.Fg = color
				} else {
					s.curStyle.Bg = color
				}
				i += 2
				continue
			}
			if i+4 < len(params) && params[i+1] == 2 && validColorComponent(params[i+2]) && validColorComponent(params[i+3]) && validColorComponent(params[i+4]) {
				color := Color{Mode: ColorRGB, R: uint8(params[i+2]), G: uint8(params[i+3]), B: uint8(params[i+4])}
				if n == 38 {
					s.curStyle.Fg = color
				} else {
					s.curStyle.Bg = color
				}
				i += 4
			}
			continue
		}
		switch n {
		case 0:
			s.curStyle = Style{}
		case 1:
			s.curStyle.Attrs |= AttrBold
		case 2:
			s.curStyle.Attrs |= AttrDim
		case 3:
			s.curStyle.Attrs |= AttrItalic
		case 4:
			s.curStyle.Attrs |= AttrUnderline
		case 5, 6:
			s.curStyle.Attrs |= AttrBlink
		case 7:
			s.curStyle.Attrs |= AttrReverse
		case 8:
			s.curStyle.Attrs |= AttrHidden
		case 9:
			s.curStyle.Attrs |= AttrStrikethrough
		case 21, 24:
			s.curStyle.Attrs &^= AttrUnderline
		case 22:
			s.curStyle.Attrs &^= AttrBold | AttrDim
		case 23:
			s.curStyle.Attrs &^= AttrItalic
		case 25:
			s.curStyle.Attrs &^= AttrBlink
		case 27:
			s.curStyle.Attrs &^= AttrReverse
		case 28:
			s.curStyle.Attrs &^= AttrHidden
		case 29:
			s.curStyle.Attrs &^= AttrStrikethrough
		case 30, 31, 32, 33, 34, 35, 36, 37:
			s.curStyle.Fg = Color{Mode: ColorANSI, Value: uint8(n - 30)}
		case 39:
			s.curStyle.Fg = Color{}
		case 40, 41, 42, 43, 44, 45, 46, 47:
			s.curStyle.Bg = Color{Mode: ColorANSI, Value: uint8(n - 40)}
		case 49:
			s.curStyle.Bg = Color{}
		case 90, 91, 92, 93, 94, 95, 96, 97:
			s.curStyle.Fg = Color{Mode: ColorANSI, Value: uint8(n - 90 + 8)}
		case 100, 101, 102, 103, 104, 105, 106, 107:
			s.curStyle.Bg = Color{Mode: ColorANSI, Value: uint8(n - 100 + 8)}
		}
	}
}

func validColorComponent(value int) bool {
	return value >= 0 && value <= 255
}

func (s *Screen) applyPrivateModeLocked(params []int, enable bool) {
	for _, mode := range params {
		switch mode {
		case 47, 1047:
			if enable {
				s.enterAltScreenLocked(false)
			} else {
				s.leaveAltScreenLocked(false)
			}
		case 1048:
			if enable {
				s.savedX, s.savedY = s.cursorX, s.cursorY
			} else {
				s.cursorX = clampInt(s.savedX, 0, s.width)
				s.cursorY = clampInt(s.savedY, 0, s.height-1)
			}
		case 1049:
			if enable {
				s.enterAltScreenLocked(true)
			} else {
				s.leaveAltScreenLocked(true)
			}
		}
	}
}

func (s *Screen) enterAltScreenLocked(saveCursor bool) {
	if saveCursor {
		s.savedX, s.savedY = s.cursorX, s.cursorY
	}
	if s.altScreen {
		s.clearScreenLocked(2)
		s.cursorX, s.cursorY = 0, 0
		return
	}
	s.mainCells = cloneCells(s.cells)
	s.mainWraps = append([]bool(nil), s.wraps...)
	s.mainCursorX, s.mainCursorY = s.cursorX, s.cursorY
	s.mainSavedX, s.mainSavedY = s.savedX, s.savedY
	s.cells = newCells(s.width, s.height)
	s.wraps = make([]bool, s.height)
	s.cursorX, s.cursorY = 0, 0
	s.savedX, s.savedY = 0, 0
	s.altScreen = true
}

func (s *Screen) leaveAltScreenLocked(restoreCursor bool) {
	if !s.altScreen {
		if restoreCursor {
			s.cursorX = clampInt(s.savedX, 0, s.width)
			s.cursorY = clampInt(s.savedY, 0, s.height-1)
		}
		return
	}
	if len(s.mainCells) > 0 {
		s.cells = resizeCells(s.mainCells, s.width, s.height)
	} else {
		s.cells = newCells(s.width, s.height)
	}
	s.wraps = resizeWraps(s.mainWraps, s.height)
	s.cursorX = clampInt(s.mainCursorX, 0, s.width)
	s.cursorY = clampInt(s.mainCursorY, 0, s.height-1)
	s.savedX = clampInt(s.mainSavedX, 0, s.width)
	s.savedY = clampInt(s.mainSavedY, 0, s.height-1)
	s.altScreen = false
	s.mainCells = nil
	s.mainWraps = nil
}

func parseCSIParams(raw string) []int {
	raw = strings.TrimLeft(raw, "?=>")
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == ':'
	})
	params := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			params[i] = 0
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			params[i] = 0
			continue
		}
		params[i] = v
	}
	return params
}

func csiParam(params []int, index, fallback int) int {
	if index >= len(params) || params[index] <= 0 {
		return fallback
	}
	return params[index]
}

func (s *Screen) resetLocked() {
	s.cells = newCells(s.width, s.height)
	s.wraps = make([]bool, s.height)
	s.altScreen = false
	s.mainCells = nil
	s.mainWraps = nil
	s.mainCursorX, s.mainCursorY = 0, 0
	s.mainSavedX, s.mainSavedY = 0, 0
	s.cursorX, s.cursorY = 0, 0
	s.savedX, s.savedY = 0, 0
	s.curStyle = Style{}
	s.state = screenNormal
	s.csi = s.csi[:0]
	s.utf8Buf = nil
}

func (s *Screen) putRuneLocked(r rune) {
	if s.width <= 0 || s.height <= 0 {
		return
	}
	if s.cursorX >= s.width {
		s.autoWrapLocked()
	}
	if s.cursorY < 0 || s.cursorY >= s.height {
		return
	}
	if r < 0x20 {
		return
	}
	s.cells[s.cursorY][s.cursorX] = cell{r: r, used: true, style: s.curStyle}
	s.cursorX++
}

func (s *Screen) lineFeedLocked() {
	s.setWrappedLocked(s.cursorY, false)
	if s.cursorY >= s.height-1 {
		s.scrollUpLocked(1)
		s.cursorY = s.height - 1
		s.setWrappedLocked(s.cursorY, false)
		return
	}
	s.cursorY++
	s.setWrappedLocked(s.cursorY, false)
}

func (s *Screen) autoWrapLocked() {
	s.setWrappedLocked(s.cursorY, true)
	if s.cursorY >= s.height-1 {
		s.scrollUpLocked(1)
		s.cursorY = s.height - 1
	} else {
		s.cursorY++
	}
	s.cursorX = 0
	s.setWrappedLocked(s.cursorY, false)
}

func (s *Screen) clearScreenLocked(mode int) {
	switch mode {
	case 1:
		for y := 0; y <= s.cursorY; y++ {
			start, end := 0, s.width
			if y == s.cursorY {
				end = clampInt(s.cursorX+1, 0, s.width)
			}
			s.fillLineLocked(y, start, end, ' ', false)
		}
	case 2, 3:
		for y := 0; y < s.height; y++ {
			s.fillLineLocked(y, 0, s.width, ' ', false)
			s.setWrappedLocked(y, false)
		}
	default:
		for y := s.cursorY; y < s.height; y++ {
			start := 0
			if y == s.cursorY {
				start = clampInt(s.cursorX, 0, s.width)
			}
			s.fillLineLocked(y, start, s.width, ' ', false)
		}
	}
}

func (s *Screen) clearLineLocked(mode int) {
	switch mode {
	case 1:
		s.fillLineLocked(s.cursorY, 0, clampInt(s.cursorX+1, 0, s.width), ' ', false)
	case 2:
		s.fillLineLocked(s.cursorY, 0, s.width, ' ', false)
		s.setWrappedLocked(s.cursorY, false)
	default:
		s.fillLineLocked(s.cursorY, clampInt(s.cursorX, 0, s.width), s.width, ' ', false)
	}
}

func (s *Screen) eraseCharsLocked(count int) {
	start := clampInt(s.cursorX, 0, s.width)
	end := clampInt(start+count, 0, s.width)
	s.fillLineLocked(s.cursorY, start, end, ' ', false)
}

func (s *Screen) deleteCharsLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	row := s.cells[s.cursorY]
	x := clampInt(s.cursorX, 0, s.width)
	if x >= s.width {
		return
	}
	copy(row[x:], row[minInt(x+count, s.width):])
	for i := maxInt(s.width-count, x); i < s.width; i++ {
		row[i] = blankCellWithBackground(s.curStyle.Bg)
	}
}

func (s *Screen) insertBlankCharsLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	row := s.cells[s.cursorY]
	x := clampInt(s.cursorX, 0, s.width)
	if x >= s.width {
		return
	}
	count = minInt(count, s.width-x)
	copy(row[x+count:], row[x:s.width-count])
	for i := x; i < x+count; i++ {
		row[i] = blankCellWithBackground(s.curStyle.Bg)
	}
}

func (s *Screen) insertLinesLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	count = minInt(count, s.height-s.cursorY)
	for y := s.height - 1; y >= s.cursorY+count; y-- {
		copy(s.cells[y], s.cells[y-count])
		s.setWrappedLocked(y, s.isWrappedLocked(y-count))
	}
	for y := s.cursorY; y < s.cursorY+count; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
		s.setWrappedLocked(y, false)
	}
}

func (s *Screen) deleteLinesLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	count = minInt(count, s.height-s.cursorY)
	for y := s.cursorY; y < s.height-count; y++ {
		copy(s.cells[y], s.cells[y+count])
		s.setWrappedLocked(y, s.isWrappedLocked(y+count))
	}
	for y := s.height - count; y < s.height; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
		s.setWrappedLocked(y, false)
	}
}

func (s *Screen) scrollUpLocked(count int) {
	if s.height <= 0 || count <= 0 {
		return
	}
	count = minInt(count, s.height)
	for y := 0; y < s.height-count; y++ {
		copy(s.cells[y], s.cells[y+count])
		s.setWrappedLocked(y, s.isWrappedLocked(y+count))
	}
	for y := s.height - count; y < s.height; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
		s.setWrappedLocked(y, false)
	}
}

func (s *Screen) scrollDownLocked(count int) {
	if s.height <= 0 || count <= 0 {
		return
	}
	count = minInt(count, s.height)
	for y := s.height - 1; y >= count; y-- {
		copy(s.cells[y], s.cells[y-count])
		s.setWrappedLocked(y, s.isWrappedLocked(y-count))
	}
	for y := 0; y < count; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
		s.setWrappedLocked(y, false)
	}
}

func (s *Screen) setWrappedLocked(y int, wrapped bool) {
	if y >= 0 && y < len(s.wraps) {
		s.wraps[y] = wrapped
	}
}

func (s *Screen) isWrappedLocked(y int) bool {
	return y >= 0 && y < len(s.wraps) && s.wraps[y]
}

func (s *Screen) fillLineLocked(y, start, end int, r rune, used bool) {
	if y < 0 || y >= s.height {
		return
	}
	start = clampInt(start, 0, s.width)
	end = clampInt(end, 0, s.width)
	for x := start; x < end; x++ {
		s.cells[y][x] = cell{r: r, used: used, style: Style{Bg: s.curStyle.Bg}}
	}
}

func newCells(width, height int) [][]cell {
	cells := make([][]cell, height)
	for y := range cells {
		cells[y] = make([]cell, width)
		for x := range cells[y] {
			cells[y][x] = blankCell()
		}
	}
	return cells
}

func cloneCells(cells [][]cell) [][]cell {
	clone := make([][]cell, len(cells))
	for y := range cells {
		clone[y] = make([]cell, len(cells[y]))
		copy(clone[y], cells[y])
	}
	return clone
}

func resizeCells(old [][]cell, width, height int) [][]cell {
	cells := newCells(width, height)
	for y := 0; y < minInt(len(old), height); y++ {
		copy(cells[y], old[y])
	}
	return cells
}

func resizeWraps(old []bool, height int) []bool {
	wraps := make([]bool, height)
	copy(wraps, old)
	return wraps
}

func expandedLineSize(width int, used int) int {
	if used <= 0 || width <= 0 {
		return 0
	}
	size := used
	if quarter := width / 4; size < quarter {
		size = quarter
	} else if half := width / 2; size < half {
		size = half
	} else if width > size {
		size = width
	}
	return clampInt(size, 0, width)
}

func cellsString(cells []cell, width int) string {
	if width > len(cells) {
		width = len(cells)
	}
	runes := make([]rune, width)
	for i := 0; i < width; i++ {
		r := cells[i].r
		if r == 0 {
			r = ' '
		}
		runes[i] = r
	}
	return string(runes)
}

func lastUsedCell(cells []cell) int {
	for i := len(cells) - 1; i >= 0; i-- {
		if cells[i].used {
			return i
		}
	}
	return -1
}

func captureLineEnd(cells []cell, width int, includeEmptyCells bool) int {
	last := lastUsedCell(cells)
	if includeEmptyCells {
		for i := len(cells) - 1; i >= 0; i-- {
			if cells[i].used || cells[i].style != (Style{}) {
				last = i
				break
			}
		}
		if last >= 0 {
			return expandedLineSize(width, last+1)
		}
		return 0
	}
	if last >= 0 {
		return last + 1
	}
	return 0
}

func blankCell() cell {
	return cell{r: ' '}
}

func blankCellWithBackground(background Color) cell {
	return cell{r: ' ', style: Style{Bg: background}}
}

func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
