package model

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const HistoryBytes = 2 << 20

type Server struct {
	mu                  sync.RWMutex
	Sessions            map[string]*Session
	Clients             map[int64]*Client
	Buffers             map[string]*Buffer
	GlobalOptions       map[string]string
	GlobalWindowOptions map[string]string
	GlobalEnvironment   map[string]string
	KeyBindings         map[string]map[string]KeyBinding
	NextSessionID       int
	NextWindowID        int
	NextPaneID          int
	NextClientID        int64
	NextBufferID        int
	NextBufferOrder     int64
	SocketPath          string
	StartedAt           time.Time
}

type Session struct {
	ID          int
	Name        string
	CWD         string
	CreatedAt   time.Time
	Activity    time.Time
	Windows     []*Window
	Active      int
	Attached    int
	Options     map[string]string
	Environment map[string]string
}

type Window struct {
	ID        int
	Index     int
	Name      string
	CreatedAt time.Time
	Activity  time.Time
	Panes     []*Pane
	Active    int
	Width     int
	Height    int
	Layout    *LayoutNode
	Options   map[string]string
}

type Pane struct {
	ID        int
	Index     int
	Command   []string
	Env       []string
	CWD       string
	Left      int
	Top       int
	Width     int
	Height    int
	CreatedAt time.Time
	Activity  time.Time
	PTY       *os.File
	Process   *exec.Cmd
	History   *Ring
	Exited    bool
	ExitState string
}

type LayoutNode struct {
	Orientation string
	PaneID      int
	Children    []*LayoutNode
	Left        int
	Top         int
	Width       int
	Height      int
}

type Client struct {
	ID          int64
	SessionName string
	Width       int
	Height      int
	Prefix      bool
	ReadOnly    bool
}

type KeyBinding struct {
	Table   string
	Key     string
	Command []string
	Note    string
	Repeat  bool
}

type Buffer struct {
	Name      string
	Data      string
	CreatedAt time.Time
	Order     int64
}

func NewServer(socketPath string) *Server {
	return &Server{
		Sessions:            make(map[string]*Session),
		Clients:             make(map[int64]*Client),
		Buffers:             make(map[string]*Buffer),
		GlobalOptions:       defaultOptions(),
		GlobalWindowOptions: defaultWindowOptions(),
		GlobalEnvironment:   environmentMap(os.Environ()),
		KeyBindings:         defaultKeyBindings(),
		SocketPath:          socketPath,
		StartedAt:           time.Now(),
	}
}

func defaultOptions() map[string]string {
	return map[string]string{
		"base-index":      "0",
		"default-command": "",
		"default-shell":   DefaultShell(),
		"escape-time":     "500",
		"prefix":          "C-b",
		"status":          "on",
	}
}

func defaultWindowOptions() map[string]string {
	return map[string]string{
		"history-limit":   "2000",
		"mode-keys":       "emacs",
		"pane-base-index": "0",
	}
}

func defaultKeyBindings() map[string]map[string]KeyBinding {
	bindings := make(map[string]map[string]KeyBinding)
	add := func(table, key string, command ...string) {
		if bindings[table] == nil {
			bindings[table] = make(map[string]KeyBinding)
		}
		bindings[table][key] = KeyBinding{Table: table, Key: key, Command: command}
	}
	add("prefix", "C-b", "send-prefix")
	add("prefix", `"`, "split-window")
	add("prefix", "%", "split-window", "-h")
	add("prefix", "c", "new-window")
	add("prefix", "d", "detach-client")
	add("prefix", "n", "next-window")
	add("prefix", "p", "previous-window")
	add("prefix", "o", "select-pane", "-t", ":.+")
	add("prefix", "x", "kill-pane")
	for i := 0; i <= 9; i++ {
		key := fmt.Sprintf("%d", i)
		add("prefix", key, "select-window", "-t", ":"+key)
	}
	return bindings
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
	pane := s.newPaneLocked(session, window, cwd, command)
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
	pane := s.newPaneLocked(session, window, cwd, command)
	session.Active = len(session.Windows) - 1
	session.Activity = time.Now()
	return window, pane, nil
}

func (s *Server) SplitPane(sessionName, cwd string, command []string) (*Pane, error) {
	return s.SplitPaneWithLayout(sessionName, cwd, command, "vertical")
}

