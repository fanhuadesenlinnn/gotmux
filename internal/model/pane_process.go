package model

import "time"

func (s *Server) environmentLocked(session *Session) []string {
	env := make(map[string]string, len(s.GlobalEnvironment)+len(session.Environment))
	for key, value := range s.GlobalEnvironment {
		env[key] = value
	}
	for key, value := range session.Environment {
		env[key] = value
	}
	for key := range session.RemovedEnv {
		delete(env, key)
	}
	return environmentList(env)
}

func killPane(pane *Pane) {
	if pane == nil {
		return
	}
	pane.Generation++
	if pane.PTY != nil {
		_ = pane.PTY.Close()
	}
	if pane.Process != nil && pane.Process.Process != nil {
		_ = pane.Process.Process.Kill()
	}
	pane.Exited = true
}

func respawnPaneLocked(session *Session, window *Window, pane *Pane, cwd string, command []string, killActive bool) {
	if killActive {
		killPane(pane)
	}
	if cwd != "" {
		pane.CWD = cwd
	}
	if len(command) > 0 {
		pane.Command = NormalizeCommand(command)
	}
	pane.PTY = nil
	pane.Process = nil
	pane.History = NewRing(HistoryBytes)
	pane.Exited = false
	pane.ExitState = ""
	pane.Activity = time.Now()
	if window != nil {
		window.Activity = time.Now()
	}
	if session != nil {
		session.Activity = time.Now()
	}
}

func reindexPanes(window *Window) {
	for i, pane := range window.Panes {
		pane.Index = i
	}
}
