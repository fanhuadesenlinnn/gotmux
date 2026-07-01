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

type commandInfo struct {
	Name  string
	Alias string
	Usage string
}

type waitChannel struct {
	woken   bool
	locked  bool
	waiters []chan struct{}
	lockers []chan struct{}
}

var commandInfos = []commandInfo{
	{Name: "attach-session", Alias: "attach", Usage: "[-dErx] [-c working-directory] [-f flags] [-t target-session]"},
	{Name: "bind-key", Alias: "bind", Usage: "[-nr] [-N note] [-T key-table] key [command [arguments]]"},
	{Name: "break-pane", Alias: "breakp", Usage: "[-abdP] [-F format] [-n window-name] [-s src-pane] [-t dst-window]"},
	{Name: "capture-pane", Alias: "capturep", Usage: "[-aCeJNpPqT] [-b buffer-name] [-E end-line] [-S start-line] [-t target-pane]"},
	{Name: "clear-prompt-history", Alias: "clearphist", Usage: "[-T prompt-type]"},
	{Name: "clear-history", Alias: "clearhist", Usage: "[-H] [-t target-pane]"},
	{Name: "delete-buffer", Alias: "deleteb", Usage: "[-b buffer-name]"},
	{Name: "detach-client", Alias: "detach", Usage: "[-aP] [-E shell-command] [-s target-session] [-t target-client]"},
	{Name: "display-message", Alias: "display", Usage: "[-aCIlNpv] [-c target-client] [-d delay] [-F format] [-t target-pane] [message]"},
	{Name: "has-session", Alias: "has", Usage: "[-t target-session]"},
	{Name: "if-shell", Alias: "if", Usage: "[-bF] [-t target-pane] shell-command command [command]"},
	{Name: "join-pane", Alias: "joinp", Usage: "[-bdfhv] [-l size] [-s src-pane] [-t dst-pane]"},
	{Name: "kill-pane", Alias: "killp", Usage: "[-a] [-t target-pane]"},
	{Name: "kill-server"},
	{Name: "kill-session", Usage: "[-aC] [-t target-session]"},
	{Name: "kill-window", Alias: "killw", Usage: "[-a] [-t target-window]"},
	{Name: "last-pane", Alias: "lastp", Usage: "[-deZ] [-t target-window]"},
	{Name: "last-window", Alias: "last", Usage: "[-t target-session]"},
	{Name: "list-buffers", Alias: "lsb", Usage: "[-F format]"},
	{Name: "list-clients", Alias: "lsc", Usage: "[-F format] [-f filter] [-O order][-t target-session]"},
	{Name: "list-commands", Alias: "lscm", Usage: "[-F format] [command]"},
	{Name: "list-keys", Alias: "lsk", Usage: "[-1aN] [-F format] [-P prefix-string] [-T key-table] [key]"},
	{Name: "list-panes", Alias: "lsp", Usage: "[-as] [-F format] [-f filter] [-t target]"},
	{Name: "list-sessions", Alias: "ls", Usage: "[-F format] [-f filter]"},
	{Name: "list-windows", Alias: "lsw", Usage: "[-ar] [-F format] [-f filter] [-O order][-t target-session]"},
	{Name: "load-buffer", Alias: "loadb", Usage: "[-b buffer-name] path"},
	{Name: "move-pane", Alias: "movep", Usage: "[-bdfhv] [-l size] [-s src-pane] [-t dst-pane]"},
	{Name: "move-window", Alias: "movew", Usage: "[-abdk] [-s src-window] [-t dst-window]"},
	{Name: "new-session", Alias: "new", Usage: "[-AdDEPX] [-c start-directory] [-e environment] [-F format] [-f flags] [-n window-name] [-s session-name] [-t target-session] [-x width] [-y height] [shell-command [argument ...]]"},
	{Name: "new-window", Alias: "neww", Usage: "[-abdkPS] [-c start-directory] [-e environment] [-F format] [-n window-name] [-t target-window] [shell-command [argument ...]]"},
	{Name: "next-layout", Alias: "nextl", Usage: "[-t target-window]"},
	{Name: "next-window", Alias: "next", Usage: "[-a] [-t target-session]"},
	{Name: "paste-buffer", Alias: "pasteb", Usage: "[-dpr] [-b buffer-name] [-s separator] [-t target-pane]"},
	{Name: "previous-layout", Alias: "prevl", Usage: "[-t target-window]"},
	{Name: "previous-window", Alias: "prev", Usage: "[-a] [-t target-session]"},
	{Name: "rename-session", Alias: "rename", Usage: "[-t target-session] new-name"},
	{Name: "rename-window", Alias: "renamew", Usage: "[-t target-window] new-name"},
	{Name: "resize-pane", Alias: "resizep", Usage: "[-DLMRTUZ] [-x width] [-y height] [-t target-pane] [adjustment]"},
	{Name: "resize-window", Alias: "resizew", Usage: "[-aADLRU] [-x width] [-y height] [-t target-window] [adjustment]"},
	{Name: "respawn-pane", Alias: "respawnp", Usage: "[-k] [-c start-directory] [-e environment] [-t target-pane] [shell-command [argument ...]]"},
	{Name: "respawn-window", Alias: "respawnw", Usage: "[-k] [-c start-directory] [-e environment] [-t target-window] [shell-command [argument ...]]"},
	{Name: "rotate-window", Alias: "rotatew", Usage: "[-DUZ] [-t target-window]"},
	{Name: "run-shell", Alias: "run", Usage: "[-bCE] [-c start-directory] [-d delay] [-t target-pane] [shell-command [argument ...]]"},
	{Name: "save-buffer", Alias: "saveb", Usage: "[-a] [-b buffer-name] path"},
	{Name: "select-layout", Alias: "selectl", Usage: "[-Enop] [-t target-window] [layout-name]"},
	{Name: "select-pane", Alias: "selectp", Usage: "[-DdeLlMmRUZ] [-T title] [-t target-pane]"},
	{Name: "select-window", Alias: "selectw", Usage: "[-lnpT] [-t target-window]"},
	{Name: "send-keys", Alias: "send", Usage: "[-FHKlMRX] [-N repeat-count] [-t target-pane] key ..."},
	{Name: "send-prefix", Usage: "[-2] [-t target-pane]"},
	{Name: "set-buffer", Alias: "setb", Usage: "[-aw] [-b buffer-name] [-n new-buffer-name] [-t target-client] [data]"},
	{Name: "set-environment", Alias: "setenv", Usage: "[-Fhgru] [-t target-session] name [value]"},
	{Name: "set-option", Alias: "set", Usage: "[-aFgopqsuUw] [-t target-pane] option [value]"},
	{Name: "set-window-option", Alias: "setw", Usage: "[-aFgoqu] [-t target-window] option [value]"},
	{Name: "show-buffer", Alias: "showb", Usage: "[-b buffer-name]"},
	{Name: "show-environment", Alias: "showenv", Usage: "[-hgs] [-t target-session] [name]"},
	{Name: "show-options", Alias: "show", Usage: "[-AgHpqsvw] [-t target-pane] [option]"},
	{Name: "show-prompt-history", Alias: "showphist", Usage: "[-T prompt-type]"},
	{Name: "show-window-options", Alias: "showw", Usage: "[-gv] [-t target-window] [option]"},
	{Name: "source-file", Alias: "source", Usage: "[-Fnqv] path ..."},
	{Name: "split-window", Alias: "splitw", Usage: "[-bdfhIvPZ] [-c start-directory] [-e environment] [-F format] [-l size] [-t target-pane] [shell-command [argument ...]]"},
	{Name: "start-server", Alias: "start"},
	{Name: "swap-pane", Alias: "swapp", Usage: "[-dDUZ] [-s src-pane] [-t dst-pane]"},
	{Name: "swap-window", Alias: "swapw", Usage: "[-d] [-s src-window] [-t dst-window]"},
	{Name: "unlink-window", Alias: "unlinkw", Usage: "[-k] [-t target-window]"},
	{Name: "unbind-key", Alias: "unbind", Usage: "[-anq] [-T key-table] key"},
	{Name: "wait-for", Alias: "wait", Usage: "[-L|-S|-U] channel"},
}

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
		return ok(listWindowsCommand(rt.state, args, currentSession))
	case "list-panes":
		return ok(listPanesCommand(rt.state, args, currentSession))
	case "list-clients":
		return rt.cmdListClients(args)
	case "list-commands":
		return rt.cmdListCommands(args)
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
	case "show-window-options":
		windowArgs := append([]string{"-w"}, args...)
		return rt.cmdShowOptions(windowArgs, currentSession)
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
	case "if-shell":
		return rt.cmdIfShell(args, currentSession, width, height)
	case "wait-for":
		return rt.cmdWaitFor(args)
	case "send-prefix":
		return rt.cmdSendPrefix(args, currentSession)
	case "select-window":
		if hasAny(args, "-n") || hasAny(args, "-p") {
			target := rt.windowSessionTarget(args, currentSession)
			delta := 1
			if hasAny(args, "-p") {
				delta = -1
			}
			if err := rt.state.SelectRelativeWindow(target, delta); err != nil {
				return fail(err.Error())
			}
			return ok("")
		}
		if hasAny(args, "-l") {
			target := rt.windowSessionTarget(args, currentSession)
			if err := rt.state.SelectLastWindow(target); err != nil {
				return fail(err.Error())
			}
			return ok("")
		}
		target := optionValue(args, "-t", "")
		sessionName, index, parsed := selectWindowTarget(target, currentSession)
		if !parsed {
			return fail("bad window target")
		}
		if err := rt.state.SelectWindow(sessionName, index); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "last-window":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if target == "" {
			target = firstSessionName(rt.state)
		}
		if strings.Contains(target, ":") {
			target, _, _, _, _ = parsePaneTarget(target)
		}
		if err := rt.state.SelectLastWindow(target); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "next-window":
		if err := rt.state.SelectRelativeWindow(rt.windowSessionTarget(args, currentSession), 1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "previous-window":
		if err := rt.state.SelectRelativeWindow(rt.windowSessionTarget(args, currentSession), -1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "select-pane":
		if hasAny(args, "-l") {
			return rt.cmdLastPane(args, currentSession)
		}
		if direction := selectPaneDirection(args); direction != "" {
			pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
			if pane == nil {
				return fail("can't find pane")
			}
			if err := rt.state.SelectPaneDirectionFrom(pane.ID, direction); err != nil {
				return fail(err.Error())
			}
			return ok("")
		}
		delta := 1
		target := optionValue(args, "-t", "")
		if target != "" && !strings.Contains(target, ".+") && !strings.Contains(target, ".-") &&
			!hasAny(args, "-U", "-D", "-L", "-R") {
			pane := rt.targetPane(target, currentSession)
			if pane == nil {
				return fail("can't find pane")
			}
			if err := rt.state.SelectPaneByID(pane.ID); err != nil {
				return fail(err.Error())
			}
			return ok("")
		}
		if strings.Contains(target, ".-") || hasAny(args, "-U", "-L") {
			delta = -1
		}
		if err := rt.state.SelectRelativePane(currentSession, delta); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "last-pane":
		return rt.cmdLastPane(args, currentSession)
	case "resize-pane":
		return rt.cmdResizePane(args, currentSession)
	case "resize-window":
		return rt.cmdResizeWindow(args, currentSession)
	case "respawn-pane":
		return rt.cmdRespawnPane(args, currentSession, width, height)
	case "respawn-window":
		return rt.cmdRespawnWindow(args, currentSession, width, height)
	case "next-layout":
		return rt.cmdApplyLayout(args, currentSession, "", "next")
	case "previous-layout":
		return rt.cmdApplyLayout(args, currentSession, "", "previous")
	case "select-layout":
		return rt.cmdSelectLayout(args, currentSession)
	case "swap-pane":
		return rt.cmdSwapPane(args, currentSession)
	case "rotate-window":
		return rt.cmdRotateWindow(args, currentSession)
	case "run-shell":
		return rt.cmdRunShell(args, currentSession, width, height)
	case "break-pane":
		return rt.cmdBreakPane(args, currentSession)
	case "join-pane", "move-pane":
		return rt.cmdJoinPane(args, currentSession)
	case "kill-pane":
		pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
		if pane == nil {
			return fail("can't find pane")
		}
		if hasAny(args, "-a") {
			killed, err := rt.state.KillOtherPanesByID(pane.ID)
			if err != nil {
				return fail(err.Error())
			}
			rt.screensMu.Lock()
			for _, paneID := range killed {
				delete(rt.screens, paneID)
			}
			rt.screensMu.Unlock()
			rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
			return ok("")
		}
		if err := rt.state.KillPaneByID(pane.ID); err != nil {
			return fail(err.Error())
		}
		rt.screensMu.Lock()
		delete(rt.screens, pane.ID)
		rt.screensMu.Unlock()
		rt.resizeSessionPanes(currentSession)
		return ok("")
	case "kill-window":
		return rt.cmdKillWindow(args, currentSession)
	case "unlink-window":
		return rt.cmdUnlinkWindow(args, currentSession)
	case "swap-window":
		return rt.cmdSwapWindow(args, currentSession)
	case "move-window":
		return rt.cmdMoveWindow(args, currentSession)
	case "kill-session":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if target == "" {
			return fail("no current session")
		}
		if err := rt.state.KillSession(target); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "start-server":
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
		sessionName, windowIndex, hasWindow, _, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
		if !found {
			return fail("can't find window")
		}
		var err error
		if hasWindow {
			err = rt.state.RenameWindowByIndex(sessionName, windowIndex, name)
		} else {
			err = rt.state.RenameWindow(sessionName, name)
		}
		if err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "send-keys":
		return rt.cmdSendKeys(args, currentSession)
	case "display-message":
		return rt.cmdDisplayMessage(args, currentSession)
	case "capture-pane":
		return rt.cmdCapturePane(args, currentSession)
	case "clear-history":
		return rt.cmdClearHistory(args, currentSession)
	case "show-prompt-history":
		return rt.cmdShowPromptHistory(args)
	case "clear-prompt-history":
		return rt.cmdClearPromptHistory(args)
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
		"-s": true, "-n": true, "-c": true, "-F": true, "-t": true, "-x": true, "-y": true,
	})
	if hasAny(args, "-A") && name != "" && sessionExists(rt.state, name) {
		text := ""
		if hasAny(args, "-P") {
			text = formatString(optionValue(args, "-F", "#{session_name}:"), activeFormatContext(rt.state, name))
		}
		return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text, Session: name}
	}
	session, _, pane, err := rt.state.NewSession(name, cwd, windowName, command)
	if err != nil {
		return fail(err.Error())
	}
	rt.state.SetActiveWindowSize(session.Name, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	text := ""
	if hasAny(args, "-P") {
		text = formatString(optionValue(args, "-F", "#{session_name}:"), activeFormatContext(rt.state, session.Name))
	}
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text, Session: session.Name}
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
	command := trailingCommand(args, map[string]bool{"-n": true, "-c": true, "-F": true, "-t": true})
	_, pane, err := rt.state.NewWindowDetached(currentSession, name, cwd, command, hasAny(args, "-d"))
	if err != nil {
		return fail(err.Error())
	}
	rt.state.SetActiveWindowSize(currentSession, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	if hasAny(args, "-P") {
		template := optionValue(args, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		return ok(formatString(template, formatContextForPaneID(rt.state, pane.ID)))
	}
	return ok("")
}

func (rt *Runtime) cmdSplitWindow(args []string, currentSession string, width, height int) protocol.Message {
	sessionName, windowIndex, hasWindow, _, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	cwd := optionValue(args, "-c", "")
	command := trailingCommand(args, map[string]bool{"-c": true, "-F": true, "-t": true, "-l": true, "-p": true})
	if !hasWindow {
		rt.state.SetActiveWindowSize(sessionName, width, height)
	}
	orientation := "vertical"
	if hasAny(args, "-h") {
		orientation = "horizontal"
	}
	var pane *model.Pane
	var err error
	if hasWindow {
		pane, err = rt.state.SplitPaneWithLayoutByIndexDetached(sessionName, windowIndex, cwd, command, orientation, hasAny(args, "-d"))
	} else {
		pane, err = rt.state.SplitPaneWithLayoutDetached(sessionName, cwd, command, orientation, hasAny(args, "-d"))
	}
	if err != nil {
		return fail(err.Error())
	}
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	if hasAny(args, "-P") {
		template := optionValue(args, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		return ok(formatString(template, formatContextForPaneID(rt.state, pane.ID)))
	}
	return ok("")
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
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	if err := rt.state.ResizePaneByID(pane.ID, direction, amount); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	return ok("")
}

func (rt *Runtime) cmdResizeWindow(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sessionName, windowIndex, _, paneIDs, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	width, err := positiveOption(args, "-x", "width")
	if err != nil {
		return fail(err.Error())
	}
	height, err := positiveOption(args, "-y", "height")
	if err != nil {
		return fail(err.Error())
	}
	direction := ""
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
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			amount = parsed
			break
		}
	}
	if err := rt.state.ResizeWindowByIndex(sessionName, windowIndex, width, height, direction, amount); err != nil {
		return fail(err.Error())
	}
	if len(paneIDs) > 0 {
		rt.resizePanes(rt.state.WindowPanesContainingPane(paneIDs[0]))
	}
	return ok("")
}

