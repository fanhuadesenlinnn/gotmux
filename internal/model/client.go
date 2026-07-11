package model

import (
	"fmt"
	"sort"
	"time"
)

type Client struct {
	ID              int64
	SessionName     string
	LastSessionName string
	Width           int
	Height          int
	Prefix          bool
	ReadOnly        bool
}

func (s *Server) AttachClient(sessionName string, width, height int) (*Client, *Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionName == "" {
		for _, candidate := range s.Sessions {
			sessionName = candidate.Name
			break
		}
	}
	session := s.Sessions[sessionName]
	if session == nil {
		return nil, nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	c := &Client{
		ID:          s.NextClientID,
		SessionName: session.Name,
		Width:       width,
		Height:      height,
	}
	s.NextClientID++
	s.Clients[c.ID] = c
	session.Attached++
	session.Activity = time.Now()
	return c, session, nil
}

func (s *Server) DetachClient(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[id]
	if client == nil {
		return
	}
	if session := s.Sessions[client.SessionName]; session != nil && session.Attached > 0 {
		session.Attached--
	}
	delete(s.Clients, id)
}

func (s *Server) SwitchClient(clientID int64, sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[clientID]
	if client == nil {
		return fmt.Errorf("no current client")
	}
	target := s.Sessions[sessionName]
	if target == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	s.switchClientLocked(client, target)
	return nil
}

func (s *Server) SwitchClientRelative(clientID int64, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[clientID]
	if client == nil {
		return fmt.Errorf("no current client")
	}
	sessions := make([]*Session, 0, len(s.Sessions))
	for _, session := range s.Sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})
	if len(sessions) == 0 {
		return fmt.Errorf("can't find session")
	}
	currentIndex := -1
	for index, session := range sessions {
		if session.Name == client.SessionName {
			currentIndex = index
			break
		}
	}
	if currentIndex == -1 {
		return fmt.Errorf("can't find session: %s", client.SessionName)
	}
	next := sessions[(currentIndex+delta+len(sessions))%len(sessions)]
	s.switchClientLocked(client, next)
	return nil
}

func (s *Server) SwitchClientLast(clientID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[clientID]
	if client == nil {
		return fmt.Errorf("no current client")
	}
	if client.LastSessionName == "" {
		return fmt.Errorf("can't find last session")
	}
	target := s.Sessions[client.LastSessionName]
	if target == nil {
		return fmt.Errorf("can't find last session")
	}
	s.switchClientLocked(client, target)
	return nil
}

func (s *Server) switchClientLocked(client *Client, target *Session) {
	if client.SessionName == target.Name {
		return
	}
	if current := s.Sessions[client.SessionName]; current != nil && current.Attached > 0 {
		current.Attached--
	}
	client.LastSessionName = client.SessionName
	client.SessionName = target.Name
	target.Attached++
	target.Activity = time.Now()
}

func (s *Server) ListClients() []Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]Client, 0, len(s.Clients))
	for _, client := range s.Clients {
		clients = append(clients, *client)
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ID < clients[j].ID
	})
	return clients
}

func (s *Server) ClientExists(id int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Clients[id] != nil
}

func (s *Server) SetClientSize(id int64, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[id]
	if client == nil {
		return
	}
	client.Width = width
	client.Height = height
}

func (s *Server) Snapshot() ([]*Session, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Session, 0, len(s.Sessions))
	for _, session := range s.Sessions {
		copySession := *session
		copySession.Windows = make([]*Window, len(session.Windows))
		for i, window := range session.Windows {
			copyWindow := *window
			copyWindow.Panes = make([]*Pane, len(window.Panes))
			for j, pane := range window.Panes {
				copyPane := *pane
				copyPane.PTY = nil
				copyPane.Process = nil
				copyWindow.Panes[j] = &copyPane
			}
			copySession.Windows[i] = &copyWindow
		}
		out = append(out, &copySession)
	}
	return out, s.NextClientID
}

func (s *Server) ActivePane(sessionName string) *Pane {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil
	}
	window := session.ActiveWindow()
	if window == nil {
		return nil
	}
	return window.ActivePane()
}

func (s *Server) TargetPane(sessionName string, windowIndex int, hasWindow bool, paneIndex int, hasPane bool) *Pane {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil
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

func (s *Server) ApplyPaneEnvironmentOverrides(paneID int, overrides map[string]string) {
	if len(overrides) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.Sessions {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if pane.ID != paneID {
					continue
				}
				env := environmentMap(pane.Env)
				for key, value := range overrides {
					env[key] = value
				}
				pane.Env = environmentList(env)
				return
			}
		}
	}
}

func (s *Server) ActiveSessionName(clientID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client := s.Clients[clientID]
	if client == nil {
		return ""
	}
	return client.SessionName
}

func (s *Server) ClientSize(clientID int64) (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client := s.Clients[clientID]
	if client == nil {
		return 80, 24
	}
	return client.Width, client.Height
}

func (s *Server) ClientPrefix(clientID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client := s.Clients[clientID]
	return client != nil && client.Prefix
}

func (s *Server) SetClientPrefix(clientID int64, prefix bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.Clients[clientID]
	if client != nil {
		client.Prefix = prefix
	}
}
