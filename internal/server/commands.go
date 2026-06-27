package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) execute(argv []string, currentSession string, width, height int) protocol.Message {
	if len(argv) == 0 {
		argv = []string{"new-session"}
	}
	cmd := normalizeCommandName(argv[0])
	args := argv[1:]

	switch cmd {
	case "new-session":
		return rt.cmdNewSession(args, width, height)
	case "attach-session":
		return ok("attach is handled by the interactive client")
	case "has-session":
		target := optionValue(args, "-t", currentSession)
		if target == "" {
			target = firstSessionName(rt.state)
		}
		if !sessionExists(rt.state, cleanSessionTarget(target)) {
			return fail(fmt.Sprintf("can't find session: %s", target))
		}
		return ok("")
	case "list-sessions":
		return ok(listSessions(rt.state))
	case "list-windows":
		return ok(listWindows(rt.state, currentSession))
	case "list-panes":
		return ok(listPanes(rt.state, currentSession))
	case "new-window":
		return rt.cmdNewWindow(args, currentSession, width, height)
	case "split-window":
		return rt.cmdSplitWindow(args, currentSession, width, height)
	case "select-window":
		target := optionValue(args, "-t", "")
		index, parsed := parseWindowTarget(target)
		if !parsed {
			return fail("bad window target")
		}
		if err := rt.state.SelectWindow(currentSession, index); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "next-window":
		if err := rt.state.SelectRelativeWindow(currentSession, 1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "previous-window":
		if err := rt.state.SelectRelativeWindow(currentSession, -1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "select-pane":
		delta := 1
		target := optionValue(args, "-t", "")
		if strings.Contains(target, ".-") || hasAny(args, "-U", "-L") {
			delta = -1
		}
		if err := rt.state.SelectRelativePane(currentSession, delta); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "kill-pane":
		if err := rt.state.KillActivePane(currentSession); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "kill-window":
		if err := rt.state.KillActiveWindow(currentSession); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "kill-session":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if target == "" {
			return fail("no current session")
		}
		if err := rt.state.KillSession(target); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "kill-server":
		go func() {
			time.Sleep(100 * time.Millisecond)
			os.Exit(0)
		}()
		return ok("server exited")
	case "rename-session":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		name := lastNonOption(args)
		if name == "" {
			return fail("missing session name")
		}
		if err := rt.state.RenameSession(target, name); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "rename-window":
		name := lastNonOption(args)
		if name == "" {
			return fail("missing window name")
		}
		if err := rt.state.RenameWindow(currentSession, name); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "send-keys":
		keys := nonOptionArgs(args)
		rt.sendKeys(currentSession, keys)
		return ok("")
	case "display-message":
		return ok(strings.Join(nonOptionArgs(args), " "))
	case "detach-client":
		return protocol.Message{Type: protocol.TypeExit, OK: true, Text: "detached"}
	case "version":
		return ok("gotmux 0.1.0")
	default:
		return fail(fmt.Sprintf("unknown command: %s", argv[0]))
	}
}

func (rt *Runtime) cmdNewSession(args []string, width, height int) protocol.Message {
	name := optionValue(args, "-s", "")
	windowName := optionValue(args, "-n", "")
	cwd := optionValue(args, "-c", "")
	command := trailingCommand(args, map[string]bool{
		"-s": true, "-n": true, "-c": true, "-t": true, "-x": true, "-y": true,
	})
	if hasAny(args, "-A") && name != "" && sessionExists(rt.state, name) {
		return ok(name)
	}
	session, _, pane, err := rt.state.NewSession(name, cwd, windowName, command)
	if err != nil {
		return fail(err.Error())
	}
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: session.Name, Session: session.Name}
}

func (rt *Runtime) cmdNewWindow(args []string, currentSession string, width, height int) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	name := optionValue(args, "-n", "")
	cwd := optionValue(args, "-c", "")
	command := trailingCommand(args, map[string]bool{"-n": true, "-c": true, "-t": true})
	window, pane, err := rt.state.NewWindow(currentSession, name, cwd, command)
	if err != nil {
		return fail(err.Error())
	}
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	return ok(fmt.Sprintf("%d:%s", window.Index, window.Name))
}

func (rt *Runtime) cmdSplitWindow(args []string, currentSession string, width, height int) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	cwd := optionValue(args, "-c", "")
	command := trailingCommand(args, map[string]bool{"-c": true, "-t": true, "-l": true, "-p": true})
	pane, err := rt.state.SplitPane(currentSession, cwd, command)
	if err != nil {
		return fail(err.Error())
	}
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	return ok(fmt.Sprintf("pane %d", pane.Index))
}

