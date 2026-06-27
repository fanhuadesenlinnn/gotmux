package model

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const HistoryBytes = 2 << 20

type Server struct {
	mu            sync.RWMutex
	Sessions      map[string]*Session
	Clients       map[int64]*Client
	NextSessionID int
	NextWindowID  int
	NextPaneID    int
	NextClientID  int64
	SocketPath    string
	StartedAt     time.Time
}

type Session struct {
	ID        int
	Name      string
	CWD       string
	CreatedAt time.Time
	Activity  time.Time
	Windows   []*Window
	Active    int
	Attached  int
}

type Window struct {
	ID        int
	Index     int
	Name      string
	CreatedAt time.Time
	Activity  time.Time
	Panes     []*Pane
	Active    int
}

type Pane struct {
	ID        int
	Index     int
	Command   []string
	CWD       string
	CreatedAt time.Time
	Activity  time.Time
	PTY       *os.File
	Process   *exec.Cmd
	History   *Ring
	Exited    bool
	ExitState string
	Width     int
	Height    int
}

type Client struct {
	ID          int64
	SessionName string
	Width       int
	Height      int
	Prefix      bool
	ReadOnly    bool
}

func NewServer(socketPath string) *Server {
	return &Server{
		Sessions:   make(map[string]*Session),
		Clients:    make(map[int64]*Client),
		SocketPath: socketPath,
		StartedAt:  time.Now(),
	}
}

func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" && filepath.IsAbs(shell) {
		if st, err := os.Stat(shell); err == nil && !st.IsDir() {
			return shell
		}
	}
	return "/bin/sh"
}

func NormalizeCommand(args []string) []string {
	if len(args) == 0 {
		return []string{DefaultShell()}
	}
	return append([]string(nil), args...)
}

func CommandString(args []string) string {
	if len(args) == 0 {
		return DefaultShell()
	}
	return strings.Join(args, " ")
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
		ID:        s.NextSessionID,
		Name:      name,
		CWD:       cwd,
		CreatedAt: time.Now(),
		Activity:  time.Now(),
	}
	s.NextSessionID++
	s.Sessions[name] = session

	window := s.newWindowLocked(session, defaultWindowName(windowName, command))
	pane := s.newPaneLocked(window, cwd, command)
	return session, window, pane, nil
}

func (s *Server) NewWindow(sessionName, name, cwd string, command []string) (*Window, *Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	if cwd == "" {
		cwd = session.CWD
	}
	window := s.newWindowLocked(session, defaultWindowName(name, command))
	pane := s.newPaneLocked(window, cwd, command)
	session.Active = len(session.Windows) - 1
	session.Activity = time.Now()
	return window, pane, nil
}

