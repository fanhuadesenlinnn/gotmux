package terminal

import (
	"strconv"
	"strings"
)

const (
	ColorDefault uint8 = iota
	ColorANSI
	Color256
	ColorRGB
)

const (
	AttrBold uint16 = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrHidden
	AttrStrikethrough
)

type Color struct {
	Mode  uint8
	Value uint8
	R     uint8
	G     uint8
	B     uint8
}

type Style struct {
	Fg    Color
	Bg    Color
	Attrs uint16
}

type StyledCell struct {
	Rune  rune
	Used  bool
	Style Style
}

type StyledRow struct {
	Cells   []StyledCell
	Wrapped bool
}

func styleTransition(last, current Style) string {
	var out strings.Builder
	removed := last.Attrs &^ current.Attrs
	baseAttrs := last.Attrs
	params := make([]int, 0, 9)
	reset := removed != 0
	if reset {
		params = append(params, 0)
		baseAttrs = 0
	}
	attrs := []struct {
		mask uint16
		code int
	}{
		{AttrBold, 1},
		{AttrDim, 2},
		{AttrItalic, 3},
		{AttrUnderline, 4},
		{AttrBlink, 5},
		{AttrReverse, 7},
		{AttrHidden, 8},
		{AttrStrikethrough, 9},
	}
	for _, attr := range attrs {
		if current.Attrs&attr.mask != 0 && baseAttrs&attr.mask == 0 {
			params = append(params, attr.code)
		}
	}
	if len(params) > 0 {
		writeSGR(&out, params)
	}
	writeColorTransition(&out, last.Fg, current.Fg, true, reset)
	writeColorTransition(&out, last.Bg, current.Bg, false, reset)
	return out.String()
}

func StyleSequence(last, current Style) string {
	return styleTransition(last, current)
}

func writeColorTransition(out *strings.Builder, last, current Color, foreground, reset bool) {
	if !reset && last == current {
		return
	}
	if reset && current.Mode == ColorDefault {
		return
	}
	params := colorSGR(current, foreground)
	if len(params) > 0 {
		writeSGR(out, params)
	}
}

func colorSGR(color Color, foreground bool) []int {
	switch color.Mode {
	case ColorANSI:
		value := int(color.Value)
		if value < 8 {
			if foreground {
				return []int{30 + value}
			}
			return []int{40 + value}
		}
		if foreground {
			return []int{90 + value - 8}
		}
		return []int{100 + value - 8}
	case Color256:
		if foreground {
			return []int{38, 5, int(color.Value)}
		}
		return []int{48, 5, int(color.Value)}
	case ColorRGB:
		if foreground {
			return []int{38, 2, int(color.R), int(color.G), int(color.B)}
		}
		return []int{48, 2, int(color.R), int(color.G), int(color.B)}
	default:
		if foreground {
			return []int{39}
		}
		return []int{49}
	}
}

func writeSGR(out *strings.Builder, params []int) {
	out.WriteString("\x1b[")
	for i, value := range params {
		if i > 0 {
			out.WriteByte(';')
		}
		out.WriteString(strconv.Itoa(value))
	}
	out.WriteByte('m')
}
