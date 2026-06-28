package server

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/command"
	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

func (rt *Runtime) executeMessage(msg protocol.Message, currentSession string) protocol.Message {
	commands := msg.Commands
	var err error
	if len(commands) == 0 {
		commands, err = command.ParseArgv(msg.Command)
		if err != nil {
			return fail(err.Error())
		}
	}
	return rt.executeCommands(commands, currentSession, msg.Width, msg.Height)
}

func (rt *Runtime) executeCommands(commands [][]string, currentSession string, width, height int) protocol.Message {
	var texts []string
	last := ok("")
	activeSession := currentSession
	for _, argv := range commands {
		if len(argv) == 0 {
			continue
		}
		last = rt.execute(argv, activeSession, width, height)
		if last.Session != "" {
			activeSession = last.Session
		} else if activeSession == "" {
			activeSession = firstSessionName(rt.state)
		}
		if last.Text != "" {
			texts = append(texts, last.Text)
		}
		if !last.OK {
			break
		}
	}
	if len(texts) > 0 {
		last.Text = strings.Join(texts, "\n")
	}
	if last.Session == "" {
		last.Session = activeSession
	}
	return last
}

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
		return ok(listSessionsFormat(rt.state, optionValue(args, "-F", "")))
	case "list-windows":
		return ok(listWindowsFormat(rt.state, targetSession(args, currentSession), optionValue(args, "-F", "")))
	case "list-panes":
		return ok(listPanesFormat(rt.state, targetSession(args, currentSession), optionValue(args, "-F", "")))
	case "new-window":
		return rt.cmdNewWindow(args, currentSession, width, height)
	case "split-window":
		return rt.cmdSplitWindow(args, currentSession, width, height)
	case "source-file":
		return rt.cmdSourceFile(args, currentSession, width, height)
	case "set-option":
		return rt.cmdSetOption(args, currentSession, "session")
	case "set-window-option":
		return rt.cmdSetOption(args, currentSession, "window")
	case "show-options":
		return rt.cmdShowOptions(args, currentSession)
	case "bind-key":
		return rt.cmdBindKey(args)
	case "unbind-key":
		return rt.cmdUnbindKey(args)
	case "list-keys":
		return rt.cmdListKeys(args)
	case "set-environment":
		return rt.cmdSetEnvironment(args, currentSession)
	case "show-environment":
		return rt.cmdShowEnvironment(args, currentSession)
	case "send-prefix":
		return rt.cmdSendPrefix(args, currentSession)
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
	case "resize-pane":
		return rt.cmdResizePane(args, currentSession)
	case "select-layout":
		return rt.cmdSelectLayout(args, currentSession)
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
		return rt.cmdDisplayMessage(args, currentSession)
	case "capture-pane":
		return rt.cmdCapturePane(args, currentSession)
	case "set-buffer":
		return rt.cmdSetBuffer(args)
	case "show-buffer":
		return rt.cmdShowBuffer(args)
	case "list-buffers":
		return rt.cmdListBuffers(args)
	case "delete-buffer":
		return rt.cmdDeleteBuffer(args)
	case "paste-buffer":
		return rt.cmdPasteBuffer(args, currentSession)
	case "load-buffer":
		return rt.cmdLoadBuffer(args)
	case "save-buffer":
		return rt.cmdSaveBuffer(args)
	case "detach-client":
		return protocol.Message{Type: protocol.TypeExit, OK: true, Text: "detached"}
	case "version":
		return ok("gotmux 0.1.0")
	default:
		return fail(fmt.Sprintf("unknown command: %s", argv[0]))
	}
}

func (rt *Runtime) cmdNewSession(args []string, width, height int) protocol.Message {
	width, height = commandSize(args, width, height)
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
	rt.state.SetActiveWindowSize(session.Name, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: session.Name, Session: session.Name}
}