func (s *Server) SplitPane(sessionName, cwd string, command []string) (*Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	window := session.ActiveWindow()
	if window == nil {
		return nil, fmt.Errorf("session has no windows: %s", sessionName)
	}
	if cwd == "" {
		cwd = session.CWD
	}
	pane := s.newPaneLocked(window, cwd, command)
	window.Active = len(window.Panes) - 1
	window.Activity = time.Now()
	session.Activity = time.Now()
	return pane, nil
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

func (s *Server) KillActivePane(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	window := session.ActiveWindow()
	if window == nil {
		return fmt.Errorf("session has no active window")
	}
	pane := window.ActivePane()
	if pane == nil {
		return fmt.Errorf("window has no active pane")
	}
	killPane(pane)
	window.Panes = append(window.Panes[:window.Active], window.Panes[window.Active+1:]...)
	reindexPanes(window)
	if len(window.Panes) == 0 {
		session.Windows = append(session.Windows[:session.Active], session.Windows[session.Active+1:]...)
		reindexWindows(session)
	}
	if window.Active >= len(window.Panes) {
		window.Active = len(window.Panes) - 1
	}
	if session.Active >= len(session.Windows) {
		session.Active = len(session.Windows) - 1
	}
	if len(session.Windows) == 0 {
		delete(s.Sessions, session.Name)
	}
	return nil
}

func (s *Server) KillActiveWindow(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	window := session.ActiveWindow()
	if window == nil {
		return fmt.Errorf("session has no active window")
	}
	for _, pane := range window.Panes {
		killPane(pane)
	}
	session.Windows = append(session.Windows[:session.Active], session.Windows[session.Active+1:]...)
	reindexWindows(session)
	if session.Active >= len(session.Windows) {
		session.Active = len(session.Windows) - 1
	}
	if len(session.Windows) == 0 {
		delete(s.Sessions, session.Name)
	}
	return nil
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

func (s *Server) RenameWindow(sessionName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	window := session.ActiveWindow()
	if window == nil {
		return fmt.Errorf("session has no active window")
	}
	window.Name = newName
	return nil
}

func (s *Server) SelectWindow(sessionName string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	if index < 0 || index >= len(session.Windows) {
		return fmt.Errorf("can't find window: %d", index)
	}
	session.Active = index
	session.Activity = time.Now()
	return nil
}

func (s *Server) SelectRelativeWindow(sessionName string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	if len(session.Windows) == 0 {
		return fmt.Errorf("session has no windows")
	}
	session.Active = (session.Active + delta + len(session.Windows)) % len(session.Windows)
	session.Activity = time.Now()
	return nil
}

func (s *Server) SelectRelativePane(sessionName string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	window := session.ActiveWindow()
	if window == nil || len(window.Panes) == 0 {
		return fmt.Errorf("window has no panes")
	}
	window.Active = (window.Active + delta + len(window.Panes)) % len(window.Panes)
	window.Activity = time.Now()
	session.Activity = time.Now()
	return nil
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

func (s *Session) ActiveWindow() *Window {
	if s == nil || s.Active < 0 || s.Active >= len(s.Windows) {
		return nil
	}
	return s.Windows[s.Active]
}

func (w *Window) ActivePane() *Pane {
	if w == nil || w.Active < 0 || w.Active >= len(w.Panes) {
		return nil
	}
	return w.Panes[w.Active]
}

func (s *Server) newWindowLocked(session *Session, name string) *Window {
	if name == "" {
		name = DefaultShellName()
	}
	window := &Window{
		ID:        s.NextWindowID,
		Index:     len(session.Windows),
		Name:      name,
		CreatedAt: time.Now(),
		Activity:  time.Now(),
	}
	s.NextWindowID++
	session.Windows = append(session.Windows, window)
	return window
}

func defaultWindowName(name string, command []string) string {
	if name != "" {
		return name
	}
	if len(command) == 0 {
		return ""
	}
	base := filepath.Base(command[0])
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return strings.TrimPrefix(base, "-")
}

func (s *Server) newPaneLocked(window *Window, cwd string, command []string) *Pane {
	pane := &Pane{
		ID:        s.NextPaneID,
		Index:     len(window.Panes),
		Command:   NormalizeCommand(command),
		CWD:       cwd,
		CreatedAt: time.Now(),
		Activity:  time.Now(),
		History:   NewRing(HistoryBytes),
	}
	s.NextPaneID++
	window.Panes = append(window.Panes, pane)
	return pane
}

func DefaultShellName() string {
	base := filepath.Base(DefaultShell())
	if base == "." || base == "/" || base == "" {
		return "shell"
	}
	return strings.TrimPrefix(base, "-")
}

func killPane(pane *Pane) {
	if pane == nil {
		return
	}
	if pane.PTY != nil {
		_ = pane.PTY.Close()
	}
	if pane.Process != nil && pane.Process.Process != nil {
		_ = pane.Process.Process.Kill()
	}
	pane.Exited = true
}

func reindexPanes(window *Window) {
	for i, pane := range window.Panes {
		pane.Index = i
	}
}

func reindexWindows(session *Session) {
	for i, window := range session.Windows {
		window.Index = i
	}
}
