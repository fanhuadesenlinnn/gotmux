package server

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/command"
	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdDisplayMessage(args []string, currentSession string) protocol.Message {
	template := optionValue(args, "-F", "")
	if template == "" {
		values := displayMessageOperands(args)
		if len(values) > 0 {
			template = strings.Join(values, " ")
		}
	}
	if template == "" {
		template = "#{session_name}: #{window_index}:#{window_name}, current pane #{pane_index}"
	}
	ctx := activeFormatContext(rt.state, currentSession)
	if target := optionValue(args, "-t", ""); target != "" {
		pane := rt.targetPane(target, currentSession)
		if pane == nil {
			return fail("can't find pane")
		}
		ctx = formatContextForPaneID(rt.state, pane.ID)
	}
	text := formatString(template, ctx)
	if hasAny(args, "-p") {
		return ok(text)
	}
	return status(text)
}

func (rt *Runtime) cmdDisplayPanes(clientID int64) protocol.Message {
	sessionName := rt.state.ActiveSessionName(clientID)
	panes := rt.state.ActiveWindowPanes(sessionName)
	if len(panes) == 0 {
		return ok("")
	}
	labels := make([]string, 0, len(panes))
	for _, pane := range panes {
		labels = append(labels, strconv.Itoa(pane.Index))
	}
	return ok("panes: " + strings.Join(labels, " "))
}

func (rt *Runtime) cmdChooseTree(args []string, currentSession string) protocol.Message {
	sessions, _ := rt.state.Snapshot()
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})
	sessionOnly := hasAny(args, "-s")
	parts := make([]string, 0)
	for _, session := range sessions {
		sessionMark := ""
		if session.Name == currentSession {
			sessionMark = "*"
		}
		if sessionOnly {
			parts = append(parts, session.Name+sessionMark)
			continue
		}
		windows := make([]*model.Window, len(session.Windows))
		copy(windows, session.Windows)
		sort.Slice(windows, func(i, j int) bool {
			return windows[i].Index < windows[j].Index
		})
		if len(windows) == 0 {
			parts = append(parts, session.Name+sessionMark)
			continue
		}
		for _, window := range windows {
			parts = append(parts, fmt.Sprintf("%s:%d:%s%s", session.Name, window.Index, window.Name, windowFlags(session, window)))
		}
	}
	if len(parts) == 0 {
		return status("choose-tree: empty")
	}
	return status("choose-tree: " + strings.Join(parts, " "))
}

