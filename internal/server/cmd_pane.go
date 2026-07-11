package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdSplitWindow(args []string, currentSession string, width, height int) protocol.Message {
	valueOptions := map[string]bool{"-c": true, "-e": true, "-F": true, "-t": true, "-l": true, "-p": true}
	optionArgs := commandOptionArgs(args, valueOptions)
	sessionName, windowIndex, hasWindow, _, found := rt.targetWindowInfo(optionValue(optionArgs, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	cwd := optionValue(optionArgs, "-c", "")
	command := trailingCommand(args, valueOptions)
	orientation := "vertical"
	if hasAny(optionArgs, "-h") {
		orientation = "horizontal"
	}
	var pane *model.Pane
	var err error
	if hasWindow {
		pane, err = rt.state.SplitPaneWithLayoutByIndexDetached(sessionName, windowIndex, cwd, command, orientation, hasAny(optionArgs, "-d"))
	} else {
		pane, err = rt.state.SplitPaneWithLayoutDetached(sessionName, cwd, command, orientation, hasAny(optionArgs, "-d"))
	}
	if err != nil {
		return fail(err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(pane.ID, environmentOverrides(optionArgs))
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	if hasAny(optionArgs, "-P") {
		template := optionValue(optionArgs, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		return ok(formatString(template, formatContextForPaneID(rt.state, pane.ID)))
	}
	return ok("")
}

func (rt *Runtime) cmdNewPane(args []string, currentSession string, width, height int) protocol.Message {
	valueOptions := map[string]bool{
		"-B": true, "-c": true, "-e": true, "-F": true, "-l": true,
		"-m": true, "-p": true, "-s": true, "-S": true, "-R": true,
		"-t": true, "-T": true, "-x": true, "-X": true, "-y": true, "-Y": true,
	}
	optionArgs := commandOptionArgs(args, valueOptions)
	if hasAny(optionArgs, "-L") {
		return rt.cmdSplitWindow(args, currentSession, width, height)
	}
	sessionName, windowIndex, hasWindow, _, found := rt.targetWindowInfo(optionValue(optionArgs, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find pane")
	}
	cwd := optionValue(optionArgs, "-c", "")
	command := trailingCommand(args, valueOptions)
	windowWidth, windowHeight := rt.windowSize(sessionName, windowIndex)
	if windowWidth <= 0 {
		windowWidth = width
	}
	if windowHeight <= 0 {
		windowHeight = height
	}
	paneWidth := sizeOption(optionArgs, "-x", windowWidth, max(1, windowWidth/2))
	paneHeight := sizeOption(optionArgs, "-y", windowHeight, max(1, windowHeight/4))
	left := positionOption(optionArgs, "-X", windowWidth, 4)
	top := positionOption(optionArgs, "-Y", windowHeight, 2)
	pane, err := rt.state.NewFloatingPaneDetached(sessionName, windowIndex, hasWindow, cwd, command, hasAny(optionArgs, "-d"), left, top, paneWidth, paneHeight)
	if err != nil {
		return fail(err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(pane.ID, environmentOverrides(optionArgs))
	if err := rt.startPane(pane, pane.Width, pane.Height); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	if hasAny(optionArgs, "-P") {
		template := optionValue(optionArgs, "-F", "#{session_name}:#{window_index}.#{pane_index}")
		return ok(formatString(template, formatContextForPaneID(rt.state, pane.ID)))
	}
	return ok("")
}

func (rt *Runtime) cmdResizePane(args []string, currentSession string) protocol.Message {
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	pane := rt.targetPane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	if hasAny(args, "-Z") {
		if err := rt.state.TogglePaneZoom(pane.ID); err != nil {
			return fail(err.Error())
		}
		rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
		return ok("")
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
	if err := rt.state.ResizePaneByID(pane.ID, direction, amount); err != nil {
		return fail(err.Error())
	}
	rt.resizePanes(rt.state.WindowPanesContainingPane(pane.ID))
	return ok("")
}

func (rt *Runtime) cmdRespawnPane(args []string, currentSession string, width, height int) protocol.Message {
	valueOptions := map[string]bool{"-c": true, "-e": true, "-t": true}
	optionArgs := commandOptionArgs(args, valueOptions)
	pane := rt.targetPane(optionValue(optionArgs, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	command := trailingCommand(args, valueOptions)
	respawned, err := rt.state.RespawnPaneByID(pane.ID, optionValue(optionArgs, "-c", ""), command, hasAny(optionArgs, "-k"))
	if err != nil {
		return fail("respawn pane failed: " + err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(respawned.ID, environmentOverrides(optionArgs))
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
	valueOptions := map[string]bool{"-c": true, "-e": true, "-t": true}
	optionArgs := commandOptionArgs(args, valueOptions)
	sessionName, windowIndex, _, _, found := rt.targetWindowInfo(optionValue(optionArgs, "-t", currentSession), currentSession)
	if !found {
		return fail("can't find window")
	}
	command := trailingCommand(args, valueOptions)
	pane, killed, err := rt.state.RespawnWindowByIndex(sessionName, windowIndex, optionValue(optionArgs, "-c", ""), command, hasAny(optionArgs, "-k"))
	if err != nil {
		return fail("respawn window failed: " + err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(pane.ID, environmentOverrides(optionArgs))
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

func (rt *Runtime) cmdPipePane(args []string, currentSession string) protocol.Message {
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	if pane.Exited {
		return fail("target pane has exited")
	}
	command := strings.Join(trailingCommand(args, map[string]bool{"-t": true}), " ")
	if command == "" {
		rt.closePanePipe(pane.ID)
		return ok("")
	}
	in := hasAny(args, "-I")
	out := hasAny(args, "-O")
	if !in {
		out = true
	}
	if err := rt.openPanePipe(pane, command, in, out, hasAny(args, "-o")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func listPanes(state *model.Server, sessionName string) string {
	return listPanesFormat(state, sessionName, "")
}

func listPanesCommand(state *model.Server, args []string, currentSession string) string {
	format := optionValue(args, "-F", "")
	filter := optionValue(args, "-f", "")
	if hasAny(args, "-a") {
		lines := make([]string, 0)
		for _, session := range snapshotSessions(state) {
			for _, window := range session.Windows {
				lines = append(lines, listPanesForWindow(session, window, format, filter, true, true)...)
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
				lines = append(lines, listPanesForWindow(session, window, format, filter, false, true)...)
			}
			return strings.Join(lines, "\n")
		}
		return ""
	}
	return listPanesFormatFiltered(state, targetSession(args, currentSession), format, filter)
}

func listPanesFormat(state *model.Server, sessionName string, format string) string {
	return listPanesFormatFiltered(state, sessionName, format, "")
}

func listPanesFormatFiltered(state *model.Server, sessionName string, format string, filter string) string {
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
		return strings.Join(listPanesForWindow(session, window, format, filter, false, false), "\n")
	}
	return ""
}

func listPanesForWindow(session *model.Session, window *model.Window, format string, filter string, includeSession bool, includeWindow bool) []string {
	lines := make([]string, 0, len(window.Panes))
	for _, pane := range window.Panes {
		ctx := formatContext{session: session, window: window, pane: pane}
		if filter != "" && !formatTruthy(filter, ctx) {
			continue
		}
		if format != "" {
			lines = append(lines, formatString(format, ctx))
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