func (rt *Runtime) sendKeys(session string, keys []string) {
	pane := rt.state.ActivePane(session)
	if pane == nil || pane.PTY == nil {
		return
	}
	for i, key := range keys {
		if i > 0 {
			_, _ = pane.PTY.Write([]byte(" "))
		}
		switch key {
		case "Enter", "C-m":
			_, _ = pane.PTY.Write([]byte{'\r'})
		case "Space":
			_, _ = pane.PTY.Write([]byte(" "))
		case "Tab":
			_, _ = pane.PTY.Write([]byte{'\t'})
		case "BSpace", "Backspace":
			_, _ = pane.PTY.Write([]byte{0x7f})
		default:
			_, _ = pane.PTY.Write([]byte(key))
		}
	}
}

func normalizeCommandName(name string) string {
	switch name {
	case "new":
		return "new-session"
	case "attach", "at":
		return "attach-session"
	case "has":
		return "has-session"
	case "ls":
		return "list-sessions"
	case "lsp":
		return "list-panes"
	case "lsw":
		return "list-windows"
	case "neww":
		return "new-window"
	case "splitw":
		return "split-window"
	case "selectw":
		return "select-window"
	case "next":
		return "next-window"
	case "prev":
		return "previous-window"
	case "selectp":
		return "select-pane"
	case "killp":
		return "kill-pane"
	case "killw":
		return "kill-window"
	case "kill-server", "kill-session", "rename-session", "rename-window",
		"send-keys", "display-message", "detach-client", "version":
		return name
	default:
		return name
	}
}

func ok(text string) protocol.Message {
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text}
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

func lastNonOption(args []string) string {
	values := nonOptionArgs(args)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func trailingCommand(args []string, optionsWithValues map[string]bool) []string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return append([]string(nil), args[i+1:]...)
		}
		if strings.HasPrefix(arg, "-") {
			if optionsWithValues[arg] && i+1 < len(args) {
				i++
			}
			continue
		}
		return append([]string(nil), args[i:]...)
	}
	return nil
}

func listSessions(state *model.Server) string {
	sessions, _ := state.Snapshot()
	if len(sessions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(sessions))
	for _, session := range sessions {
		lines = append(lines, fmt.Sprintf("%s: %d windows (created %s) [%dx%d]",
			session.Name, len(session.Windows), session.CreatedAt.Format("Mon Jan _2 15:04:05 2006"), activeWidth(session), activeHeight(session)))
	}
	return strings.Join(lines, "\n")
}

func listWindows(state *model.Server, sessionName string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		lines := make([]string, 0, len(session.Windows))
		for _, window := range session.Windows {
			mark := ""
			if window.Index == session.Active {
				mark = "*"
			}
			lines = append(lines, fmt.Sprintf("%d: %s%s (%d panes)", window.Index, window.Name, mark, len(window.Panes)))
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func listPanes(state *model.Server, sessionName string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		window := session.ActiveWindow()
		if window == nil {
			return ""
		}
		lines := make([]string, 0, len(window.Panes))
		for _, pane := range window.Panes {
			mark := ""
			if pane.Index == window.Active {
				mark = "*"
			}
			state := "running"
			if pane.Exited {
				state = "exited"
			}
			lines = append(lines, fmt.Sprintf("%d:%s [%dx%d] %s %s",
				pane.Index, mark, pane.Width, pane.Height, state, model.CommandString(pane.Command)))
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func sessionExists(state *model.Server, name string) bool {
	for _, session := range snapshotSessions(state) {
		if session.Name == name {
			return true
		}
	}
	return false
}

func firstSessionName(state *model.Server) string {
	for _, session := range snapshotSessions(state) {
		return session.Name
	}
	return ""
}

func snapshotSessions(state *model.Server) []*model.Session {
	sessions, _ := state.Snapshot()
	return sessions
}

func cleanSessionTarget(target string) string {
	target = strings.TrimPrefix(target, "=")
	target = strings.TrimPrefix(target, "$")
	return target
}

func activeWidth(session *model.Session) int {
	if pane := activePane(session); pane != nil && pane.Width > 0 {
		return pane.Width
	}
	return 80
}

func activeHeight(session *model.Session) int {
	if pane := activePane(session); pane != nil && pane.Height > 0 {
		return pane.Height
	}
	return 24
}

func activePane(session *model.Session) *model.Pane {
	if session == nil {
		return nil
	}
	window := session.ActiveWindow()
	if window == nil {
		return nil
	}
	return window.ActivePane()
}

func parsePaneDelta(target string) (int, bool) {
	if strings.HasSuffix(target, ".+") {
		return 1, true
	}
	if strings.HasSuffix(target, ".-") {
		return -1, true
	}
	n, err := strconv.Atoi(target)
	if err != nil {
		return 0, false
	}
	return n, true
}
