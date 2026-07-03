package server

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
)

type formatContext struct {
	session *model.Session
	window  *model.Window
	pane    *model.Pane
}

func formatString(template string, ctx formatContext) string {
	if template == "" {
		return ""
	}
	out := expandFormatExpressions(template, ctx)
	for _, alias := range []struct {
		key   string
		value string
	}{
		{"#S", formatValue("session_name", ctx)},
		{"#I", formatValue("window_index", ctx)},
		{"#W", formatValue("window_name", ctx)},
		{"#P", formatValue("pane_index", ctx)},
		{"#H", formatValue("host", ctx)},
		{"#h", formatValue("host_short", ctx)},
	} {
		out = strings.ReplaceAll(out, alias.key, alias.value)
	}
	return out
}

func expandFormatExpressions(template string, ctx formatContext) string {
	var out strings.Builder
	for i := 0; i < len(template); {
		if i+1 < len(template) && template[i] == '#' && template[i+1] == '{' {
			end := findFormatEnd(template, i)
			if end == -1 {
				out.WriteString(template[i:])
				break
			}
			out.WriteString(formatExpression(template[i+2:end], ctx))
			i = end + 1
			continue
		}
		out.WriteByte(template[i])
		i++
	}
	return out.String()
}

func findFormatEnd(text string, start int) int {
	depth := 0
	for i := start + 2; i < len(text); i++ {
		if i+1 < len(text) && text[i] == '#' && text[i+1] == '{' {
			depth++
			i++
			continue
		}
		if text[i] != '}' {
			continue
		}
		if depth == 0 {
			return i
		}
		depth--
	}
	return -1
}

func formatExpression(expr string, ctx formatContext) string {
	if strings.HasPrefix(expr, "?") {
		parts := splitFormatExpression(expr[1:])
		if len(parts) != 3 {
			return ""
		}
		if formatConditionTruthy(parts[0], ctx) {
			return formatString(parts[1], ctx)
		}
		return formatString(parts[2], ctx)
	}
	if strings.HasPrefix(expr, "=") {
		return formatTrimExpression(expr[1:], ctx)
	}
	return formatValue(expr, ctx)
}

func splitFormatExpression(expr string) []string {
	var parts []string
	start := 0
	depth := 0
	for i := 0; i < len(expr); i++ {
		if i+1 < len(expr) && expr[i] == '#' && expr[i+1] == '{' {
			depth++
			i++
			continue
		}
		if expr[i] == '}' && depth > 0 {
			depth--
			continue
		}
		if expr[i] == ',' && depth == 0 {
			parts = append(parts, expr[start:i])
			start = i + 1
		}
	}
	parts = append(parts, expr[start:])
	return parts
}

func formatConditionTruthy(expr string, ctx formatContext) bool {
	value := ""
	if strings.Contains(expr, "#") {
		value = formatString(expr, ctx)
	} else if formatted, ok := formatValueLookup(expr, ctx); ok {
		value = formatted
	}
	value = strings.TrimSpace(value)
	return value != "" && value != "0"
}

func formatTrimExpression(expr string, ctx formatContext) string {
	separator := strings.Index(expr, ":")
	if separator <= 0 {
		return ""
	}
	width, err := strconv.Atoi(expr[:separator])
	if err != nil || width < 0 {
		return ""
	}
	value := formatFieldOrString(expr[separator+1:], ctx)
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width])
}

func formatFieldOrString(expr string, ctx formatContext) string {
	if strings.Contains(expr, "#") {
		return formatString(expr, ctx)
	}
	if formatted, ok := formatValueLookup(expr, ctx); ok {
		return formatted
	}
	return ""
}

func formatTruthy(template string, ctx formatContext) bool {
	value := strings.TrimSpace(formatString(template, ctx))
	return value != "" && value != "0"
}

func formatValue(key string, ctx formatContext) string {
	value, _ := formatValueLookup(key, ctx)
	return value
}

func formatValueLookup(key string, ctx formatContext) (string, bool) {
	switch key {
	case "session_name":
		if ctx.session != nil {
			return ctx.session.Name, true
		}
		return "", true
	case "session_id":
		if ctx.session != nil {
			return "$" + strconv.Itoa(ctx.session.ID), true
		}
		return "", true
	case "session_windows":
		if ctx.session != nil {
			return strconv.Itoa(len(ctx.session.Windows)), true
		}
		return "", true
	case "session_attached":
		if ctx.session != nil {
			return strconv.Itoa(ctx.session.Attached), true
		}
		return "", true
	case "host":
		return hostName(), true
	case "host_short":
		host := hostName()
		if idx := strings.Index(host, "."); idx > 0 {
			return host[:idx], true
		}
		return host, true
	case "time":
		return strconv.FormatInt(time.Now().Unix(), 10), true
	case "window_id":
		if ctx.window != nil {
			return "@" + strconv.Itoa(ctx.window.ID), true
		}
		return "", true
	case "window_index":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Index), true
		}
		return "", true
	case "window_name":
		if ctx.window != nil {
			return ctx.window.Name, true
		}
		return "", true
	case "window_width":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Width), true
		}
		return "", true
	case "window_height":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Height), true
		}
		return "", true
	case "window_panes":
		if ctx.window != nil {
			return strconv.Itoa(len(ctx.window.Panes)), true
		}
		return "", true
	case "window_active":
		if ctx.session != nil && ctx.window != nil {
			active := ctx.session.ActiveWindow()
			if active == ctx.window {
				return "1", true
			}
		}
		return "0", true
	case "window_flags":
		return windowFlags(ctx.session, ctx.window), true
	case "window_zoomed_flag":
		if ctx.window != nil && ctx.window.Zoomed {
			return "1", true
		}
		return "0", true
	case "pane_id":
		if ctx.pane != nil {
			return "%" + strconv.Itoa(ctx.pane.ID), true
		}
		return "", true
	case "pane_index":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Index), true
		}
		return "", true
	case "pane_left":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Left), true
		}
		return "", true
	case "pane_top":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Top), true
		}
		return "", true
	case "pane_active":
		if ctx.window != nil && ctx.pane != nil {
			active := ctx.window.ActivePane()
			if active != nil && active.ID == ctx.pane.ID {
				return "1", true
			}
		}
		return "0", true
	case "pane_width":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Width), true
		}
		return "", true
	case "pane_height":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Height), true
		}
		return "", true
	case "pane_current_command":
		if ctx.pane != nil {
			return currentCommandName(ctx.pane.Command), true
		}
		return "", true
	case "pane_title":
		if ctx.pane != nil {
			return currentCommandName(ctx.pane.Command), true
		}
		return "", true
	}
	return "", false
}

func hostName() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}

func currentCommandName(command []string) string {
	if len(command) == 0 {
		return model.DefaultShellName()
	}
	name := command[0]
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.TrimPrefix(name, "-")
}