func (rt *Runtime) cmdRespawnPane(args []string, currentSession string, width, height int) protocol.Message {
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	command := trailingCommand(args, map[string]bool{"-c": true, "-e": true, "-t": true})
	respawned, err := rt.state.RespawnPaneByID(pane.ID, optionValue(args, "-c", ""), command, hasAny(args, "-k"))
	if err != nil {
		return fail("respawn pane failed: " + err.Error())
	}
	rt.screensMu.Lock()
	delete(rt.screens, respawned.ID)
	rt.screensMu.Unlock()
	if err := rt.startPane(respawned, width, height); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(respawned.ID))
	return ok("")
}

func (rt *Runtime) cmdRespawnWindow(args []string, currentSession string, width, height int) protocol.Message {
	sessionName, windowIndex, _, _, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	command := trailingCommand(args, map[string]bool{"-c": true, "-e": true, "-t": true})
	pane, killed, err := rt.state.RespawnWindowByIndex(sessionName, windowIndex, optionValue(args, "-c", ""), command, hasAny(args, "-k"))
	if err != nil {
		return fail("respawn window failed: " + err.Error())
	}
	rt.screensMu.Lock()
	delete(rt.screens, pane.ID)
	for _, paneID := range killed {
		delete(rt.screens, paneID)
	}
	rt.screensMu.Unlock()
	rt.state.SetActiveWindowSize(sessionName, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	return ok("")
}

func (rt *Runtime) cmdLastPane(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sessionName, windowIndex, _, _, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	if err := rt.state.SelectLastPaneByIndex(sessionName, windowIndex); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdSelectLayout(args []string, currentSession string) protocol.Message {
	mode := "last"
	values := optionOperands(args)
	layout := ""
	if len(values) > 0 {
		mode = "named"
		layout = values[len(values)-1]
	}
	if hasAny(args, "-n") {
		mode = "next"
	}
	if hasAny(args, "-p") {
		mode = "previous"
	}
	resolvedLayout, supportedLayout := model.ResolveLayoutName(layout)
	if mode == "named" && !supportedLayout {
		return fail(fmt.Sprintf("unsupported layout: %s", layout))
	}
	return rt.cmdApplyLayout(args, currentSession, resolvedLayout, mode)
}

func (rt *Runtime) cmdApplyLayout(args []string, currentSession string, layout string, mode string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sessionName, windowIndex, hasWindow, paneIDs, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	var err error
	switch mode {
	case "named":
		if hasWindow {
			err = rt.state.SelectLayoutByIndex(sessionName, windowIndex, layout)
		} else {
			err = rt.state.SelectLayout(sessionName, layout)
		}
	case "next":
		if hasWindow {
			err = rt.state.SelectNextLayoutByIndex(sessionName, windowIndex)
		} else {
			err = rt.state.SelectNextLayout(sessionName)
		}
	case "previous":
		if hasWindow {
			err = rt.state.SelectPreviousLayoutByIndex(sessionName, windowIndex)
		} else {
			err = rt.state.SelectPreviousLayout(sessionName)
		}
	default:
		if hasWindow {
			err = rt.state.SelectLastLayoutByIndex(sessionName, windowIndex)
		} else {
			err = rt.state.SelectLastLayout(sessionName)
		}
	}
	if err != nil {
		return fail(err.Error())
	}
	if len(paneIDs) > 0 {
		rt.resizePanes(rt.state.WindowPanesContainingPane(paneIDs[0]))
	}
	return ok("")
}

func (rt *Runtime) cmdSwapPane(args []string, currentSession string) protocol.Message {
	target := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if target == nil {
		return fail("can't find pane")
	}
	source := (*model.Pane)(nil)
	switch {
	case hasAny(args, "-U"):
		source = rt.adjacentPane(target.ID, -1)
	case hasAny(args, "-D"):
		source = rt.adjacentPane(target.ID, 1)
	default:
		sourceTarget := optionValue(args, "-s", "")
		if sourceTarget == "" {
			return fail("can't find pane")
		}
		source = rt.targetPane(sourceTarget, currentSession)
	}
	if source == nil {
		return fail("can't find pane")
	}
	sourceID := source.ID
	targetID := target.ID
	if err := rt.state.SwapPanesByID(sourceID, targetID, hasAny(args, "-d")); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(sourceID))
	if sourceID != targetID {
		rt.resizePanes(rt.state.WindowPanesContainingPane(targetID))
	}
	return ok("")
}

func (rt *Runtime) cmdRotateWindow(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sessionName, windowIndex, hasWindow, paneIDs, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	var err error
	if hasWindow {
		err = rt.state.RotateWindowByIndex(sessionName, windowIndex, hasAny(args, "-D"))
	} else {
		err = rt.state.RotateWindow(sessionName, hasAny(args, "-D"))
	}
	if err != nil {
		return fail(err.Error())
	}
	if len(paneIDs) > 0 {
		rt.resizePanes(rt.state.WindowPanesContainingPane(paneIDs[0]))
	}
	return ok("")
}

func (rt *Runtime) cmdBreakPane(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	source := rt.targetPane(optionValue(args, "-s", currentSession), currentSession)
	if source == nil {
		return fail("can't find pane")
	}
	sourceWindowPanes := rt.state.WindowPanesContainingPane(source.ID)
	session, window, pane, err := rt.state.BreakPaneByID(source.ID, optionValue(args, "-n", ""), hasAny(args, "-d"))
	if err != nil {
		return fail(err.Error())
	}
	for _, oldPane := range sourceWindowPanes {
		if oldPane.ID != pane.ID {
			rt.resizePanes(rt.state.WindowPanesContainingPane(oldPane.ID))
			break
		}
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	text := ""
	if hasAny(args, "-P") {
		template := optionValue(args, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		text = formatString(template, formatContext{session: session, window: window, pane: pane})
	}
	return ok(text)
}

func (rt *Runtime) cmdJoinPane(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	source := rt.targetPane(optionValue(args, "-s", ""), currentSession)
	if source == nil {
		return fail("can't find pane")
	}
	target := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if target == nil {
		return fail("can't find pane")
	}
	sourceWindowPanes := rt.state.WindowPanesContainingPane(source.ID)
	orientation := "vertical"
	if hasAny(args, "-h") {
		orientation = "horizontal"
	}
	_, _, pane, err := rt.state.JoinPaneByID(source.ID, target.ID, orientation, hasAny(args, "-d"))
	if err != nil {
		return fail(err.Error())
	}
	for _, oldPane := range sourceWindowPanes {
		if oldPane.ID != pane.ID {
			rt.resizePanes(rt.state.WindowPanesContainingPane(oldPane.ID))
			break
		}
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	return ok("")
}

func (rt *Runtime) cmdSwapWindow(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sourceSession, sourceWindow, _, _, sourceFound := rt.targetWindowInfo(optionValue(args, "-s", currentSession), currentSession)
	if !sourceFound {
		return fail("can't find window")
	}
	targetSession, targetWindow, _, _, targetFound := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !targetFound {
		return fail("can't find window")
	}
	if err := rt.state.SwapWindows(sourceSession, sourceWindow, targetSession, targetWindow, hasAny(args, "-d")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdKillWindow(args []string, currentSession string) protocol.Message {
	sessionName, windowIndex, hasWindow, paneIDs, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	if hasAny(args, "-a") {
		killed, err := rt.state.KillOtherWindows(sessionName, windowIndex)
		if err != nil {
			return fail(err.Error())
		}
		rt.screensMu.Lock()
		for _, paneID := range killed {
			delete(rt.screens, paneID)
		}
		rt.screensMu.Unlock()
		rt.resizeSessionPanes(sessionName)
		return ok("")
	}
	return rt.killTargetWindow(sessionName, windowIndex, hasWindow, paneIDs)
}

func (rt *Runtime) cmdUnlinkWindow(args []string, currentSession string) protocol.Message {
	sessionName, windowIndex, hasWindow, paneIDs, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	if !hasAny(args, "-k") {
		return fail("window only linked to one session")
	}
	return rt.killTargetWindow(sessionName, windowIndex, hasWindow, paneIDs)
}

func (rt *Runtime) killTargetWindow(sessionName string, windowIndex int, hasWindow bool, paneIDs []int) protocol.Message {
	var err error
	if hasWindow {
		err = rt.state.KillWindow(sessionName, windowIndex)
	} else {
		err = rt.state.KillActiveWindow(sessionName)
	}
	if err != nil {
		return fail(err.Error())
	}
	rt.screensMu.Lock()
	for _, paneID := range paneIDs {
		delete(rt.screens, paneID)
	}
	rt.screensMu.Unlock()
	rt.resizeSessionPanes(sessionName)
	return ok("")
}

func (rt *Runtime) cmdMoveWindow(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	if hasAny(args, "-r") {
		targetSessionName := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if targetSessionName == "" {
			targetSessionName = firstSessionName(rt.state)
		}
		if strings.Contains(targetSessionName, ":") {
			targetSessionName, _, _, _, _ = parsePaneTarget(targetSessionName)
		}
		if err := rt.state.RenumberWindows(targetSessionName); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}

	sourceSession, sourceWindow, _, _, sourceFound := rt.targetWindowInfo(optionValue(args, "-s", currentSession), currentSession)
	if !sourceFound {
		return fail("can't find window")
	}
	target := optionValue(args, "-t", "")
	targetSession, targetWindow, hasWindow := moveWindowTarget(target, currentSession)
	if !hasWindow {
		return fail("bad window target")
	}
	if err := rt.state.MoveWindow(sourceSession, sourceWindow, targetSession, targetWindow, hasAny(args, "-d")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

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
	return ok(formatString(template, ctx))
}

func (rt *Runtime) cmdCapturePane(args []string, currentSession string) protocol.Message {
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	joinLines := hasAny(args, "-J")
	includeEmptyCells := !joinLines && !hasAny(args, "-T")
	trimTrailing := !joinLines && !hasAny(args, "-N")
	rows := rt.capturePaneRows(pane, includeEmptyCells, trimTrailing)
	rows = sliceCaptureRows(rows, optionValue(args, "-S", ""), optionValue(args, "-E", ""))
	text := formatCaptureRows(rows, hasAny(args, "-L"), hasAny(args, "-F"), joinLines, hasAny(args, "-C"))
	if !hasAny(args, "-p") {
		if len(rows) == 0 || !joinLines || !rows[len(rows)-1].Wrapped {
			text += "\n"
		}
		rt.state.SetBuffer(optionValue(args, "-b", ""), text, hasAny(args, "-a"))
		return ok("")
	}
	return ok(text)
}

func (rt *Runtime) cmdClearHistory(args []string, currentSession string) protocol.Message {
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	if pane.History != nil {
		pane.History.Reset()
	}
	return ok("")
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

func (rt *Runtime) cmdSetBuffer(args []string) protocol.Message {
	if newName := optionValue(args, "-n", ""); newName != "" {
		if err := rt.state.RenameBuffer(optionValue(args, "-b", ""), newName); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
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
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
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

func (rt *Runtime) cmdListClients(args []string) protocol.Message {
	format := optionValue(args, "-F", "")
	target := cleanSessionTarget(optionValue(args, "-t", ""))
	clients := rt.state.ListClients()
	lines := make([]string, 0, len(clients))
	for _, client := range clients {
		if target != "" && client.SessionName != target {
			continue
		}
		if format != "" {
			lines = append(lines, formatClient(format, client))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s [%dx%d screen-256color]",
			clientName(client), client.SessionName, client.Width, client.Height))
	}
	return ok(strings.Join(lines, "\n"))
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
	delay, err := runShellDelay(args)
	if err != nil {
		return fail(err.Error())
	}
	values := runShellOperands(args)
	if len(values) == 0 {
		return ok("")
	}
	shellCommand := strings.Join(values, " ")
	if hasAny(args, "-C") {
		if delay > 0 {
			time.Sleep(delay)
		}
		commands, err := command.ParseArgv([]string{shellCommand})
		if err != nil {
			return fail(err.Error())
		}
		return rt.executeCommands(commands, currentSession, width, height)
	}
	cwd := expandPath(optionValue(args, "-c", ""))
	showStderr := hasAny(args, "-E")
	if hasAny(args, "-b") {
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
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil || pane.PTY == nil {
		return ok("")
	}
	_, _ = pane.PTY.Write([]byte{rt.prefixByte()})
	return ok("")
}

func (rt *Runtime) cmdSendKeys(args []string, currentSession string) protocol.Message {
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	repeat := 1
	if value := optionValue(args, "-N", ""); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return fail("repeat count invalid")
		}
		repeat = parsed
	}
	keys := nonOptionArgs(args)
	for i := 0; i < repeat; i++ {
		sendKeysToPane(pane, keys, hasAny(args, "-l"))
	}
	return ok("")
}

func sendKeysToPane(pane *model.Pane, keys []string, literal bool) {
	if pane == nil || pane.PTY == nil {
		return
	}
	for _, key := range keys {
		if literal {
			_, _ = pane.PTY.Write([]byte(key))
			continue
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
	valueFlags := map[string]bool{
		"-c": true,
		"-d": true,
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
	case "splitw":
		return "split-window"
	case "selectw":
		return "select-window"
	case "display":
		return "display-message"
	case "last":
		return "last-window"
	case "next":
		return "next-window"
	case "prev":
		return "previous-window"
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
	case "loadb":
		return "load-buffer"
	case "saveb":
		return "save-buffer"
	case "killp":
		return "kill-pane"
	case "killw":
		return "kill-window"
	case "unlinkw":
		return "unlink-window"
	case "rename":
		return "rename-session"
	case "renamew":
		return "rename-window"
	case "swapw":
		return "swap-window"
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
	case "wait":
		return "wait-for"
	case "kill-server", "kill-session", "rename-session", "rename-window", "swap-window", "move-window", "unlink-window",
		"send-keys", "display-message", "capture-pane", "clear-history", "clear-prompt-history", "detach-client", "version",
		"source-file", "set-option", "set-window-option", "show-options", "show-window-options",
		"bind-key", "unbind-key", "list-keys", "set-environment",
		"show-environment", "if-shell", "send-prefix", "resize-pane", "resize-window", "respawn-pane", "respawn-window", "last-window", "last-pane", "next-layout", "previous-layout", "select-layout",
		"swap-pane", "rotate-window", "run-shell", "break-pane", "join-pane", "move-pane",
		"set-buffer", "show-buffer", "list-buffers", "delete-buffer",
		"paste-buffer", "load-buffer", "save-buffer", "list-clients", "list-commands", "show-prompt-history", "start-server", "wait-for":
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

func listWindowsCommand(state *model.Server, args []string, currentSession string) string {
	format := optionValue(args, "-F", "")
	if hasAny(args, "-a") {
		lines := make([]string, 0)
		for _, session := range snapshotSessions(state) {
			lines = append(lines, listWindowsForSession(session, format, true)...)
		}
		return strings.Join(lines, "\n")
	}
	return listWindowsFormat(state, targetSession(args, currentSession), format)
}

func listWindowsFormat(state *model.Server, sessionName string, format string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		return strings.Join(listWindowsForSession(session, format, false), "\n")
	}
	return ""
}

func listWindowsForSession(session *model.Session, format string, includeSession bool) []string {
	lines := make([]string, 0, len(session.Windows))
	for _, window := range session.Windows {
		if format != "" {
			lines = append(lines, formatString(format, formatContext{session: session, window: window, pane: window.ActivePane()}))
			continue
		}
		mark := ""
		active := session.ActiveWindow()
		if active != nil && active.ID == window.ID {
			mark = "*"
		}
		prefix := ""
		if includeSession {
			prefix = session.Name + ":"
		}
		lines = append(lines, fmt.Sprintf("%s%d: %s%s (%d panes)", prefix, window.Index, window.Name, mark, len(window.Panes)))
	}
	return lines
}

func listPanes(state *model.Server, sessionName string) string {
	return listPanesFormat(state, sessionName, "")
}

func listPanesCommand(state *model.Server, args []string, currentSession string) string {
	format := optionValue(args, "-F", "")
	if hasAny(args, "-a") {
		lines := make([]string, 0)
		for _, session := range snapshotSessions(state) {
			for _, window := range session.Windows {
				lines = append(lines, listPanesForWindow(session, window, format, true, true)...)
			}
		}
		return strings.Join(lines, "\n")
	}
	if hasAny(args, "-s") {
		sessionName, _, _, _, _ := parsePaneTarget(targetSession(args, currentSession))
		if sessionName == "" {
			sessionName = targetSession(args, currentSession)
		}
		if sessionName == "" {
			sessionName = firstSessionName(state)
		}
		for _, session := range snapshotSessions(state) {
			if session.Name != sessionName {
				continue
			}
			lines := make([]string, 0)
			for _, window := range session.Windows {
				lines = append(lines, listPanesForWindow(session, window, format, false, true)...)
			}
			return strings.Join(lines, "\n")
		}
		return ""
	}
	return listPanesFormat(state, targetSession(args, currentSession), format)
}

func listPanesFormat(state *model.Server, sessionName string, format string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	targetSessionName, targetWindowIndex, _, hasWindow, _ := parsePaneTarget(sessionName)
	if targetSessionName != "" || hasWindow {
		sessionName = targetSessionName
	}
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		window := session.ActiveWindow()
		if hasWindow {
			window = nil
			for _, candidate := range session.Windows {
				if candidate.Index == targetWindowIndex {
					window = candidate
					break
				}
			}
		}
		if window == nil {
			return ""
		}
		return strings.Join(listPanesForWindow(session, window, format, false, false), "\n")
	}
	return ""
}

func listPanesForWindow(session *model.Session, window *model.Window, format string, includeSession bool, includeWindow bool) []string {
	lines := make([]string, 0, len(window.Panes))
	for _, pane := range window.Panes {
		if format != "" {
			lines = append(lines, formatString(format, formatContext{session: session, window: window, pane: pane}))
			continue
		}
		mark := ""
		if pane.Index == window.Active {
			mark = "*"
		}
		state := "running"
		if pane.Exited {
			state = "exited"
		}
		prefix := ""
		if includeSession {
			prefix += session.Name + ":"
		}
		if includeWindow {
			prefix += fmt.Sprintf("%d.", window.Index)
		}
		lines = append(lines, fmt.Sprintf("%s%d:%s [%dx%d] %s %s",
			prefix, pane.Index, mark, pane.Width, pane.Height, state, model.CommandString(pane.Command)))
	}
	return lines
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

func formatContextForPaneID(state *model.Server, paneID int) formatContext {
	for _, session := range snapshotSessions(state) {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if pane.ID == paneID {
					return formatContext{session: session, window: window, pane: pane}
				}
			}
		}
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

func formatCommand(template string, info commandInfo) string {
	out := template
	replacements := map[string]string{
		"#{command_list_name}":  info.Name,
		"#{command_list_alias}": info.Alias,
		"#{command_list_usage}": info.Usage,
	}
	for old, newValue := range replacements {
		out = strings.ReplaceAll(out, old, newValue)
	}
	return out
}

func formatClient(template string, client model.Client) string {
	out := template
	replacements := map[string]string{
		"#{client_name}":     clientName(client),
		"#{session_name}":    client.SessionName,
		"#{client_width}":    strconv.Itoa(client.Width),
		"#{client_height}":   strconv.Itoa(client.Height),
		"#{client_termname}": "screen-256color",
	}
	for old, newValue := range replacements {
		out = strings.ReplaceAll(out, old, newValue)
	}
	return out
}

func clientName(client model.Client) string {
	return fmt.Sprintf("client-%d", client.ID)
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

func (rt *Runtime) targetLivePane(target string, currentSession string) *model.Pane {
	sessionName, windowIndex, paneIndex, hasWindow, hasPane := parsePaneTarget(target)
	if sessionName == "" {
		sessionName = currentSession
	}
	if sessionName == "" {
		sessionName = firstSessionName(rt.state)
	}
	return rt.state.TargetPane(sessionName, windowIndex, hasWindow, paneIndex, hasPane)
}

func (rt *Runtime) adjacentPane(paneID int, delta int) *model.Pane {
	for _, session := range snapshotSessions(rt.state) {
		for _, window := range session.Windows {
			for index, pane := range window.Panes {
				if pane.ID != paneID {
					continue
				}
				if len(window.Panes) == 0 {
					return nil
				}
				next := (index + delta + len(window.Panes)) % len(window.Panes)
				return window.Panes[next]
			}
		}
	}
	return nil
}

func (rt *Runtime) targetWindowInfo(target string, currentSession string) (string, int, bool, []int, bool) {
	sessionName, windowIndex, _, hasWindow, _ := parsePaneTarget(target)
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
			return sessionName, windowIndex, hasWindow, nil, false
		}
		paneIDs := make([]int, 0, len(window.Panes))
		for _, pane := range window.Panes {
			paneIDs = append(paneIDs, pane.ID)
		}
		return sessionName, window.Index, hasWindow, paneIDs, true
	}
	return sessionName, windowIndex, hasWindow, nil, false
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
	rows := rt.capturePaneRows(pane, true, trimTrailing)
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.Text
	}
	return lines
}

type capturePaneRow struct {
	Text    string
	Wrapped bool
	Number  int
}

func (rt *Runtime) capturePaneRows(pane *model.Pane, includeEmptyCells bool, trimTrailing bool) []capturePaneRow {
	var rows []capturePaneRow
	rt.screensMu.RLock()
	screen := rt.screens[pane.ID]
	rt.screensMu.RUnlock()
	if screen != nil {
		screenRows := screen.CaptureRowsWithOptions(includeEmptyCells, trimTrailing)
		rows = make([]capturePaneRow, len(screenRows))
		for i, row := range screenRows {
			rows[i] = capturePaneRow{Text: row.Text, Wrapped: row.Wrapped, Number: i}
		}
	} else {
		lines := visibleTextLines(pane.History.Bytes(), pane.Height)
		if pane.Height > 0 && len(lines) < pane.Height {
			lines = append(lines, make([]string, pane.Height-len(lines))...)
		}
		rows = make([]capturePaneRow, len(lines))
		for i, line := range lines {
			if includeEmptyCells && pane.Width > 0 && len(line) < pane.Width {
				width := captureExpandedLineSize(pane.Width, len(line))
				line += strings.Repeat(" ", width-len(line))
			}
			if trimTrailing {
				line = strings.TrimRight(line, " ")
			}
			rows[i] = capturePaneRow{Text: line, Number: i}
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

func sliceCaptureRows(rows []capturePaneRow, startValue string, endValue string) []capturePaneRow {
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

func formatCaptureRows(rows []capturePaneRow, numberLines bool, showFlags bool, joinLines bool, escapeSequences bool) string {
	var b strings.Builder
	for i, row := range rows {
		if numberLines {
			b.WriteString(strconv.Itoa(row.Number))
			b.WriteByte(' ')
		}
		if showFlags {
			if row.Wrapped {
				b.WriteByte('W')
			} else {
				b.WriteByte('-')
			}
			b.WriteByte(' ')
		}
		text := row.Text
		if escapeSequences {
			text = escapeCaptureText(text)
		}
		b.WriteString(text)
		if i < len(rows)-1 && (!joinLines || !row.Wrapped) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func escapeCaptureText(text string) string {
	return strings.ReplaceAll(text, `\`, `\\`)
}

func captureExpandedLineSize(width int, used int) int {
	if used <= 0 || width <= 0 {
		return 0
	}
	size := used
	if quarter := width / 4; size < quarter {
		size = quarter
	} else if half := width / 2; size < half {
		size = half
	} else if width > size {
		size = width
	}
	if size > width {
		return width
	}
	return size
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

func selectWindowTarget(target string, currentSession string) (string, int, bool) {
	cleaned := cleanSessionTarget(target)
	if strings.Contains(cleaned, ":") {
		sessionName, windowIndex, _, hasWindow, _ := parsePaneTarget(cleaned)
		if sessionName == "" {
			sessionName = currentSession
		}
		return sessionName, windowIndex, hasWindow
	}
	index, ok := parseWindowTarget(cleaned)
	return currentSession, index, ok
}

func moveWindowTarget(target string, currentSession string) (string, int, bool) {
	sessionName, windowIndex, _, hasWindow, _ := parsePaneTarget(target)
	if sessionName == "" {
		sessionName = currentSession
	}
	return sessionName, windowIndex, hasWindow
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
