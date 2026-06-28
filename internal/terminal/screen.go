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

	altScreen   bool
	mainCells   [][]cell
	mainCursorX int
	mainCursorY int
	mainSavedX  int
	mainSavedY  int

	cursorX int
	cursorY int
	savedX  int
	savedY  int

	state   int
	csi     []byte
	utf8Buf []byte
}

type cell struct {
	r    rune
	used bool
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

func (s *Screen) CaptureLines(preserveTrailing bool) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lines := make([]string, s.height)
	for y := 0; y < s.height; y++ {
		line := cellsString(s.cells[y], s.width)
		if preserveTrailing {
			last := lastUsedCell(s.cells[y])
			if last < 0 {
				line = ""
			} else {
				line = cellsString(s.cells[y][:last+1], last+1)
			}
		} else {
			line = strings.TrimRight(line, " ")
		}
		lines[y] = line
	}
	return lines
}

func (s *Screen) resizeLocked(width, height int) {
	s.cells = resizeCells(s.cells, width, height)
	if s.altScreen {
		s.mainCells = resizeCells(s.mainCells, width, height)
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
	case 'm', 'h', 'l', 'r':
		return
	}
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
	s.mainCursorX, s.mainCursorY = s.cursorX, s.cursorY
	s.mainSavedX, s.mainSavedY = s.savedX, s.savedY
	s.cells = newCells(s.width, s.height)
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
	s.cursorX = clampInt(s.mainCursorX, 0, s.width)
	s.cursorY = clampInt(s.mainCursorY, 0, s.height-1)
	s.savedX = clampInt(s.mainSavedX, 0, s.width)
	s.savedY = clampInt(s.mainSavedY, 0, s.height-1)
	s.altScreen = false
	s.mainCells = nil
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
	s.altScreen = false
	s.mainCells = nil
	s.mainCursorX, s.mainCursorY = 0, 0
	s.mainSavedX, s.mainSavedY = 0, 0
	s.cursorX, s.cursorY = 0, 0
	s.savedX, s.savedY = 0, 0
	s.state = screenNormal
	s.csi = s.csi[:0]
	s.utf8Buf = nil
}

func (s *Screen) putRuneLocked(r rune) {
	if s.width <= 0 || s.height <= 0 {
		return
	}
	if s.cursorX >= s.width {
		s.cursorX = 0
		s.lineFeedLocked()
	}
	if s.cursorY < 0 || s.cursorY >= s.height {
		return
	}
	if r < 0x20 {
		return
	}
	s.cells[s.cursorY][s.cursorX] = cell{r: r, used: true}
	s.cursorX++
}

func (s *Screen) lineFeedLocked() {
	if s.cursorY >= s.height-1 {
		s.scrollUpLocked(1)
		s.cursorY = s.height - 1
		return
	}
	s.cursorY++
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
		row[i] = blankCell()
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
		row[i] = blankCell()
	}
}

func (s *Screen) insertLinesLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	count = minInt(count, s.height-s.cursorY)
	for y := s.height - 1; y >= s.cursorY+count; y-- {
		copy(s.cells[y], s.cells[y-count])
	}
	for y := s.cursorY; y < s.cursorY+count; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
	}
}

func (s *Screen) deleteLinesLocked(count int) {
	if s.cursorY < 0 || s.cursorY >= s.height || count <= 0 {
		return
	}
	count = minInt(count, s.height-s.cursorY)
	for y := s.cursorY; y < s.height-count; y++ {
		copy(s.cells[y], s.cells[y+count])
	}
	for y := s.height - count; y < s.height; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
	}
}

func (s *Screen) scrollUpLocked(count int) {
	if s.height <= 0 || count <= 0 {
		return
	}
	count = minInt(count, s.height)
	for y := 0; y < s.height-count; y++ {
		copy(s.cells[y], s.cells[y+count])
	}
	for y := s.height - count; y < s.height; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
	}
}

func (s *Screen) scrollDownLocked(count int) {
	if s.height <= 0 || count <= 0 {
		return
	}
	count = minInt(count, s.height)
	for y := s.height - 1; y >= count; y-- {
		copy(s.cells[y], s.cells[y-count])
	}
	for y := 0; y < count; y++ {
		s.fillLineLocked(y, 0, s.width, ' ', false)
	}
}

func (s *Screen) fillLineLocked(y, start, end int, r rune, used bool) {
	if y < 0 || y >= s.height {
		return
	}
	start = clampInt(start, 0, s.width)
	end = clampInt(end, 0, s.width)
	for x := start; x < end; x++ {
		s.cells[y][x] = cell{r: r, used: used}
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

func blankCell() cell {
	return cell{r: ' '}
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
