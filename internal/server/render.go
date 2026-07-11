package server

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

type renderCell struct {
	r     rune
	style terminal.Style
}

func renderPanes(width, height int, panes []*model.Pane, screenRows map[int][]terminal.StyledRow) []byte {
	if width <= 0 || height <= 0 {
		return nil
	}
	canvas := renderStyledPaneCanvas(width, height, panes, screenRows)
	var out bytes.Buffer
	out.WriteString("\x1b[?25l\x1b[2J")
	for y, row := range canvas {
		out.WriteString(fmt.Sprintf("\x1b[%d;1H\x1b[0m", y+1))
		lastStyle := terminal.Style{}
		for _, cell := range row {
			out.WriteString(terminal.StyleSequence(lastStyle, cell.style))
			lastStyle = cell.style
			out.WriteRune(cell.r)
		}
	}
	out.WriteString("\x1b[?25h")
	return out.Bytes()
}

func renderStyledPaneCanvas(width, height int, panes []*model.Pane, screenRows map[int][]terminal.StyledRow) [][]renderCell {
	canvas := make([][]renderCell, height)
	covered := make([][]bool, height)
	for y := range canvas {
		canvas[y] = make([]renderCell, width)
		covered[y] = make([]bool, width)
		for x := range canvas[y] {
			canvas[y][x].r = ' '
		}
	}
	for _, pane := range panes {
		drawStyledPane(canvas, covered, pane, screenRows)
	}
	drawStyledBorders(canvas, covered)
	return canvas
}

func drawStyledPane(canvas [][]renderCell, covered [][]bool, pane *model.Pane, screenRows map[int][]terminal.StyledRow) {
	if pane == nil || len(canvas) == 0 || pane.Width <= 0 || pane.Height <= 0 {
		return
	}
	height, width := len(canvas), len(canvas[0])
	left := clamp(pane.Left, 0, width)
	top := clamp(pane.Top, 0, height)
	right := clamp(pane.Left+pane.Width, 0, width)
	bottom := clamp(pane.Top+pane.Height, 0, height)
	if left >= right || top >= bottom {
		return
	}
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			covered[y][x] = true
		}
	}
	if rows, ok := screenRows[pane.ID]; ok {
		for rowIndex, row := range rows {
			y := top + rowIndex
			if y >= bottom {
				break
			}
			for cellIndex, cell := range row.Cells {
				x := left + cellIndex
				if x >= right {
					break
				}
				r := cell.Rune
				if r == 0 {
					r = ' '
				}
				canvas[y][x] = renderCell{r: r, style: cell.Style}
			}
		}
		return
	}
	textLines := visibleTextLines(pane.History.Bytes(), bottom-top)
	startY := bottom - len(textLines)
	for i, line := range textLines {
		y := startY + i
		x := left
		for _, r := range line {
			if x >= right {
				break
			}
			canvas[y][x].r = r
			x++
		}
	}
}

func drawStyledBorders(canvas [][]renderCell, covered [][]bool) {
	height := len(canvas)
	if height == 0 {
		return
	}
	width := len(canvas[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if covered[y][x] {
				continue
			}
			left := x > 0 && covered[y][x-1]
			right := x+1 < width && covered[y][x+1]
			up := y > 0 && covered[y-1][x]
			down := y+1 < height && covered[y+1][x]
			switch {
			case (left || right) && (up || down):
				canvas[y][x].r = '+'
			case left || right:
				canvas[y][x].r = '|'
			case up || down:
				canvas[y][x].r = '-'
			}
		}
	}
}

func renderPaneCanvas(width, height int, panes []*model.Pane, screenLines ...map[int][]string) []string {
	paneLines := map[int][]string(nil)
	if len(screenLines) > 0 {
		paneLines = screenLines[0]
	}
	canvas := make([][]rune, height)
	covered := make([][]bool, height)
	for y := range canvas {
		canvas[y] = []rune(strings.Repeat(" ", width))
		covered[y] = make([]bool, width)
	}

	for _, pane := range panes {
		drawPane(canvas, covered, pane, paneLines)
	}
	drawBorders(canvas, covered)

	lines := make([]string, height)
	for y := range canvas {
		lines[y] = string(canvas[y])
	}
	return lines
}

func drawPane(canvas [][]rune, covered [][]bool, pane *model.Pane, screenLines map[int][]string) {
	if pane == nil {
		return
	}
	height := len(canvas)
	if height == 0 || pane.Width <= 0 || pane.Height <= 0 {
		return
	}
	width := len(canvas[0])
	left := clamp(pane.Left, 0, width)
	top := clamp(pane.Top, 0, height)
	right := clamp(pane.Left+pane.Width, 0, width)
	bottom := clamp(pane.Top+pane.Height, 0, height)
	if left >= right || top >= bottom {
		return
	}
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			covered[y][x] = true
		}
	}

	if lines, ok := screenLines[pane.ID]; ok {
		drawTextLines(canvas, left, top, right, bottom, lines, true)
		return
	}
	textLines := visibleTextLines(pane.History.Bytes(), bottom-top)
	drawTextLines(canvas, left, top, right, bottom, textLines, false)
}

func drawTextLines(canvas [][]rune, left, top, right, bottom int, textLines []string, fromTop bool) {
	startY := top
	if !fromTop {
		startY = bottom - len(textLines)
	}
	for i, line := range textLines {
		y := startY + i
		if y < top || y >= bottom {
			continue
		}
		x := left
		for _, r := range line {
			if x >= right {
				break
			}
			if r == '\t' {
				for j := 0; j < 4 && x < right; j++ {
					canvas[y][x] = ' '
					x++
				}
				continue
			}
			canvas[y][x] = r
			x++
		}
	}
}

func drawBorders(canvas [][]rune, covered [][]bool) {
	height := len(canvas)
	if height == 0 {
		return
	}
	width := len(canvas[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if covered[y][x] {
				continue
			}
			left := x > 0 && covered[y][x-1]
			right := x+1 < width && covered[y][x+1]
			up := y > 0 && covered[y-1][x]
			down := y+1 < height && covered[y+1][x]
			switch {
			case (left || right) && (up || down):
				canvas[y][x] = '+'
			case left || right:
				canvas[y][x] = '|'
			case up || down:
				canvas[y][x] = '-'
			}
		}
	}
}

func visibleTextLines(data []byte, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	clean := stripANSI(data)
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	lines := strings.Split(clean, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = trimRunes(lines[i], 4096)
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func stripANSI(data []byte) string {
	var out strings.Builder
	for i := 0; i < len(data); {
		b := data[i]
		if b == 0x1b {
			i++
			if i < len(data) && data[i] == '[' {
				i++
				for i < len(data) {
					if data[i] >= 0x40 && data[i] <= 0x7e {
						i++
						break
					}
					i++
				}
			}
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		i += size
		if r == '\n' || r == '\r' || r == '\t' || r >= 0x20 {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