func (s *Server) SplitPaneWithLayout(sessionName, cwd string, command []string, orientation string) (*Pane, error) {
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
	active := window.ActivePane()
	pane := s.newPaneLocked(session, window, cwd, command)
	if active != nil && active.ID != pane.ID {
		window.splitLeaf(active.ID, pane.ID, orientation)
	}
	window.Active = len(window.Panes) - 1
	window.recalculateLayout()
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
	s.killPaneAtLocked(session, window, window.Active)
	return nil
}

func (s *Server) KillPaneByID(paneID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.Sessions {
		for _, window := range session.Windows {
			for index, pane := range window.Panes {
				if pane.ID == paneID {
					s.killPaneAtLocked(session, window, index)
					return nil
				}
			}
		}
	}
	return fmt.Errorf("can't find pane: %d", paneID)
}

func (s *Server) killPaneAtLocked(session *Session, window *Window, paneIndex int) {
	pane := window.Panes[paneIndex]
	killPane(pane)
	window.Panes = append(window.Panes[:paneIndex], window.Panes[paneIndex+1:]...)
	window.removePaneFromLayout(pane.ID)
	reindexPanes(window)
	if len(window.Panes) == 0 {
		for index, candidate := range session.Windows {
			if candidate.ID == window.ID {
				session.Windows = append(session.Windows[:index], session.Windows[index+1:]...)
				break
			}
		}
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
	s.killWindowAtLocked(session, session.Active)
	return nil
}

func (s *Server) KillWindow(sessionName string, windowIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for index, window := range session.Windows {
		if window.Index == windowIndex {
			s.killWindowAtLocked(session, index)
			return nil
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
}

func (s *Server) killWindowAtLocked(session *Session, windowIndex int) {
	window := session.Windows[windowIndex]
	for _, pane := range window.Panes {
		killPane(pane)
	}
	session.Windows = append(session.Windows[:windowIndex], session.Windows[windowIndex+1:]...)
	reindexWindows(session)
	if session.Active >= len(session.Windows) {
		session.Active = len(session.Windows) - 1
	}
	if len(session.Windows) == 0 {
		delete(s.Sessions, session.Name)
	}
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

func (s *Server) RenameWindowByIndex(sessionName string, windowIndex int, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			window.Name = newName
			return nil
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
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

func (s *Server) SelectPaneByID(paneID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.Sessions {
		for windowIndex, window := range session.Windows {
			for paneIndex, pane := range window.Panes {
				if pane.ID == paneID {
					session.Active = windowIndex
					window.Active = paneIndex
					window.Activity = time.Now()
					session.Activity = time.Now()
					return nil
				}
			}
		}
	}
	return fmt.Errorf("can't find pane: %d", paneID)
}

func (s *Server) SetActiveWindowSize(sessionName string, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return
	}
	window := session.ActiveWindow()
	if window == nil {
		return
	}
	if width > 0 {
		window.Width = width
	}
	if height > 0 {
		window.Height = height
	}
	window.recalculateLayout()
}

func (s *Server) ActiveWindowPanes(sessionName string) []*Pane {
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
	out := make([]*Pane, len(window.Panes))
	copy(out, window.Panes)
	return out
}

func (s *Server) ResizeActivePane(sessionName, direction string, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if amount <= 0 {
		amount = 1
	}
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
	if resizeLayout(window.Layout, pane.ID, direction, amount) {
		window.recalculateLayout()
	}
	return nil
}

func (s *Server) SelectEvenLayout(sessionName, layout string) error {
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
	if len(window.Panes) == 0 {
		return nil
	}
	if len(window.Panes) == 1 {
		window.Layout = &LayoutNode{PaneID: window.Panes[0].ID}
		window.recalculateLayout()
		return nil
	}
	orientation := "horizontal"
	if layout == "even-vertical" {
		orientation = "vertical"
	}
	root := &LayoutNode{Orientation: orientation}
	for _, pane := range window.Panes {
		root.Children = append(root.Children, &LayoutNode{PaneID: pane.ID})
	}
	window.Layout = root
	window.recalculateLayout()
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

func (s *Server) SetOption(scope, sessionName, name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case "global":
		s.GlobalOptions[name] = value
	case "global-window":
		s.GlobalWindowOptions[name] = value
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		if session.Options == nil {
			session.Options = make(map[string]string)
		}
		session.Options[name] = value
	case "window":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if window == nil {
			return fmt.Errorf("session has no active window")
		}
		if window.Options == nil {
			window.Options = make(map[string]string)
		}
		window.Options[name] = value
	default:
		return fmt.Errorf("unknown option scope: %s", scope)
	}
	return nil
}

func (s *Server) Options(scope, sessionName string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string)
	copyOptions := func(src map[string]string) {
		for k, v := range src {
			out[k] = v
		}
	}
	switch scope {
	case "global":
		copyOptions(s.GlobalOptions)
	case "global-window":
		copyOptions(s.GlobalWindowOptions)
	case "session":
		copyOptions(s.GlobalOptions)
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		copyOptions(session.Options)
	case "window":
		copyOptions(s.GlobalWindowOptions)
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if window == nil {
			return nil, fmt.Errorf("session has no active window")
		}
		copyOptions(window.Options)
	default:
		return nil, fmt.Errorf("unknown option scope: %s", scope)
	}
	return out, nil
}

func (s *Server) GlobalOption(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.GlobalOptions[name]
}

func (s *Server) BindKey(table, key string, command []string, note string, repeat bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if table == "" {
		table = "prefix"
	}
	if s.KeyBindings[table] == nil {
		s.KeyBindings[table] = make(map[string]KeyBinding)
	}
	s.KeyBindings[table][key] = KeyBinding{
		Table:   table,
		Key:     key,
		Command: append([]string(nil), command...),
		Note:    note,
		Repeat:  repeat,
	}
}

func (s *Server) UnbindKey(table, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if table == "" {
		table = "prefix"
	}
	if s.KeyBindings[table] != nil {
		delete(s.KeyBindings[table], key)
	}
}

func (s *Server) KeyBinding(table, key string) (KeyBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if table == "" {
		table = "prefix"
	}
	binding, ok := s.KeyBindings[table][key]
	return binding, ok
}

func (s *Server) ListKeyBindings(table string) []KeyBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []KeyBinding
	appendTable := func(tableName string, tableBindings map[string]KeyBinding) {
		for _, binding := range tableBindings {
			binding.Table = tableName
			out = append(out, binding)
		}
	}
	if table != "" {
		appendTable(table, s.KeyBindings[table])
	} else {
		for tableName, tableBindings := range s.KeyBindings {
			appendTable(tableName, tableBindings)
		}
	}
	return out
}

func (s *Server) SetEnvironment(scope, sessionName, name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case "global":
		s.GlobalEnvironment[name] = value
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		if session.Environment == nil {
			session.Environment = make(map[string]string)
		}
		session.Environment[name] = value
	default:
		return fmt.Errorf("unknown environment scope: %s", scope)
	}
	return nil
}

func (s *Server) UnsetEnvironment(scope, sessionName, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case "global":
		delete(s.GlobalEnvironment, name)
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		delete(session.Environment, name)
	default:
		return fmt.Errorf("unknown environment scope: %s", scope)
	}
	return nil
}