func (rt *Runtime) cmdNewWindow(args []string, currentSession string, width, height int) protocol.Message {
	if target := targetSession(args, currentSession); target != "" {
		currentSession = target
	}
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
	rt.state.SetActiveWindowSize(currentSession, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	return ok(fmt.Sprintf("%d:%s", window.Index, window.Name))
}

func (rt *Runtime) cmdSplitWindow(args []string, currentSession string, width, height int) protocol.Message {
	if target := targetSession(args, currentSession); target != "" {
		currentSession = target
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	cwd := optionValue(args, "-c", "")
	command := trailingCommand(args, map[string]bool{"-c": true, "-t": true, "-l": true, "-p": true})
	rt.state.SetActiveWindowSize(currentSession, width, height)
	orientation := "vertical"
	if hasAny(args, "-h") {
		orientation = "horizontal"
	}
	pane, err := rt.state.SplitPaneWithLayout(currentSession, cwd, command, orientation)
	if err != nil {
		return fail(err.Error())
	}
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	rt.resizeSessionPanes(currentSession)
	return ok(fmt.Sprintf("pane %d", pane.Index))
}

func (rt *Runtime) cmdResizePane(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	direction := "R"
	switch {
	case hasAny(args, "-L"):
		direction = "L"
	case hasAny(args, "-R"):
		direction = "R"
	case hasAny(args, "-U"):
		direction = "U"
	case hasAny(args, "-D"):
		direction = "D"
	}
	amount := 1
	for _, value := range optionOperands(args) {
		if parsed, err := strconv.Atoi(value); err == nil {
			amount = parsed
			break
		}
	}
	if err := rt.state.ResizeActivePane(currentSession, direction, amount); err != nil {
		return fail(err.Error())
	}
	rt.resizeSessionPanes(currentSession)
	return ok("")
}

func (rt *Runtime) cmdSelectLayout(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	layout := "even-horizontal"
	values := optionOperands(args)
	if len(values) > 0 {
		layout = values[len(values)-1]
	}
	switch layout {
	case "even-horizontal", "even-vertical":
		if err := rt.state.SelectEvenLayout(currentSession, layout); err != nil {
			return fail(err.Error())
		}
		rt.resizeSessionPanes(currentSession)
		return ok("")
	default:
		return fail(fmt.Sprintf("unsupported layout: %s", layout))
	}
}

func (rt *Runtime) cmdDisplayMessage(args []string, currentSession string) protocol.Message {
	template := optionValue(args, "-F", "")
	if template == "" {
		values := nonOptionArgs(args)
		if len(values) > 0 {
			template = strings.Join(values, " ")
		}
	}
	if template == "" {
		template = "#{session_name}: #{window_index}:#{window_name}, current pane #{pane_index}"
	}
	return ok(formatString(template, activeFormatContext(rt.state, currentSession)))
}

func (rt *Runtime) cmdCapturePane(args []string, currentSession string) protocol.Message {
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	if hasAny(args, "-J") {
		rows := rt.capturePaneRows(pane, true)
		rows = sliceCaptureRows(rows, optionValue(args, "-S", ""), optionValue(args, "-E", ""))
		text := joinCaptureRows(rows)
		if !hasAny(args, "-p") {
			if len(rows) == 0 || !rows[len(rows)-1].Wrapped {
				text += "\n"
			}
			rt.state.SetBuffer(optionValue(args, "-b", ""), text, hasAny(args, "-a"))
			return ok("")
		}
		return ok(text)
	}
	lines := rt.capturePaneLines(pane, !hasAny(args, "-N"))
	lines = sliceCaptureLines(lines, optionValue(args, "-S", ""), optionValue(args, "-E", ""))
	text := strings.Join(lines, "\n")
	if !hasAny(args, "-p") {
		rt.state.SetBuffer(optionValue(args, "-b", ""), text+"\n", hasAny(args, "-a"))
		return ok("")
	}
	return ok(text)
}

func (rt *Runtime) cmdSetBuffer(args []string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing buffer data")
	}
	rt.state.SetBuffer(optionValue(args, "-b", ""), strings.Join(values, " "), hasAny(args, "-a"))
	return ok("")
}

func (rt *Runtime) cmdShowBuffer(args []string) protocol.Message {
	data, err := rt.state.ShowBuffer(optionValue(args, "-b", ""))
	if err != nil {
		return fail(err.Error())
	}
	return ok(data)
}

func (rt *Runtime) cmdListBuffers(args []string) protocol.Message {
	format := optionValue(args, "-F", "")
	buffers := rt.state.ListBuffers()
	lines := make([]string, 0, len(buffers))
	for _, buffer := range buffers {
		if format != "" {
			lines = append(lines, formatBuffer(format, buffer))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %d bytes: %s", buffer.Name, len(buffer.Data), quoteBufferSample(buffer.Data)))
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdDeleteBuffer(args []string) protocol.Message {
	if err := rt.state.DeleteBuffer(optionValue(args, "-b", "")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdPasteBuffer(args []string, currentSession string) protocol.Message {
	data, err := rt.state.ShowBuffer(optionValue(args, "-b", ""))
	if err != nil {
		return fail(err.Error())
	}
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil || pane.PTY == nil {
		return ok("")
	}
	_, _ = pane.PTY.Write([]byte(data))
	if hasAny(args, "-d") {
		_ = rt.state.DeleteBuffer(optionValue(args, "-b", ""))
	}
	return ok("")
}

func (rt *Runtime) cmdLoadBuffer(args []string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing path")
	}
	path := expandPath(values[len(values)-1])
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(fmt.Sprintf("No such file or directory: %s", path))
		}
		return fail(err.Error())
	}
	rt.state.SetBuffer(optionValue(args, "-b", ""), string(data), false)
	return ok("")
}

func (rt *Runtime) cmdSaveBuffer(args []string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing path")
	}
	data, err := rt.state.ShowBuffer(optionValue(args, "-b", ""))
	if err != nil {
		return fail(err.Error())
	}
	path := expandPath(values[len(values)-1])
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if hasAny(args, "-a") {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	file, err := os.OpenFile(path, flag, 0o666)
	if err != nil {
		return fail(err.Error())
	}
	defer file.Close()
	if _, err := file.WriteString(data); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdSourceFile(args []string, currentSession string, width, height int) protocol.Message {
	paths := optionOperands(args)
	if len(paths) == 0 {
		return fail("missing path")
	}
	var texts []string
	last := ok("")
	for _, path := range paths {
		data, err := os.ReadFile(expandPath(path))
		if err != nil {
			if hasAny(args, "-q") {
				continue
			}
			return fail(err.Error())
		}
		commands, err := command.ParseScript(string(data))
		if err != nil {
			return fail(err.Error())
		}
		last = rt.executeCommands(commands, currentSession, width, height)
		if last.Text != "" {
			texts = append(texts, last.Text)
		}
		if !last.OK {
			return last
		}
	}
	last.Text = strings.Join(texts, "\n")
	return last
}

func (rt *Runtime) cmdSetOption(args []string, currentSession string, defaultScope string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing option")
	}
	name := values[0]
	value := ""
	if len(values) > 1 {
		value = strings.Join(values[1:], " ")
	}
	scope := defaultScope
	if hasAny(args, "-g") {
		if defaultScope == "window" {
			scope = "global-window"
		} else {
			scope = "global"
		}
	}
	if hasAny(args, "-w") {
		scope = "window"
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	if err := rt.state.SetOption(scope, currentSession, name, value); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdShowOptions(args []string, currentSession string) protocol.Message {
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	if hasAny(args, "-w") {
		if hasAny(args, "-g") {
			scope = "global-window"
		} else {
			scope = "window"
		}
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	options, err := rt.state.Options(scope, currentSession)
	if err != nil {
		return fail(err.Error())
	}
	names := optionOperands(args)
	valueOnly := hasAny(args, "-v")
	if len(names) > 0 {
		value, exists := options[names[0]]
		if !exists {
			if hasAny(args, "-q") {
				return ok("")
			}
			return fail(fmt.Sprintf("invalid option: %s", names[0]))
		}
		if valueOnly {
			return ok(value)
		}
		return ok(fmt.Sprintf("%s %s", names[0], value))
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		if valueOnly {
			lines = append(lines, options[key])
		} else {
			lines = append(lines, fmt.Sprintf("%s %s", key, options[key]))
		}
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdBindKey(args []string) protocol.Message {
	table := optionValue(args, "-T", "prefix")
	if hasAny(args, "-n") {
		table = "root"
	}
	note := optionValue(args, "-N", "")
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing key")
	}
	key := values[0]
	boundCommand := []string{"send-prefix"}
	if len(values) > 1 {
		boundCommand = values[1:]
	}
	rt.state.BindKey(table, key, boundCommand, note, hasAny(args, "-r"))
	return ok("")
}

func (rt *Runtime) cmdUnbindKey(args []string) protocol.Message {
	table := optionValue(args, "-T", "prefix")
	if hasAny(args, "-n") {
		table = "root"
	}
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing key")
	}
	rt.state.UnbindKey(table, values[0])
	return ok("")
}

func (rt *Runtime) cmdListKeys(args []string) protocol.Message {
	table := optionValue(args, "-T", "")
	format := optionValue(args, "-F", "")
	filterKeys := optionOperands(args)
	bindings := rt.state.ListKeyBindings(table)
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Table == bindings[j].Table {
			return bindings[i].Key < bindings[j].Key
		}
		return bindings[i].Table < bindings[j].Table
	})
	var lines []string
	for _, binding := range bindings {
		if len(filterKeys) > 0 && binding.Key != filterKeys[0] {
			continue
		}
		if format != "" {
			lines = append(lines, formatKeyBinding(format, binding))
		} else {
			lines = append(lines, fmt.Sprintf("bind-key -T %s %s %s", binding.Table, binding.Key, strings.Join(binding.Command, " ")))
		}
		if hasAny(args, "-1") && len(lines) > 0 {
			break
		}
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdSetEnvironment(args []string, currentSession string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing variable")
	}
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	name := values[0]
	if hasAny(args, "-u") {
		if err := rt.state.UnsetEnvironment(scope, currentSession, name); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	value := ""
	if len(values) > 1 {
		value = strings.Join(values[1:], " ")
	}
	if err := rt.state.SetEnvironment(scope, currentSession, name, value); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdShowEnvironment(args []string, currentSession string) protocol.Message {
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	env, err := rt.state.Environment(scope, currentSession)
	if err != nil {
		return fail(err.Error())
	}
	names := optionOperands(args)
	shellFormat := hasAny(args, "-s")
	if len(names) > 0 {
		value, exists := env[names[0]]
		if !exists {
			return fail(fmt.Sprintf("unknown variable: %s", names[0]))
		}
		if shellFormat {
			return ok(fmt.Sprintf("%s=%s; export %s;", names[0], shellQuote(value), names[0]))
		}
		return ok(fmt.Sprintf("%s=%s", names[0], value))
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		if shellFormat {
			lines = append(lines, fmt.Sprintf("%s=%s; export %s;", key, shellQuote(env[key]), key))
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, env[key]))
		}
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdSendPrefix(args []string, currentSession string) protocol.Message {
	pane := rt.state.ActivePane(currentSession)
	if pane == nil || pane.PTY == nil {
		return ok("")
	}
	_, _ = pane.PTY.Write([]byte{rt.prefixByte()})
	return ok("")
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

func shellQuote(value string) string {
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\n\"'\\$`") {
		return "\"" + value + "\""
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
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
	case "capturep":
		return "capture-pane"
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
	case "loadb":
		return "load-buffer"
	case "saveb":
		return "save-buffer"
	case "killp":
		return "kill-pane"
	case "killw":
		return "kill-window"
	case "source":
		return "source-file"
	case "set":
		return "set-option"
	case "setw":
		return "set-window-option"
	case "show":
		return "show-options"
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
	case "resizep":
		return "resize-pane"
	case "selectl":
		return "select-layout"
	case "kill-server", "kill-session", "rename-session", "rename-window",
		"send-keys", "display-message", "capture-pane", "detach-client", "version",
		"source-file", "set-option", "set-window-option", "show-options",
		"bind-key", "unbind-key", "list-keys", "set-environment",
		"show-environment", "send-prefix", "resize-pane", "select-layout",
		"set-buffer", "show-buffer", "list-buffers", "delete-buffer",
		"paste-buffer", "load-buffer", "save-buffer":
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

func targetSession(args []string, currentSession string) string {
	return cleanSessionTarget(optionValue(args, "-t", currentSession))
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

func optionOperands(args []string) []string {
	valueFlags := map[string]bool{
		"-b": true, "-c": true, "-d": true, "-E": true, "-F": true, "-f": true,
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
	format := ""
	return listSessionsFormat(state, format)
}

func listSessionsFormat(state *model.Server, format string) string {
	sessions := snapshotSessions(state)
	if len(sessions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if format != "" {
			lines = append(lines, formatString(format, formatContext{session: session, window: session.ActiveWindow(), pane: activePane(session)}))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %d windows (created %s) [%dx%d]",
				session.Name, len(session.Windows), session.CreatedAt.Format("Mon Jan _2 15:04:05 2006"), activeWidth(session), activeHeight(session)))
		}
	}
	return strings.Join(lines, "\n")
}

func listWindows(state *model.Server, sessionName string) string {
	return listWindowsFormat(state, sessionName, "")
}

func listWindowsFormat(state *model.Server, sessionName string, format string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		lines := make([]string, 0, len(session.Windows))
		for _, window := range session.Windows {
			if format != "" {
				lines = append(lines, formatString(format, formatContext{session: session, window: window, pane: window.ActivePane()}))
			} else {
				mark := ""
				if window.Index == session.Active {
					mark = "*"
				}
				lines = append(lines, fmt.Sprintf("%d: %s%s (%d panes)", window.Index, window.Name, mark, len(window.Panes)))
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func listPanes(state *model.Server, sessionName string) string {
	return listPanesFormat(state, sessionName, "")
}

func listPanesFormat(state *model.Server, sessionName string, format string) string {
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
			if format != "" {
				lines = append(lines, formatString(format, formatContext{session: session, window: window, pane: pane}))
			} else {
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
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})
	return sessions
}

func activeFormatContext(state *model.Server, sessionName string) formatContext {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		window := session.ActiveWindow()
		var pane *model.Pane
		if window != nil {
			pane = window.ActivePane()
		}
		return formatContext{session: session, window: window, pane: pane}
	}
	return formatContext{}
}

func cleanSessionTarget(target string) string {
	target = strings.TrimPrefix(target, "=")
	target = strings.TrimPrefix(target, "$")
	return target
}

func expandPath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}

func formatKeyBinding(template string, binding model.KeyBinding) string {
	out := template
	replacements := map[string]string{
		"#{key_table}": binding.Table,
		"#{key}":       binding.Key,
		"#{command}":   strings.Join(binding.Command, " "),
		"#{note}":      binding.Note,
	}
	for old, newValue := range replacements {
		out = strings.ReplaceAll(out, old, newValue)
	}
	return out
}

func formatBuffer(template string, buffer model.Buffer) string {
	out := template
	replacements := map[string]string{
		"#{buffer_name}":   buffer.Name,
		"#{buffer_size}":   strconv.Itoa(len(buffer.Data)),
		"#{buffer_sample}": bufferSample(buffer.Data),
	}
	for old, newValue := range replacements {
		out = strings.ReplaceAll(out, old, newValue)
	}
	return out
}

func bufferSample(data string) string {
	data = strings.ReplaceAll(data, "\\", "\\\\")
	data = strings.ReplaceAll(data, "\r", "\\r")
	data = strings.ReplaceAll(data, "\n", "\\n")
	if len(data) > 50 {
		data = data[:50]
	}
	return data
}

func quoteBufferSample(data string) string {
	return `"` + strings.ReplaceAll(bufferSample(data), `"`, `\"`) + `"`
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

func (rt *Runtime) targetPane(target string, currentSession string) *model.Pane {
	sessionName, windowIndex, paneIndex, hasWindow, hasPane := parsePaneTarget(target)
	if sessionName == "" {
		sessionName = currentSession
	}
	if sessionName == "" {
		sessionName = firstSessionName(rt.state)
	}
	for _, session := range snapshotSessions(rt.state) {
		if session.Name != sessionName {
			continue
		}
		window := session.ActiveWindow()
		if hasWindow {
			window = nil
			for _, candidate := range session.Windows {
				if candidate.Index == windowIndex {
					window = candidate
					break
				}
			}
		}
		if window == nil {
			return nil
		}
		if hasPane {
			for _, pane := range window.Panes {
				if pane.Index == paneIndex {
					return pane
				}
			}
			return nil
		}
		return window.ActivePane()
	}
	return nil
}

func parsePaneTarget(target string) (session string, window int, pane int, hasWindow bool, hasPane bool) {
	target = cleanSessionTarget(target)
	if target == "" {
		return "", 0, 0, false, false
	}
	if strings.HasPrefix(target, ":") {
		target = target[1:]
	} else if before, after, ok := strings.Cut(target, ":"); ok {
		session = before
		target = after
	} else if !strings.Contains(target, ".") {
		session = target
		return session, 0, 0, false, false
	}
	if before, after, ok := strings.Cut(target, "."); ok {
		if before != "" {
			if parsed, err := strconv.Atoi(strings.TrimPrefix(before, "=")); err == nil {
				window = parsed
				hasWindow = true
			}
		}
		if after != "" {
			if parsed, err := strconv.Atoi(strings.TrimPrefix(after, "=")); err == nil {
				pane = parsed
				hasPane = true
			}
		}
		return session, window, pane, hasWindow, hasPane
	}
	if target != "" {
		if parsed, err := strconv.Atoi(strings.TrimPrefix(target, "=")); err == nil {
			window = parsed
			hasWindow = true
		}
	}
	return session, window, pane, hasWindow, hasPane
}

func (rt *Runtime) capturePaneLines(pane *model.Pane, trimTrailing bool) []string {
	rows := rt.capturePaneRows(pane, !trimTrailing)
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.Text
	}
	return lines
}

func (rt *Runtime) capturePaneRows(pane *model.Pane, preserveTrailing bool) []terminal.CaptureRow {
	var rows []terminal.CaptureRow
	rt.screensMu.RLock()
	screen := rt.screens[pane.ID]
	rt.screensMu.RUnlock()
	if screen != nil {
		rows = screen.CaptureRows(preserveTrailing)
	} else {
		lines := visibleTextLines(pane.History.Bytes(), pane.Height)
		if pane.Height > 0 && len(lines) < pane.Height {
			lines = append(lines, make([]string, pane.Height-len(lines))...)
		}
		rows = make([]terminal.CaptureRow, len(lines))
		for i, line := range lines {
			if !preserveTrailing {
				line = strings.TrimRight(line, " ")
			}
			rows[i] = terminal.CaptureRow{Text: line}
		}
	}
	return rows
}

func sliceCaptureLines(lines []string, startValue string, endValue string) []string {
	if len(lines) == 0 {
		return nil
	}
	start := 0
	end := len(lines) - 1
	if startValue != "" {
		start = parseCaptureLineIndex(startValue, len(lines), 0)
	}
	if endValue != "" {
		end = parseCaptureLineIndex(endValue, len(lines), end)
	}
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if end < start {
		return nil
	}
	return lines[start : end+1]
}

func sliceCaptureRows(rows []terminal.CaptureRow, startValue string, endValue string) []terminal.CaptureRow {
	if len(rows) == 0 {
		return nil
	}
	start := 0
	end := len(rows) - 1
	if startValue != "" {
		start = parseCaptureLineIndex(startValue, len(rows), 0)
	}
	if endValue != "" {
		end = parseCaptureLineIndex(endValue, len(rows), end)
	}
	if start < 0 {
		start = 0
	}
	if end >= len(rows) {
		end = len(rows) - 1
	}
	if end < start {
		return nil
	}
	return rows[start : end+1]
}

func joinCaptureRows(rows []terminal.CaptureRow) string {
	var b strings.Builder
	for i, row := range rows {
		b.WriteString(row.Text)
		if i < len(rows)-1 && !row.Wrapped {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func parseCaptureLineIndex(value string, lineCount int, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < 0 {
		return lineCount + parsed
	}
	return parsed
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

func parseWindowTarget(s string) (int, bool) {
	if len(s) > 0 && s[0] == ':' {
		s = s[1:]
	}
	if len(s) > 0 && s[0] == '=' {
		s = s[1:]
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