func (rt *Runtime) cmdShowMessages(args []string) protocol.Message {
	if hasAny(args, "-J") || hasAny(args, "-T") {
		return ok("")
	}
	messages := rt.state.MessageLog()
	lines := make([]string, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		lines = append(lines, fmt.Sprintf("%s: %s", msg.Time.Format("15:04"), msg.Text))
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdShowPromptHistory(args []string) protocol.Message {
	promptType := optionValue(args, "-T", "")
	text, err := promptHistoryText(promptType)
	if err != nil {
		return fail(err.Error())
	}
	return ok(text)
}

func (rt *Runtime) cmdClearPromptHistory(args []string) protocol.Message {
	promptType := optionValue(args, "-T", "")
	if promptType != "" && !validPromptType(promptType) {
		return fail(fmt.Sprintf("invalid type: %s", promptType))
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

func (rt *Runtime) cmdListCommands(args []string) protocol.Message {
	format := optionValue(args, "-F", "")
	values := optionOperands(args)
	commands := commandInfos
	if len(values) > 0 {
		info, err := findCommandInfo(values[0])
		if err != nil {
			return fail(err.Error())
		}
		commands = []commandInfo{info}
	}
	lines := make([]string, 0, len(commands))
	for _, info := range commands {
		if format != "" {
			lines = append(lines, formatCommand(format, info))
			continue
		}
		line := info.Name
		if info.Alias != "" {
			line += " (" + info.Alias + ")"
		}
		if info.Usage != "" {
			line += " " + info.Usage
		}
		lines = append(lines, line)
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdServerAccess(args []string) protocol.Message {
	if hasAny(args, "-l") {
		entries := rt.state.ListServerAccess()
		lines := make([]string, 0, len(entries))
		for _, entry := range entries {
			access := "R"
			if entry.Write {
				access = "W"
			}
			lines = append(lines, fmt.Sprintf("%s (%s)", entry.Name, access))
		}
		return ok(strings.Join(lines, "\n"))
	}
	values := serverAccessOperands(args)
	if len(values) == 0 {
		return fail("missing user argument")
	}
	if hasAny(args, "-a") && hasAny(args, "-d") {
		return fail("-a and -d cannot be used together")
	}
	if hasAny(args, "-r") && hasAny(args, "-w") {
		return fail("-r and -w cannot be used together")
	}
	if err := rt.state.ChangeServerAccess(values[0], hasAny(args, "-a"), hasAny(args, "-d"), hasAny(args, "-r"), hasAny(args, "-w")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func serverAccessOperands(args []string) []string {
	values := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			values = append(values, args[i+1:]...)
			break
		}
		if arg == "-t" && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			continue
		}
		values = append(values, arg)
	}
	return values
}

func (rt *Runtime) cmdIfShell(args []string, currentSession string, width, height int) protocol.Message {
	values := ifShellOperands(args)
	if len(values) < 2 {
		return fail("not enough arguments")
	}
	shellCommand := values[0]
	ifCommand := values[1]
	elseCommand := ""
	if len(values) > 2 {
		elseCommand = values[2]
	}
	runSelected := func(selected string) protocol.Message {
		if selected == "" {
			return ok("")
		}
		commands, err := command.ParseArgv([]string{selected})
		if err != nil {
			return fail(err.Error())
		}
		return rt.executeCommands(commands, currentSession, width, height)
	}
	if hasAny(args, "-F") {
		if shellCommand != "" && shellCommand != "0" {
			return runSelected(ifCommand)
		}
		return runSelected(elseCommand)
	}
	if hasAny(args, "-b") {
		go func() {
			_, code, err := runShellCommand(shellCommand, "", false)
			if err == nil && code == 0 {
				_ = runSelected(ifCommand)
				return
			}
			_ = runSelected(elseCommand)
		}()
		return ok("")
	}
	_, code, err := runShellCommand(shellCommand, "", false)
	if err == nil && code == 0 {
		return runSelected(ifCommand)
	}
	return runSelected(elseCommand)
}

func (rt *Runtime) cmdWaitFor(args []string) protocol.Message {
	values := waitForOperands(args)
	if len(values) == 0 {
		return fail("missing channel")
	}
	if len(values) > 1 {
		return fail("too many arguments")
	}
	name := values[0]
	switch {
	case hasAny(args, "-S"):
		rt.waitSignal(name)
		return ok("")
	case hasAny(args, "-L"):
		rt.waitLock(name)
		return ok("")
	case hasAny(args, "-U"):
		if !rt.waitUnlock(name) {
			return fail(fmt.Sprintf("channel %s not locked", name))
		}
		return ok("")
	default:
		rt.waitForSignal(name)
		return ok("")
	}
}

func (rt *Runtime) cmdRunShell(args []string, currentSession string, width, height int) protocol.Message {
	valueOptions := map[string]bool{"-c": true, "-d": true, "-t": true}
	optionArgs := commandOptionArgs(args, valueOptions)
	delay, err := runShellDelay(optionArgs)
	if err != nil {
		return fail(err.Error())
	}
	values := trailingCommand(args, valueOptions)
	if len(values) == 0 {
		return ok("")
	}
	shellCommand := strings.Join(values, " ")
	if hasAny(optionArgs, "-C") {
		if delay > 0 {
			time.Sleep(delay)
		}
		commands, err := command.ParseArgv([]string{shellCommand})
		if err != nil {
			return fail(err.Error())
		}
		return rt.executeCommands(commands, currentSession, width, height)
	}
	cwd := expandPath(optionValue(optionArgs, "-c", ""))
	showStderr := hasAny(optionArgs, "-E")
	if hasAny(optionArgs, "-b") {
		go func() {
			if delay > 0 {
				time.Sleep(delay)
			}
			_, _, _ = runShellCommand(shellCommand, cwd, showStderr)
		}()
		return ok("")
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	text, code, err := runShellCommand(shellCommand, cwd, showStderr)
	if err != nil {
		return protocol.Message{Type: protocol.TypeResult, OK: false, Text: text, Code: code}
	}
	return ok(text)
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

func runShellDelay(args []string) (time.Duration, error) {
	value := optionValue(args, "-d", "")
	if value == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid delay time: %s", value)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func runShellOperands(args []string) []string {
	return trailingCommand(args, map[string]bool{"-c": true, "-d": true, "-t": true})
}

func ifShellOperands(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if arg == "-t" && i+1 < len(args) {
				i++
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func displayMessageOperands(args []string) []string {
	valueFlags := map[string]bool{
		"-c": true,
		"-d": true,
		"-F": true,
		"-t": true,
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

func waitForOperands(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func promptHistoryText(promptType string) (string, error) {
	if promptType != "" {
		if !validPromptType(promptType) {
			return "", fmt.Errorf("invalid type: %s", promptType)
		}
		return fmt.Sprintf("History for %s:\n\n", promptType), nil
	}
	var parts []string
	for _, item := range promptTypes() {
		parts = append(parts, fmt.Sprintf("History for %s:\n\n", item))
	}
	return strings.Join(parts, "\n"), nil
}

func validPromptType(promptType string) bool {
	for _, item := range promptTypes() {
		if item == promptType {
			return true
		}
	}
	return false
}

func promptTypes() []string {
	return []string{"command", "search", "target", "window-target"}
}

func runShellCommand(shellCommand string, cwd string, showStderr bool) (string, int, error) {
	cmd := exec.Command("sh", "-c", shellCommand)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var output []byte
	var err error
	if showStderr {
		output, err = cmd.CombinedOutput()
	} else {
		cmd.Stderr = io.Discard
		output, err = cmd.Output()
	}
	text := strings.TrimRight(string(output), "\n")
	if err == nil {
		return text, 0, nil
	}
	code := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	}
	if code != 0 {
		line := fmt.Sprintf("'%s' returned %d", shellCommand, code)
		if text == "" {
			text = line
		} else {
			text += "\n" + line
		}
	}
	return text, code, err
}

func (rt *Runtime) waitChannelLocked(name string) *waitChannel {
	if rt.waitChans == nil {
		rt.waitChans = make(map[string]*waitChannel)
	}
	wc := rt.waitChans[name]
	if wc == nil {
		wc = &waitChannel{}
		rt.waitChans[name] = wc
	}
	return wc
}

func (rt *Runtime) removeWaitChannelLocked(name string, wc *waitChannel) {
	if wc.locked || wc.woken || len(wc.waiters) > 0 || len(wc.lockers) > 0 {
		return
	}
	delete(rt.waitChans, name)
}

func (rt *Runtime) waitSignal(name string) {
	rt.waitMu.Lock()
	wc := rt.waitChannelLocked(name)
	waiters := wc.waiters
	wc.waiters = nil
	if len(waiters) == 0 {
		wc.woken = true
		rt.waitMu.Unlock()
		return
	}
	rt.removeWaitChannelLocked(name, wc)
	rt.waitMu.Unlock()
	for _, waiter := range waiters {
		close(waiter)
	}
}

func (rt *Runtime) waitForSignal(name string) {
	rt.waitMu.Lock()
	wc := rt.waitChannelLocked(name)
	if wc.woken {
		wc.woken = false
		rt.removeWaitChannelLocked(name, wc)
		rt.waitMu.Unlock()
		return
	}
	waiter := make(chan struct{})
	wc.waiters = append(wc.waiters, waiter)
	rt.waitMu.Unlock()
	<-waiter
}

func (rt *Runtime) waitLock(name string) {
	rt.waitMu.Lock()
	wc := rt.waitChannelLocked(name)
	if !wc.locked {
		wc.locked = true
		rt.waitMu.Unlock()
		return
	}
	locker := make(chan struct{})
	wc.lockers = append(wc.lockers, locker)
	rt.waitMu.Unlock()
	<-locker
}

func (rt *Runtime) waitUnlock(name string) bool {
	rt.waitMu.Lock()
	wc := (*waitChannel)(nil)
	if rt.waitChans != nil {
		wc = rt.waitChans[name]
	}
	if wc == nil || !wc.locked {
		rt.waitMu.Unlock()
		return false
	}
	var locker chan struct{}
	if len(wc.lockers) > 0 {
		locker = wc.lockers[0]
		wc.lockers = wc.lockers[1:]
	} else {
		wc.locked = false
		rt.removeWaitChannelLocked(name, wc)
	}
	rt.waitMu.Unlock()
	if locker != nil {
		close(locker)
	}
	return true
}
