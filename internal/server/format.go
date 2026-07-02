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
	out := template
	for strings.Contains(out, "#{") {
		start := strings.Index(out, "#{")
		end := strings.Index(out[start:], "}")
		if end == -1 {
			break
		}
		end += start
		key := out[start+2 : end]
		out = out[:start] + formatValue(key, ctx) + out[end+1:]
	}
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

func formatTruthy(template string, ctx formatContext) bool {
	value := strings.TrimSpace(formatString(template, ctx))
	return value != "" && value != "0"
}

func formatValue(key string, ctx formatContext) string {
	switch key {
	case "session_name":
		if ctx.session != nil {
			return ctx.session.Name
		}
	case "session_id":
		if ctx.session != nil {
			return "$" + strconv.Itoa(ctx.session.ID)
		}
	case "session_windows":
		if ctx.session != nil {
			return strconv.Itoa(len(ctx.session.Windows))
		}
	case "session_attached":
		if ctx.session != nil {
			return strconv.Itoa(ctx.session.Attached)
		}
	case "host":
		return hostName()
	case "host_short":
		host := hostName()
		if idx := strings.Index(host, "."); idx > 0 {
			return host[:idx]
		}
		return host
	case "time":
		return strconv.FormatInt(time.Now().Unix(), 10)
	case "window_id":
		if ctx.window != nil {
			return "@" + strconv.Itoa(ctx.window.ID)
		}
	case "window_index":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Index)
		}
	case "window_name":
		if ctx.window != nil {
			return ctx.window.Name
		}
	case "window_width":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Width)
		}
	case "window_height":
		if ctx.window != nil {
			return strconv.Itoa(ctx.window.Height)
		}
	case "window_panes":
		if ctx.window != nil {
			return strconv.Itoa(len(ctx.window.Panes))
		}
	case "window_active":
		if ctx.session != nil && ctx.window != nil {
			active := ctx.session.ActiveWindow()
			if active == ctx.window {
				return "1"
			}
		}
		return "0"
	case "pane_id":
		if ctx.pane != nil {
			return "%" + strconv.Itoa(ctx.pane.ID)
		}
	case "pane_index":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Index)
		}
	case "pane_left":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Left)
		}
	case "pane_top":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Top)
		}
	case "pane_active":
		if ctx.window != nil && ctx.pane != nil {
			active := ctx.window.ActivePane()
			if active != nil && active.ID == ctx.pane.ID {
				return "1"
			}
		}
		return "0"
	case "pane_width":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Width)
		}
	case "pane_height":
		if ctx.pane != nil {
			return strconv.Itoa(ctx.pane.Height)
		}
	case "pane_current_command":
		if ctx.pane != nil {
			return currentCommandName(ctx.pane.Command)
		}
	case "pane_title":
		if ctx.pane != nil {
			return currentCommandName(ctx.pane.Command)
		}
	}
	return ""
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