func (s *Server) SetBuffer(name, data string, appendData bool) Buffer {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Buffers == nil {
		s.Buffers = make(map[string]*Buffer)
	}
	if name == "" {
		for {
			name = fmt.Sprintf("buffer%d", s.NextBufferID)
			s.NextBufferID++
			if _, exists := s.Buffers[name]; !exists {
				break
			}
		}
	}
	buffer := s.Buffers[name]
	if buffer == nil {
		buffer = &Buffer{Name: name, CreatedAt: time.Now()}
		s.Buffers[name] = buffer
	}
	if appendData {
		buffer.Data += data
	} else {
		buffer.Data = data
	}
	s.NextBufferOrder++
	buffer.Order = s.NextBufferOrder
	return *buffer
}

func (s *Server) ShowBuffer(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffer := s.bufferLocked(name)
	if buffer == nil {
		return "", noBufferError(name)
	}
	return buffer.Data, nil
}

func (s *Server) DeleteBuffer(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	buffer := s.bufferLocked(name)
	if buffer == nil {
		return noBufferError(name)
	}
	delete(s.Buffers, buffer.Name)
	return nil
}

func (s *Server) ListBuffers() []Buffer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buffers := make([]Buffer, 0, len(s.Buffers))
	for _, buffer := range s.Buffers {
		buffers = append(buffers, *buffer)
	}
	sort.Slice(buffers, func(i, j int) bool {
		if buffers[i].Order == buffers[j].Order {
			return buffers[i].Name < buffers[j].Name
		}
		return buffers[i].Order > buffers[j].Order
	})
	return buffers
}

func (s *Server) bufferLocked(name string) *Buffer {
	if len(s.Buffers) == 0 {
		return nil
	}
	if name != "" {
		return s.Buffers[name]
	}
	var selected *Buffer
	for _, buffer := range s.Buffers {
		if selected == nil || buffer.Order > selected.Order {
			selected = buffer
		}
	}
	return selected
}

