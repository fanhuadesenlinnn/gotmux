package server

import (
	"fmt"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/command"
	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/version"
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
	{Name: "choose-buffer", Usage: "[-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
	{Name: "choose-client", Usage: "[-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
	{Name: "choose-tree", Usage: "[-GNrswZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]"},
	{Name: "clear-prompt-history", Alias: "clearphist", Usage: "[-T prompt-type]"},
	{Name: "clear-history", Alias: "clearhist", Usage: "[-H] [-t target-pane]"},
	{Name: "clock-mode", Usage: "[-t target-pane]"},
	{Name: "command-prompt", Usage: "[-1CbeFiklN] [-I inputs] [-p prompts] [-t target-client] [-T prompt-type] [template]"},
	{Name: "copy-mode", Usage: "[-deHMqSu] [-s src-pane] [-t target-pane]"},
	{Name: "customize-mode", Usage: "[-NZ] [-F format] [-f filter] [-t target-pane]"},
	{Name: "delete-buffer", Alias: "deleteb", Usage: "[-b buffer-name]"},
	{Name: "detach-client", Alias: "detach", Usage: "[-aP] [-E shell-command] [-s target-session] [-t target-client]"},
	{Name: "confirm-before", Alias: "confirm", Usage: "[-by] [-c confirm-key] [-p prompt] [-t target-client] command"},
	{Name: "display-menu", Alias: "menu", Usage: "[-MO] [-b border-lines] [-c target-client] [-C starting-choice] [-H selected-style] [-s style] [-S border-style] [-t target-pane] [-T title] [-x position] [-y position] name [key] [command] ..."},
	{Name: "display-message", Alias: "display", Usage: "[-aCIlNpv] [-c target-client] [-d delay] [-F format] [-t target-pane] [message]"},
	{Name: "display-panes", Alias: "displayp", Usage: "[-bN] [-d duration] [-t target-client] [template]"},
	{Name: "display-popup", Alias: "popup", Usage: "[-BCEkN] [-b border-lines] [-c target-client] [-d start-directory] [-e environment] [-h height] [-s style] [-S border-style] [-t target-pane] [-T title] [-w width] [-x position] [-y position] [shell-command [argument ...]]"},
	{Name: "find-window", Alias: "findw", Usage: "[-CiNrTZ] [-t target-pane] match-string"},
	{Name: "has-session", Alias: "has", Usage: "[-t target-session]"},
	{Name: "if-shell", Alias: "if", Usage: "[-bF] [-t target-pane] shell-command command [command]"},
	{Name: "join-pane", Alias: "joinp", Usage: "[-bdfhv] [-l size] [-s src-pane] [-t dst-pane]"},
	{Name: "kill-pane", Alias: "killp", Usage: "[-a] [-t target-pane]"},
	{Name: "kill-server"},
	{Name: "kill-session", Usage: "[-aC] [-t target-session]"},
	{Name: "kill-window", Alias: "killw", Usage: "[-a] [-t target-window]"},
	{Name: "last-pane", Alias: "lastp", Usage: "[-deZ] [-t target-window]"},
	{Name: "last-window", Alias: "last", Usage: "[-t target-session]"},
	{Name: "link-window", Alias: "linkw", Usage: "[-abdk] [-s src-window] [-t dst-window]"},
	{Name: "list-buffers", Alias: "lsb", Usage: "[-F format] [-f filter] [-O order]"},
	{Name: "list-clients", Alias: "lsc", Usage: "[-F format] [-f filter] [-O order][-t target-session]"},
	{Name: "list-commands", Alias: "lscm", Usage: "[-F format] [command]"},
	{Name: "list-keys", Alias: "lsk", Usage: "[-1aN] [-F format] [-P prefix-string] [-T key-table] [key]"},
	{Name: "list-panes", Alias: "lsp", Usage: "[-as] [-F format] [-f filter] [-t target]"},
	{Name: "list-sessions", Alias: "ls", Usage: "[-F format] [-f filter]"},
	{Name: "list-windows", Alias: "lsw", Usage: "[-ar] [-F format] [-f filter] [-O order][-t target-session]"},
	{Name: "load-buffer", Alias: "loadb", Usage: "[-b buffer-name] path"},
	{Name: "lock-client", Alias: "lockc", Usage: "[-t target-client]"},
	{Name: "lock-server", Alias: "lock"},
	{Name: "lock-session", Alias: "locks", Usage: "[-t target-session]"},
	{Name: "move-pane", Alias: "movep", Usage: "[-bdfhv] [-l size] [-s src-pane] [-t dst-pane]"},
	{Name: "move-window", Alias: "movew", Usage: "[-abdk] [-s src-window] [-t dst-window]"},
	{Name: "new-pane", Alias: "newp", Usage: "[-bdefhIklPvZ] [-c start-directory] [-e environment] [-F format] [-l size] [-m message] [-p percentage] [-s style] [-S active-border-style] [-R inactive-border-style] [-x width] [-y height] [-X x-position] [-Y y-position] [-t target-pane] [shell-command [argument ...]]"},
	{Name: "new-session", Alias: "new", Usage: "[-AdDEPX] [-c start-directory] [-e environment] [-F format] [-f flags] [-n window-name] [-s session-name] [-t target-session] [-x width] [-y height] [shell-command [argument ...]]"},
	{Name: "new-window", Alias: "neww", Usage: "[-abdkPS] [-c start-directory] [-e environment] [-F format] [-n window-name] [-t target-window] [shell-command [argument ...]]"},
	{Name: "next-layout", Alias: "nextl", Usage: "[-t target-window]"},
	{Name: "next-window", Alias: "next", Usage: "[-a] [-t target-session]"},
	{Name: "paste-buffer", Alias: "pasteb", Usage: "[-dpr] [-b buffer-name] [-s separator] [-t target-pane]"},
	{Name: "pipe-pane", Alias: "pipep", Usage: "[-IOo] [-t target-pane] [shell-command]"},
	{Name: "previous-layout", Alias: "prevl", Usage: "[-t target-window]"},
	{Name: "previous-window", Alias: "prev", Usage: "[-a] [-t target-session]"},
	{Name: "refresh-client", Alias: "refresh", Usage: "[-cDlLRSU] [-A pane:state] [-B name:what:format] [-C XxY] [-f flags] [-r pane:report] [-t target-client] [adjustment]"},
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
	{Name: "server-access", Usage: "[-adlrw] [-t target-pane] [user]"},
	{Name: "set-buffer", Alias: "setb", Usage: "[-aw] [-b buffer-name] [-n new-buffer-name] [-t target-client] [data]"},
	{Name: "set-environment", Alias: "setenv", Usage: "[-Fhgru] [-t target-session] name [value]"},
	{Name: "set-hook", Usage: "[-agpRuw] [-t target-pane] hook [command]"},
	{Name: "set-option", Alias: "set", Usage: "[-aFgopqsuUw] [-t target-pane] option [value]"},
	{Name: "set-window-option", Alias: "setw", Usage: "[-aFgoqu] [-t target-window] option [value]"},
	{Name: "show-buffer", Alias: "showb", Usage: "[-b buffer-name]"},
	{Name: "show-environment", Alias: "showenv", Usage: "[-hgs] [-t target-session] [name]"},
	{Name: "show-hooks", Usage: "[-gpw] [-t target-pane] [hook]"},
	{Name: "show-messages", Alias: "showmsgs", Usage: "[-JT] [-t target-client]"},
	{Name: "show-options", Alias: "show", Usage: "[-AgHpqsvw] [-t target-pane] [option]"},
	{Name: "show-prompt-history", Alias: "showphist", Usage: "[-T prompt-type]"},
	{Name: "show-window-options", Alias: "showw", Usage: "[-gv] [-t target-window] [option]"},
	{Name: "source-file", Alias: "source", Usage: "[-Fnqv] path ..."},
	{Name: "split-window", Alias: "splitw", Usage: "[-bdfhIvPZ] [-c start-directory] [-e environment] [-F format] [-l size] [-t target-pane] [shell-command [argument ...]]"},
	{Name: "start-server", Alias: "start"},
	{Name: "suspend-client", Alias: "suspendc", Usage: "[-t target-client]"},
	{Name: "swap-pane", Alias: "swapp", Usage: "[-dDUZ] [-s src-pane] [-t dst-pane]"},
	{Name: "swap-window", Alias: "swapw", Usage: "[-d] [-s src-window] [-t dst-window]"},
	{Name: "switch-client", Alias: "switchc", Usage: "[-ElnprZ] [-c target-client] [-t target-session] [-T key-table] [-O order]"},
	{Name: "unlink-window", Alias: "unlinkw", Usage: "[-k] [-t target-window]"},
	{Name: "unbind-key", Alias: "unbind", Usage: "[-anq] [-T key-table] key"},
	{Name: "wait-for", Alias: "wait", Usage: "[-L|-S|-U] channel"},
}

func (rt *Runtime) executeMessage(msg protocol.Message, currentSession string) protocol.Message {
	return rt.executeMessageForClient(msg, currentSession, 0)
}

func (rt *Runtime) executeMessageForClient(msg protocol.Message, currentSession string, clientID int64) protocol.Message {
	commands := msg.Commands
	var err error
	if len(commands) == 0 {
		commands, err = command.ParseArgv(msg.Command)
		if err != nil {
			return fail(err.Error())
		}
	}
	rt.addCommandMessages(commands, clientID)
	return rt.executeCommandsForClient(commands, currentSession, msg.Width, msg.Height, clientID)
}

func (rt *Runtime) executeCommands(commands [][]string, currentSession string, width, height int) protocol.Message {
	return rt.executeCommandsForClient(commands, currentSession, width, height, 0)
}

func (rt *Runtime) executeCommandsForClient(commands [][]string, currentSession string, width, height int, clientID int64) protocol.Message {
	var texts []string
	last := ok("")
	activeSession := currentSession
	for _, argv := range commands {
		if len(argv) == 0 {
			continue
		}
		last = rt.executeWithClient(argv, activeSession, width, height, clientID)
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

func (rt *Runtime) addCommandMessages(commands [][]string, clientID int64) {
	for _, argv := range commands {
		if len(argv) == 0 {
			continue
		}
		commandText := model.CommandString(argv)
		if clientID != 0 {
			rt.state.AddMessage(fmt.Sprintf("client-%d command: %s", clientID, commandText))
			continue
		}
		rt.state.AddMessage("command: " + commandText)
	}
}

func (rt *Runtime) execute(argv []string, currentSession string, width, height int) protocol.Message {
	return rt.executeWithClient(argv, currentSession, width, height, 0)
}

func (rt *Runtime) executeWithClient(argv []string, currentSession string, width, height int, clientID int64) protocol.Message {
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
	case "choose-tree":
		if clientID == 0 {
			return ok("")
		}
		return rt.cmdChooseTree(args, currentSession)
	case "choose-buffer":
		if clientID == 0 {
			return ok("")
		}
		return rt.cmdChooseBuffer()
	case "clock-mode", "copy-mode", "choose-client", "customize-mode", "find-window":
		return ok("")
	case "command-prompt", "confirm-before", "display-menu", "display-popup", "suspend-client":
		if clientID == 0 {
			return fail("no current client")
		}
		return ok("")
	case "display-panes":
		if clientID == 0 {
			return fail("no current client")
		}
		return rt.cmdDisplayPanes(clientID)
	case "list-sessions":
		return ok(listSessionsCommand(rt.state, args))
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
	case "new-pane":
		return rt.cmdNewPane(args, currentSession, width, height)
	case "split-window":
		return rt.cmdSplitWindow(args, currentSession, width, height)
	case "source-file":
		return rt.cmdSourceFile(args, currentSession, width, height)
	case "set-option":
		return rt.cmdSetOption(args, currentSession, "session")
	case "set-window-option":
		return rt.cmdSetOption(args, currentSession, "window")
	case "set-hook":
		return rt.cmdSetHook(args, currentSession)
	case "show-options":
		return rt.cmdShowOptions(args, currentSession)
	case "show-window-options":
		windowArgs := append([]string{"-w"}, args...)
		return rt.cmdShowOptions(windowArgs, currentSession)
	case "show-hooks":
		return rt.cmdShowHooks(args, currentSession)
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
	case "server-access":
		return rt.cmdServerAccess(args)
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
	case "link-window":
		return rt.cmdLinkWindow(args, currentSession)
	case "swap-window":
		return rt.cmdSwapWindow(args, currentSession)
	case "move-window":
		return rt.cmdMoveWindow(args, currentSession)
	case "kill-session":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if target == "" {
			return fail("no current session")
		}
		if hasAny(args, "-a") {
			killed, err := rt.state.KillOtherSessions(target)
			if err != nil {
				return fail(err.Error())
			}
			rt.screensMu.Lock()
			for _, paneID := range killed {
				delete(rt.screens, paneID)
			}
			rt.screensMu.Unlock()
			return ok("")
		}
		if err := rt.state.KillSession(target); err != nil {
			return fail(err.Error())
		}
		return ok("")
	case "start-server":
		return ok("")
	case "kill-server":
		rt.stopServerSoon("server exited")
		return ok("")
	case "lock-server":
		return ok("")
	case "lock-session":
		target := cleanSessionTarget(optionValue(args, "-t", currentSession))
		if target == "" {
			target = firstSessionName(rt.state)
		}
		if !sessionExists(rt.state, target) {
			return fail(fmt.Sprintf("can't find session: %s", target))
		}
		return ok("")
	case "lock-client":
		return fail("no current client")
	case "refresh-client":
		if clientID == 0 {
			return fail("no current client")
		}
		return ok("")
	case "switch-client":
		return rt.cmdSwitchClient(args, clientID)
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
	case "show-messages":
		return rt.cmdShowMessages(args)
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
	case "pipe-pane":
		return rt.cmdPipePane(args, currentSession)
	case "load-buffer":
		return rt.cmdLoadBuffer(args)
	case "save-buffer":
		return rt.cmdSaveBuffer(args)
	case "detach-client":
		return rt.cmdDetachClient(args, clientID)
	case "version":
		return ok(version.String)
	default:
		return fail(fmt.Sprintf("unknown command: %s", argv[0]))
	}
}
