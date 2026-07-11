package server

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdNewSession(args []string, width, height int) protocol.Message {
	valueOptions := map[string]bool{
		"-s": true, "-n": true, "-c": true, "-e": true, "-F": true, "-t": true, "-x": true, "-y": true,
	}
	optionArgs := commandOptionArgs(args, valueOptions)
	width, height = commandSize(optionArgs, width, height)
	name := optionValue(optionArgs, "-s", "")
	windowName := optionValue(optionArgs, "-n", "")
	cwd := optionValue(optionArgs, "-c", "")
	command := trailingCommand(args, valueOptions)
	if hasAny(optionArgs, "-A") && name != "" && sessionExists(rt.state, name) {
		text := ""
		if hasAny(optionArgs, "-P") {
			text = formatString(optionValue(optionArgs, "-F", "#{session_name}:"), activeFormatContext(rt.state, name))
		}
		return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text, Session: name}
	}
	session, _, pane, err := rt.state.NewSession(name, cwd, windowName, command)
	if err != nil {
		return fail(err.Error())
	}
	rt.state.ApplyPaneEnvironmentOverrides(pane.ID, environmentOverrides(optionArgs))
	rt.state.SetActiveWindowSize(session.Name, width, height)
	if err := rt.startPane(pane, width, height); err != nil {
		return fail(err.Error())
	}
	text := ""
	if hasAny(optionArgs, "-P") {
		text = formatString(optionValue(optionArgs, "-F", "#{session_name}:"), activeFormatContext(rt.state, session.Name))
	}
	return protocol.Message{Type: protocol.TypeResult, OK: true, Text: text, Session: session.Name}
}

func (rt *Runtime) cmdSwitchClient(args []string, clientID int64) protocol.Message {
	targetClientID := clientID
	if targetClient := optionValue(args, "-c", ""); targetClient != "" {
		resolved, ok := rt.resolveClientTarget(targetClient)
		if !ok {
			return fail("can't find client: " + targetClient)
		}
		targetClientID = resolved
	} else if clientID == 0 {
		return fail("no current client")
	}
	if hasAny(args, "-n") {
		if err := rt.state.SwitchClientRelative(targetClientID, 1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	if hasAny(args, "-p") {
		if err := rt.state.SwitchClientRelative(targetClientID, -1); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	if hasAny(args, "-l") {
		if err := rt.state.SwitchClientLast(targetClientID); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	target := cleanSessionTarget(optionValue(args, "-t", ""))
	if target == "" {
		return ok("")
	}
	if strings.Contains(target, ":") || strings.Contains(target, ".") {
		target, _, _, _, _ = parsePaneTarget(target)
	}
	if target == "" {
		return fail("can't find session")
	}
	if err := rt.state.SwitchClient(targetClientID, target); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdDetachClient(args []string, clientID int64) protocol.Message {
	targets, errText := rt.detachClientTargets(args, clientID)
	if errText != "" {
		return fail(errText)
	}
	currentTargeted := false
	for _, target := range targets {
		if target == clientID {
			currentTargeted = true
			continue
		}
		rt.detachClient(target, "detached")
	}
	if currentTargeted {
		return protocol.Message{Type: protocol.TypeExit, OK: true, Text: "detached"}
	}
	return ok("")
}

func (rt *Runtime) detachClientTargets(args []string, clientID int64) ([]int64, string) {
	clients := rt.state.ListClients()
	if targetClient := optionValue(args, "-t", ""); targetClient != "" {
		targetID, ok := rt.resolveClientTarget(targetClient)
		if !ok {
			return nil, "can't find client: " + targetClient
		}
		if hasAny(args, "-a") {
			targets := make([]int64, 0, len(clients))
			for _, client := range clients {
				if client.ID != targetID {
					targets = append(targets, client.ID)
				}
			}
			return targets, ""
		}
		return []int64{targetID}, ""
	}
	if targetSession := cleanSessionTarget(optionValue(args, "-s", "")); targetSession != "" {
		targets := make([]int64, 0, len(clients))
		for _, client := range clients {
			if client.SessionName != targetSession {
				continue
			}
			if hasAny(args, "-a") && clientID != 0 && client.ID == clientID {
				continue
			}
			targets = append(targets, client.ID)
		}
		if len(targets) == 0 && len(clients) == 0 && clientID == 0 {
			return nil, "no current client"
		}
		return targets, ""
	}
	if clientID == 0 {
		return nil, "no current client"
	}
	if hasAny(args, "-a") {
		targets := make([]int64, 0, len(clients))
		for _, client := range clients {
			if client.ID != clientID {
				targets = append(targets, client.ID)
			}
		}
		return targets, ""
	}
	return []int64{clientID}, ""
}

func (rt *Runtime) resolveClientTarget(target string) (int64, bool) {
	idText := strings.TrimPrefix(target, "client-")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, rt.state.ClientExists(id)
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

func listSessions(state *model.Server) string {
	format := ""
	return listSessionsFormat(state, format)
}

func listSessionsCommand(state *model.Server, args []string) string {
	return listSessionsFormatFiltered(state, optionValue(args, "-F", ""), optionValue(args, "-f", ""))
}

func listSessionsFormat(state *model.Server, format string) string {
	return listSessionsFormatFiltered(state, format, "")
}

func listSessionsFormatFiltered(state *model.Server, format string, filter string) string {
	sessions := snapshotSessions(state)
	if len(sessions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ctx := formatContext{session: session, window: session.ActiveWindow(), pane: activePane(session)}
		if filter != "" && !formatTruthy(filter, ctx) {
			continue
		}
		if format != "" {
			lines = append(lines, formatString(format, ctx))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %d windows (created %s) [%dx%d]",
				session.Name, len(session.Windows), session.CreatedAt.Format("Mon Jan _2 15:04:05 2006"), activeWidth(session), activeHeight(session)))
		}
	}
	return strings.Join(lines, "\n")
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