func noBufferError(name string) error {
	if name == "" {
		return fmt.Errorf("no buffers")
	}
	return fmt.Errorf("no buffer %s", name)
}

func (s *Server) Environment(scope, sessionName string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string)
	for key, value := range s.GlobalEnvironment {
		out[key] = value
	}
	if scope == "global" {
		return out, nil
	}
	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	for key, value := range session.Environment {
		out[key] = value
	}
	return out, nil
}

func environmentMap(values []string) map[string]string {
	env := make(map[string]string, len(values))
	for _, item := range values {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			env[name] = value
		}
	}
	return env
}

func environmentList(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
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
		Width:     80,
		Height:    24,
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

func (s *Server) newPaneLocked(session *Session, window *Window, cwd string, command []string) *Pane {
	pane := &Pane{
		ID:        s.NextPaneID,
		Index:     len(window.Panes),
		Command:   NormalizeCommand(command),
		Env:       s.environmentLocked(session),
		CWD:       cwd,
		CreatedAt: time.Now(),
		Activity:  time.Now(),
		History:   NewRing(HistoryBytes),
	}
	s.NextPaneID++
	window.Panes = append(window.Panes, pane)
	if window.Layout == nil {
		window.Layout = &LayoutNode{PaneID: pane.ID}
	}
	window.recalculateLayout()
	return pane
}

func (s *Server) environmentLocked(session *Session) []string {
	env := make(map[string]string, len(s.GlobalEnvironment)+len(session.Environment))
	for key, value := range s.GlobalEnvironment {
		env[key] = value
	}
	for key, value := range session.Environment {
		env[key] = value
	}
	return environmentList(env)
}

func (w *Window) splitLeaf(oldPaneID, newPaneID int, orientation string) {
	if orientation != "horizontal" {
		orientation = "vertical"
	}
	if w.Layout == nil {
		w.Layout = &LayoutNode{PaneID: oldPaneID}
	}
	w.Layout = splitLeaf(w.Layout, oldPaneID, newPaneID, orientation)
}

func splitLeaf(node *LayoutNode, oldPaneID, newPaneID int, orientation string) *LayoutNode {
	if node == nil {
		return &LayoutNode{PaneID: newPaneID}
	}
	if node.isLeaf() {
		if node.PaneID != oldPaneID {
			return node
		}
		return &LayoutNode{
			Orientation: orientation,
			Children: []*LayoutNode{
				{PaneID: oldPaneID},
				{PaneID: newPaneID},
			},
		}
	}
	for i, child := range node.Children {
		node.Children[i] = splitLeaf(child, oldPaneID, newPaneID, orientation)
	}
	return node
}

func (w *Window) removePaneFromLayout(paneID int) {
	w.Layout = removeLayoutPane(w.Layout, paneID)
	if w.Layout == nil && len(w.Panes) > 0 {
		w.Layout = &LayoutNode{PaneID: w.Panes[0].ID}
	}
	w.recalculateLayout()
}

func removeLayoutPane(node *LayoutNode, paneID int) *LayoutNode {
	if node == nil {
		return nil
	}
	if node.isLeaf() {
		if node.PaneID == paneID {
			return nil
		}
		return node
	}
	children := node.Children[:0]
	for _, child := range node.Children {
		if updated := removeLayoutPane(child, paneID); updated != nil {
			children = append(children, updated)
		}
	}
	if len(children) == 0 {
		return nil
	}
	if len(children) == 1 {
		return children[0]
	}
	node.Children = children
	return node
}

func (w *Window) recalculateLayout() {
	if w.Width <= 0 {
		w.Width = 80
	}
	if w.Height <= 0 {
		w.Height = 24
	}
	if w.Layout == nil && len(w.Panes) > 0 {
		w.Layout = &LayoutNode{PaneID: w.Panes[0].ID}
	}
	w.applyLayout(w.Layout, 0, 0, w.Width, w.Height)
}

func (w *Window) applyLayout(node *LayoutNode, left, top, width, height int) {
	if node == nil {
		return
	}
	node.Left = left
	node.Top = top
	node.Width = maxInt(0, width)
	node.Height = maxInt(0, height)
	if node.isLeaf() {
		if pane := w.paneByID(node.PaneID); pane != nil {
			pane.Left = node.Left
			pane.Top = node.Top
			pane.Width = node.Width
			pane.Height = node.Height
		}
		return
	}
	if len(node.Children) == 0 {
		return
	}
	if len(node.Children) == 1 {
		w.applyLayout(node.Children[0], left, top, width, height)
		return
	}
	if node.Orientation == "horizontal" {
		if len(node.Children) == 2 && node.Children[0].Width > 0 && node.Children[0].Width < width {
			firstWidth := node.Children[0].Width
			secondWidth := maxInt(0, width-firstWidth-1)
			w.applyLayout(node.Children[0], left, top, firstWidth, height)
			w.applyLayout(node.Children[1], left+firstWidth+1, top, secondWidth, height)
			return
		}
		available := maxInt(0, width-(len(node.Children)-1))
		x := left
		for i, child := range node.Children {
			childWidth := available / len(node.Children)
			if i < available%len(node.Children) {
				childWidth++
			}
			w.applyLayout(child, x, top, childWidth, height)
			x += childWidth + 1
		}
		return
	}
	if len(node.Children) == 2 && node.Children[0].Height > 0 && node.Children[0].Height < height {
		firstHeight := node.Children[0].Height
		secondHeight := maxInt(0, height-firstHeight-1)
		w.applyLayout(node.Children[0], left, top, width, firstHeight)
		w.applyLayout(node.Children[1], left, top+firstHeight+1, width, secondHeight)
		return
	}
	available := maxInt(0, height-(len(node.Children)-1))
	y := top
	for i, child := range node.Children {
		childHeight := available / len(node.Children)
		if i < available%len(node.Children) {
			childHeight++
		}
		w.applyLayout(child, left, y, width, childHeight)
		y += childHeight + 1
	}
}

func (w *Window) paneByID(id int) *Pane {
	for _, pane := range w.Panes {
		if pane.ID == id {
			return pane
		}
	}
	return nil
}

func (n *LayoutNode) isLeaf() bool {
	return n != nil && len(n.Children) == 0
}

func resizeLayout(node *LayoutNode, paneID int, direction string, amount int) bool {
	if node == nil || node.isLeaf() || len(node.Children) < 2 {
		return false
	}
	first := node.Children[0]
	second := node.Children[1]
	if containsPane(first, paneID) || containsPane(second, paneID) {
		inFirst := containsPane(first, paneID)
		if node.Orientation == "horizontal" && (direction == "L" || direction == "R") {
			if inFirst && direction == "R" {
				return shiftHorizontal(first, second, amount)
			}
			if inFirst && direction == "L" {
				return shiftHorizontal(first, second, -amount)
			}
			if !inFirst && direction == "L" {
				return shiftHorizontal(first, second, -amount)
			}
			if !inFirst && direction == "R" {
				return shiftHorizontal(first, second, amount)
			}
		}
		if node.Orientation == "vertical" && (direction == "U" || direction == "D") {
			if inFirst && direction == "D" {
				return shiftVertical(first, second, amount)
			}
			if inFirst && direction == "U" {
				return shiftVertical(first, second, -amount)
			}
			if !inFirst && direction == "U" {
				return shiftVertical(first, second, -amount)
			}
			if !inFirst && direction == "D" {
				return shiftVertical(first, second, amount)
			}
		}
	}
	return resizeLayout(first, paneID, direction, amount) || resizeLayout(second, paneID, direction, amount)
}

func shiftHorizontal(first, second *LayoutNode, amount int) bool {
	if first == nil || second == nil {
		return false
	}
	if amount > 0 {
		if second.Width <= amount+1 {
			return false
		}
		first.Width += amount
		second.Width -= amount
		return true
	}
	if amount < 0 {
		amount = -amount
		if first.Width <= amount+1 {
			return false
		}
		first.Width -= amount
		second.Width += amount
		return true
	}
	return false
}

func shiftVertical(first, second *LayoutNode, amount int) bool {
	if first == nil || second == nil {
		return false
	}
	if amount > 0 {
		if second.Height <= amount+1 {
			return false
		}
		first.Height += amount
		second.Height -= amount
		return true
	}
	if amount < 0 {
		amount = -amount
		if first.Height <= amount+1 {
			return false
		}
		first.Height -= amount
		second.Height += amount
		return true
	}
	return false
}

func containsPane(node *LayoutNode, paneID int) bool {
	if node == nil {
		return false
	}
	if node.isLeaf() {
		return node.PaneID == paneID
	}
	for _, child := range node.Children {
		if containsPane(child, paneID) {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
