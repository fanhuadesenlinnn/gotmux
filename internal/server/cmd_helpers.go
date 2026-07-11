package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func normalizeCommandName(name string) string {
	switch name {
	case "new":
		return "new-session"
	case "attach", "at":
		return "attach-session"
	case "detach":
		return "detach-client"
	case "has":
		return "has-session"
	case "if":
		return "if-shell"
	case "ls":
		return "list-sessions"
	case "lsc":
		return "list-clients"
	case "lsp":
		return "list-panes"
	case "lsw":
		return "list-windows"
	case "lscm":
		return "list-commands"
	case "neww":
		return "new-window"
	case "newp":
		return "new-pane"
	case "splitw":
		return "split-window"
	case "selectw":
		return "select-window"
	case "confirm":
		return "confirm-before"
	case "display":
		return "display-message"
	case "menu":
		return "display-menu"
	case "displayp":
		return "display-panes"
	case "popup":
		return "display-popup"
	case "findw":
		return "find-window"
	case "last":
		return "last-window"
	case "next":
		return "next-window"
	case "prev":
		return "previous-window"
	case "refresh":
		return "refresh-client"
	case "selectp":
		return "select-pane"
	case "lastp":
		return "last-pane"
	case "nextl":
		return "next-layout"
	case "prevl":
		return "previous-layout"
	case "swapp":
		return "swap-pane"
	case "rotatew":
		return "rotate-window"
	case "breakp":
		return "break-pane"
	case "joinp":
		return "join-pane"
	case "movep":
		return "move-pane"
	case "capturep":
		return "capture-pane"
	case "clearphist":
		return "clear-prompt-history"
	case "clearhist":
		return "clear-history"
	case "setb":
		return "set-buffer"
	case "showb":
		return "show-buffer"
	case "lsb":
		return "list-buffers"
	case "deleteb":
		return "delete-buffer"
	case "pasteb":
		return "paste-buffer"
	case "pipep":
		return "pipe-pane"
	case "loadb":
		return "load-buffer"
	case "saveb":
		return "save-buffer"
	case "lock":
		return "lock-server"
	case "locks":
		return "lock-session"
	case "lockc":
		return "lock-client"
	case "killp":
		return "kill-pane"
	case "killw":
		return "kill-window"
	case "linkw":
		return "link-window"
	case "unlinkw":
		return "unlink-window"
	case "rename":
		return "rename-session"
	case "renamew":
		return "rename-window"
	case "swapw":
		return "swap-window"
	case "switchc":
		return "switch-client"
	case "movew":
		return "move-window"
	case "source":
		return "source-file"
	case "set":
		return "set-option"
	case "setw":
		return "set-window-option"
	case "show":
		return "show-options"
	case "showphist":
		return "show-prompt-history"
	case "showw":
		return "show-window-options"
	case "bind":
		return "bind-key"
	case "unbind":
		return "unbind-key"
	case "lsk":
		return "list-keys"
	case "setenv":
		return "set-environment"
	case "showenv":
		return "show-environment"
	case "showmsgs":
		return "show-messages"
	case "send":
		return "send-keys"
	case "resizep":
		return "resize-pane"
	case "resizew":
		return "resize-window"
	case "respawnp":
		return "respawn-pane"
	case "respawnw":
		return "respawn-window"
	case "selectl":
		return "select-layout"
	case "run":
		return "run-shell"
	case "start":
		return "start-server"
	case "suspendc":
		return "suspend-client"
	case "wait":
		return "wait-for"
	case "kill-server", "kill-session", "link-window", "lock-server", "lock-session", "lock-client", "refresh-client", "rename-session", "rename-window", "swap-window", "switch-client", "move-window", "unlink-window",
		"send-keys", "send-prefix", "server-access", "confirm-before", "display-message", "display-menu", "display-panes", "display-popup", "find-window", "capture-pane", "clear-history", "clear-prompt-history", "detach-client", "suspend-client", "version",
		"source-file", "set-hook", "set-option", "set-window-option", "show-hooks", "show-options", "show-window-options",
		"bind-key", "unbind-key", "list-keys", "set-environment",
		"show-environment", "show-messages", "if-shell", "resize-pane", "resize-window", "respawn-pane", "respawn-window", "last-window", "last-pane", "next-layout", "previous-layout", "select-layout",
		"swap-pane", "rotate-window", "run-shell", "break-pane", "join-pane", "move-pane",
		"set-buffer", "show-buffer", "list-buffers", "delete-buffer",
		"paste-buffer", "pipe-pane", "load-buffer", "save-buffer", "list-clients", "list-commands", "show-prompt-history", "start-server", "wait-for", "new-pane",
		"clock-mode", "copy-mode", "choose-buffer", "choose-client", "choose-tree", "customize-mode", "command-prompt":
		return name
	default:
		return name
	}
}

