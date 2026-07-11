package terminal

import "strings"

func (s *Screen) HistoryLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.history)
}

func (s *Screen) SetHistoryLimit(limit int) {
	if limit < 0 {
		limit = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.historyLimit = limit
	s.trimHistoryLocked()
}

func (s *Screen) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	s.historyWraps = nil
}

func (s *Screen) HistoryRows(includeEmptyCells bool, trimTrailing bool) []CaptureRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lastStyle := Style{}
	return captureGridRows(s.history, s.historyWraps, s.width, includeEmptyCells, trimTrailing, false, &lastStyle)
}

func (s *Screen) CaptureAllRowsWithOptions(includeEmptyCells bool, trimTrailing bool, withSequences bool) []CaptureRow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lastStyle := Style{}
	rows := captureGridRows(s.history, s.historyWraps, s.width, includeEmptyCells, trimTrailing, withSequences, &lastStyle)
	rows = append(rows, captureGridRows(s.cells, s.wraps, s.width, includeEmptyCells, trimTrailing, withSequences, &lastStyle)...)
	return rows
}

func (s *Screen) appendHistoryLocked(count int) {
	if s.historyLimit == 0 {
		return
	}
	for y := 0; y < count && y < len(s.cells); y++ {
		row := make([]cell, len(s.cells[y]))
		copy(row, s.cells[y])
		s.history = append(s.history, row)
		s.historyWraps = append(s.historyWraps, s.isWrappedLocked(y))
	}
	s.trimHistoryLocked()
}

func (s *Screen) appendUsedScreenToHistoryLocked() {
	last := 0
	for y, row := range s.cells {
		for _, current := range row {
			if current.used {
				last = y + 1
				break
			}
		}
	}
	if last > 0 {
		s.appendHistoryLocked(last)
	}
}

func (s *Screen) trimHistoryLocked() {
	if len(s.history) <= s.historyLimit {
		return
	}
	drop := len(s.history) - s.historyLimit
	copy(s.history, s.history[drop:])
	s.history = s.history[:s.historyLimit]
	copy(s.historyWraps, s.historyWraps[drop:])
	s.historyWraps = s.historyWraps[:s.historyLimit]
}

func captureGridRows(cells [][]cell, wraps []bool, width int, includeEmptyCells bool, trimTrailing bool, withSequences bool, lastStyle *Style) []CaptureRow {
	rows := make([]CaptureRow, len(cells))
	for y, row := range cells {
		end := captureLineEnd(row, minInt(width, len(row)), includeEmptyCells)
		var line strings.Builder
		for x := 0; x < end; x++ {
			current := row[x]
			if current.width == 0 {
				continue
			}
			if withSequences {
				line.WriteString(styleTransition(*lastStyle, current.style))
				*lastStyle = current.style
			}
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
		rows[y] = CaptureRow{Text: text, Wrapped: y < len(wraps) && wraps[y]}
	}
	return rows
}
