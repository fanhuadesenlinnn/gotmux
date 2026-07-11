package model

import (
	"fmt"
	"os"
	"time"
)

type Session struct {
	ID           int
	Name         string
	CWD          string
	CreatedAt    time.Time
	Activity     time.Time
	Windows      []*Window
	Active       int
	LastWindowID int
	Attached     int
	Options      map[string]string
	Environment  map[string]string
	HiddenEnv    map[string]string
	RemovedEnv   map[string]bool
	Hooks        map[string][]string
}

func (s *Server) NewSession(name, cwd string, windowName string, command []string) (*Session, *Window, *Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = os.Getenv("HOME")
		}
	}
	if name == "" {
		name = fmt.Sprintf("%d", s.NextSessionID)
	}
	if _, exists := s.Sessions[name]; exists {
		return nil, nil, nil, fmt.Errorf("duplicate session: %s", name)
	}

	session := &Session{
		ID:           s.NextSessionID,
		Name:         name,
		CWD:          cwd,
		CreatedAt:    time.Now(),
		Activity:     time.Now(),
		LastWindowID: -1,
	}
	s.NextSessionID++
	s.Sessions[name] = session

	window := s.newWindowLocked(session, defaultWindowName(windowName, command))
	pane := s.newPaneLocked(session, window, cwd, command)
	return session, window, pane, nil
}

func (s *Server) KillSession(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[name]
	if session == nil {
		return fmt.Errorf("can't find session: %s", name)
	}
	for _, window := range session.Windows {
		for _, pane := range window.Panes {
			killPane(pane)
		}
	}
	delete(s.Sessions, name)
	return nil
}

func (s *Server) KillOtherSessions(keepName string) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Sessions[keepName] == nil {
		return nil, fmt.Errorf("can't find session: %s", keepName)
	}
	var killedPanes []int
	for name, session := range s.Sessions {
		if name == keepName {
			continue
		}
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				killedPanes = append(killedPanes, pane.ID)
				killPane(pane)
			}
		}
		delete(s.Sessions, name)
	}
	return killedPanes, nil
}

func (s *Server) RenameSession(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newName == "" {
		return fmt.Errorf("empty session name")
	}
	if _, exists := s.Sessions[newName]; exists {
		return fmt.Errorf("duplicate session: %s", newName)
	}
	session := s.Sessions[oldName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", oldName)
	}
	delete(s.Sessions, oldName)
	session.Name = newName
	s.Sessions[newName] = session
	for _, c := range s.Clients {
		if c.SessionName == oldName {
			c.SessionName = newName
		}
	}
	return nil
}

func (s *Session) ActiveWindow() *Window {
	if s == nil || s.Active < 0 || s.Active >= len(s.Windows) {
		return nil
	}
	return s.Windows[s.Active]
}