func findCommandInfo(query string) (commandInfo, error) {
	normalized := normalizeCommandName(query)
	for _, info := range commandInfos {
		if info.Name == normalized || info.Alias == query {
			return info, nil
		}
	}
	var matches []commandInfo
	for _, info := range commandInfos {
		if strings.HasPrefix(info.Name, query) {
			matches = append(matches, info)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return commandInfo{}, fmt.Errorf("unknown command: %s", query)
	}
	names := make([]string, 0, len(matches))
	for _, info := range matches {
		names = append(names, info.Name)
	}
	return commandInfo{}, fmt.Errorf("ambiguous command: %s, could be: %s", query, strings.Join(names, ", "))
}

func ok(text string) protocol.Message {
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text}
}

func status(text string) protocol.Message {
	return protocol.Message{Type: protocol.TypeResult, OK: true, StatusText: text}
}

func fail(text string) protocol.Message {
	return protocol.Message{Type: protocol.TypeError, OK: false, Text: text, Code: 1}
}

func optionValue(args []string, name string, fallback string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, name) && len(arg) > len(name) && len(name) == 2 {
			return arg[len(name):]
		}
	}
	return fallback
}

func commandSize(args []string, width, height int) (int, int) {
	if value := optionValue(args, "-x", ""); value != "" && value != "-" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			width = parsed
		}
	}
	if value := optionValue(args, "-y", ""); value != "" && value != "-" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			height = parsed
		}
	}
	return width, height
}

func environmentOverrides(args []string) map[string]string {
	overrides := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := ""
		switch {
		case arg == "-e" && i+1 < len(args):
			value = args[i+1]
			i++
		case strings.HasPrefix(arg, "-e") && len(arg) > 2:
			value = arg[2:]
		default:
			continue
		}
		name, envValue, ok := strings.Cut(value, "=")
		if !ok {
			name = value
			envValue = os.Getenv(value)
		}
		if name != "" {
			overrides[name] = envValue
		}
	}
	return overrides
}

func sizeOption(args []string, name string, total int, fallback int) int {
	value := optionValue(args, name, "")
	if value == "" || value == "-" {
		return fallback
	}
	if strings.HasSuffix(value, "%") {
		percent, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		if err == nil && percent > 0 {
			return max(1, total*percent/100)
		}
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func positionOption(args []string, name string, total int, fallback int) int {
	value := optionValue(args, name, "")
	if value == "" || value == "-" {
		return fallback
	}
	if strings.HasSuffix(value, "%") {
		percent, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		if err == nil && percent >= 0 {
			return max(0, total*percent/100)
		}
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func positiveOption(args []string, name string, label string) (int, error) {
	value := optionValue(args, name, "")
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s invalid", label)
	}
	return parsed, nil
}

func targetSession(args []string, currentSession string) string {
	return cleanSessionTarget(optionValue(args, "-t", currentSession))
}

func (rt *Runtime) windowSessionTarget(args []string, currentSession string) string {
	target := cleanSessionTarget(optionValue(args, "-t", currentSession))
	if target == "" {
		if currentSession != "" {
			return currentSession
		}
		return firstSessionName(rt.state)
	}
	if strings.Contains(target, ":") {
		session, _, _, _, _ := parsePaneTarget(target)
		if session != "" {
			return session
		}
		if currentSession != "" {
			return currentSession
		}
		return firstSessionName(rt.state)
	}
	return target
}

func hasAny(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name {
				return true
			}
			if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], strings.TrimPrefix(name, "-")) {
				return true
			}
		}
	}
	return false
}

func selectPaneDirection(args []string) string {
	switch {
	case hasAny(args, "-L"):
		return "L"
	case hasAny(args, "-R"):
		return "R"
	case hasAny(args, "-U"):
		return "U"
	case hasAny(args, "-D"):
		return "D"
	default:
		return ""
	}
}

func nonOptionArgs(args []string) []string {
	out := make([]string, 0, len(args))
	skip := false
	for i, arg := range args {
		if skip {
			skip = false
			continue
		}
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				skip = true
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func optionOperands(args []string) []string {
	valueFlags := map[string]bool{
		"-b": true, "-c": true, "-d": true, "-E": true, "-e": true, "-F": true, "-f": true,
		"-N": true, "-S": true, "-T": true, "-t": true, "-x": true,
		"-y": true,
	}
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if valueFlags[arg] && i+1 < len(args) {
				i++
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func lastNonOption(args []string) string {
	values := nonOptionArgs(args)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func trailingCommand(args []string, optionsWithValues map[string]bool) []string {
	_, commandStart := splitCommandArgs(args, optionsWithValues)
	if commandStart < 0 {
		return nil
	}
	return append([]string(nil), args[commandStart:]...)
}

func commandOptionArgs(args []string, optionsWithValues map[string]bool) []string {
	optionsEnd, _ := splitCommandArgs(args, optionsWithValues)
	return args[:optionsEnd]
}

func splitCommandArgs(args []string, optionsWithValues map[string]bool) (int, int) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return i, i + 1
		}
		if strings.HasPrefix(arg, "-") {
			if optionsWithValues[arg] && i+1 < len(args) {
				i++
			}
			continue
		}
		return i, i
	}
	return len(args), -1
}
