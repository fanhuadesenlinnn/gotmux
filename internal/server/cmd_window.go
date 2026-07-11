package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdNewWindow(args []string, currentSession string, width, height int) protocol.Message {
	valueOptions := map[string]bool{"-n": true, "-c": true, "-e": true, "-F": true, "-t": true}
	optionArgs := commandOptionArgs(args, valueOptions)
	if target := targetSession(optionArgs, currentSession); target != "" {
		currentSession = target
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	name := optionValue(optionArgs, "-n", "")
	cwd := optionValue(optionArgs, "-c", "")
	command := trailingCommand(args, valueOptions)
	_, pane, err := rt.state.NewWindowDetached(currentSession, name, cwd, command, hasAny(optionArgs, "-d"))
	if err != nil {
		return fail(err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(pane.ID, environmentOverrides(optionArgs))
	rt.state.SetActiveWindowSize(currentSession, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	if hasAny(optionArgs, "-P") {
		template := optionValue(optionArgs, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		return ok(formatString(template, formatContextForPaneID(rt.state, pane.ID)))
	}
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
	sessionName, windowIndex, _, _, found := rt.targetWindowInfo(optionValue(args, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	killed, err := rt.state.UnlinkWindow(sessionName, windowIndex, hasAny(args, "-k"))
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

func (rt *Runtime) cmdLinkWindow(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	sourceSession, sourceWindow, _, _, sourceFound := rt.targetWindowInfo(optionValue(args, "-s", currentSession), currentSession)
	if !sourceFound {
		return fail("can't find window")
	}
	targetSession, targetWindow, hasWindow := moveWindowTarget(optionValue(args, "-t", ""), currentSession)
	if !hasWindow {
		return fail("bad window target")
	}
	killed, err := rt.state.LinkWindow(sourceSession, sourceWindow, targetSession, targetWindow, hasAny(args, "-d"), hasAny(args, "-k"))
	if err != nil {
		return fail(err.Error())
	}
	rt.screensMu.Lock()
	for _, paneID := range killed {
		delete(rt.screens, paneID)
	}
	rt.screensMu.Unlock()
	rt.resizeSessionPanes(sourceSession)
	if targetSession != sourceSession {
		rt.resizeSessionPanes(targetSession)
	}
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

func listWindows(state *model.Server, sessionName string) string {
	return listWindowsFormat(state, sessionName, "")
}

func listWindowsCommand(state *model.Server, args []string, currentSession string) string {
	format := optionValue(args, "-F", "")
	filter := optionValue(args, "-f", "")
	if hasAny(args, "-a") {
		lines := make([]string, 0)
		for _, session := range snapshotSessions(state) {
			lines = append(lines, listWindowsForSession(session, format, filter, true)...)
		}
		return strings.Join(lines, "\n")
	}
	return listWindowsFormatFiltered(state, targetSession(args, currentSession), format, filter)
}

func listWindowsFormat(state *model.Server, sessionName string, format string) string {
	return listWindowsFormatFiltered(state, sessionName, format, "")
}

func listWindowsFormatFiltered(state *model.Server, sessionName string, format string, filter string) string {
	if sessionName == "" {
		sessionName = firstSessionName(state)
	}
	for _, session := range snapshotSessions(state) {
		if session.Name != sessionName {
			continue
		}
		return strings.Join(listWindowsForSession(session, format, filter, false), "\n")
	}
	return ""
}

func listWindowsForSession(session *model.Session, format string, filter string, includeSession bool) []string {
	lines := make([]string, 0, len(session.Windows))
	for _, window := range session.Windows {
		ctx := formatContext{session: session, window: window, pane: window.ActivePane()}
		if filter != "" && !formatTruthy(filter, ctx) {
			continue
		}
		if format != "" {
			lines = append(lines, formatString(format, ctx))
			continue
		}
		prefix := ""
		if includeSession {
			prefix = session.Name + ":"
		}
		lines = append(lines, fmt.Sprintf("%s%d: %s%s (%d panes)", prefix, window.Index, window.Name, windowFlags(session, window), len(window.Panes)))
	}
	return lines
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

func (rt *Runtime) windowSize(sessionName string, windowIndex int) (int, int) {
	for _, session := range snapshotSessions(rt.state) {
		if session.Name != sessionName {
			continue
		}
		for _, window := range session.Windows {
			if window.Index == windowIndex {
				return window.Width, window.Height
			}
		}
	}
	return 0, 0
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
