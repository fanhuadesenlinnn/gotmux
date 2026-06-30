package model

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
}

type Window struct {
	ID         int
	Index      int
	Name       string
	CreatedAt  time.Time
	Activity   time.Time
	Panes      []*Pane
	Active     int
	LastPaneID int
	Width      int
	Height     int
	Layout     *LayoutNode
	LastLayout string
	Options    map[string]string
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
		"history-limit":            "2000",
		"main-pane-height":         "24",
		"main-pane-width":          "80",
		"mode-keys":                "emacs",
		"other-pane-height":        "0",
		"other-pane-width":         "0",
		"pane-base-index":          "0",
		"tiled-layout-max-columns": "0",
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
	add("prefix", ";", "last-pane")
	add("prefix", "c", "new-window")
	add("prefix", "d", "detach-client")
	add("prefix", "l", "last-window")
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

func (s *Server) NewWindow(sessionName, name, cwd string, command []string) (*Window, *Pane, error) {
	return s.NewWindowDetached(sessionName, name, cwd, command, false)
}

func (s *Server) NewWindowDetached(sessionName, name, cwd string, command []string, detached bool) (*Window, *Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	if cwd == "" {
		cwd = session.CWD
	}
	if active := session.ActiveWindow(); active != nil && !detached {
		session.LastWindowID = active.ID
	}
	window := s.newWindowLocked(session, defaultWindowName(name, command))
	pane := s.newPaneLocked(session, window, cwd, command)
	if !detached || session.ActiveWindow() == nil {
		session.Active = len(session.Windows) - 1
	}
	session.Activity = time.Now()
	return window, pane, nil
}

func (s *Server) SplitPane(sessionName, cwd string, command []string) (*Pane, error) {
	return s.SplitPaneWithLayout(sessionName, cwd, command, "vertical")
}

func (s *Server) SplitPaneWithLayout(sessionName, cwd string, command []string, orientation string) (*Pane, error) {
	return s.SplitPaneWithLayoutDetached(sessionName, cwd, command, orientation, false)
}

func (s *Server) SplitPaneWithLayoutDetached(sessionName, cwd string, command []string, orientation string, detached bool) (*Pane, error) {
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
	return s.splitPaneInWindowLocked(session, window, cwd, command, orientation, detached), nil
}

func (s *Server) SplitPaneWithLayoutByIndex(sessionName string, windowIndex int, cwd string, command []string, orientation string) (*Pane, error) {
	return s.SplitPaneWithLayoutByIndexDetached(sessionName, windowIndex, cwd, command, orientation, false)
}

func (s *Server) SplitPaneWithLayoutByIndexDetached(sessionName string, windowIndex int, cwd string, command []string, orientation string, detached bool) (*Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			return s.splitPaneInWindowLocked(session, window, cwd, command, orientation, detached), nil
		}
	}
	return nil, fmt.Errorf("can't find window: %d", windowIndex)
}

func (s *Server) splitPaneInWindowLocked(session *Session, window *Window, cwd string, command []string, orientation string, detached bool) *Pane {
	if cwd == "" {
		cwd = session.CWD
	}
	active := window.ActivePane()
	activeID := -1
	activeIndex := window.Active
	if active != nil {
		activeID = active.ID
	}
	pane := s.newPaneLocked(session, window, cwd, command)
	if active != nil && activeIndex >= 0 && activeIndex < len(window.Panes)-1 {
		last := len(window.Panes) - 1
		copy(window.Panes[activeIndex+2:], window.Panes[activeIndex+1:last])
		window.Panes[activeIndex+1] = pane
		reindexPanes(window)
	}
	if active != nil && active.ID != pane.ID {
		window.splitLeaf(active.ID, pane.ID, orientation)
	}
	if detached && activeID != -1 {
		setActivePaneByID(window, activeID)
	} else {
		selectPaneByID(window, pane.ID)
	}
	window.recalculateLayout()
	window.Activity = time.Now()
	session.Activity = time.Now()
	return pane
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

func (s *Server) KillOtherPanesByID(paneID int) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return nil, fmt.Errorf("can't find pane: %d", paneID)
	}
	killed := make([]int, 0, len(location.window.Panes)-1)
	for index := len(location.window.Panes) - 1; index >= 0; index-- {
		pane := location.window.Panes[index]
		if pane.ID == paneID {
			continue
		}
		killed = append(killed, pane.ID)
		s.killPaneAtLocked(location.session, location.window, index)
	}
	setActivePaneByID(location.window, paneID)
	location.window.recalculateLayout()
	location.window.Activity = time.Now()
	location.session.Activity = time.Now()
	return killed, nil
}

func (s *Server) BreakPaneByID(paneID int, name string, detached bool) (*Session, *Window, *Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return nil, nil, nil, fmt.Errorf("can't find pane: %d", paneID)
	}
	if len(location.window.Panes) <= 1 {
		return nil, nil, nil, fmt.Errorf("can't break pane from a single-pane window")
	}

	pane := location.pane
	sourceWindow := location.window
	sourceActiveID := -1
	if active := sourceWindow.ActivePane(); active != nil {
		sourceActiveID = active.ID
	}
	sourceWindow.Panes = append(sourceWindow.Panes[:location.paneIndex], sourceWindow.Panes[location.paneIndex+1:]...)
	sourceWindow.removePaneFromLayout(pane.ID)
	reindexPanes(sourceWindow)
	if !setActivePaneByID(sourceWindow, sourceActiveID) {
		sourceWindow.Active = clampedPaneIndex(sourceWindow, sourceWindow.Active)
	}
	sourceWindow.Activity = time.Now()

	window := s.newWindowLocked(location.session, defaultWindowName(name, pane.Command))
	window.Width = sourceWindow.Width
	window.Height = sourceWindow.Height
	pane.Index = 0
	window.Panes = []*Pane{pane}
	window.Active = 0
	window.Layout = &LayoutNode{PaneID: pane.ID}
	window.recalculateLayout()
	window.Activity = time.Now()
	if !detached {
		location.session.Active = len(location.session.Windows) - 1
	}
	location.session.Activity = time.Now()
	return location.session, window, pane, nil
}

func (s *Server) JoinPaneByID(sourceID int, targetID int, orientation string, detached bool) (*Session, *Window, *Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, ok := s.paneLocationLocked(sourceID)
	if !ok {
		return nil, nil, nil, fmt.Errorf("can't find pane: %d", sourceID)
	}
	target, ok := s.paneLocationLocked(targetID)
	if !ok {
		return nil, nil, nil, fmt.Errorf("can't find pane: %d", targetID)
	}
	if source.pane.ID == target.pane.ID {
		return nil, nil, nil, fmt.Errorf("source and target panes must be different")
	}
	if source.window == target.window {
		return nil, nil, nil, fmt.Errorf("source and target panes must be in different windows")
	}
	if orientation != "horizontal" {
		orientation = "vertical"
	}

	sourcePane := source.pane
	sourceActiveID := -1
	if active := source.window.ActivePane(); active != nil {
		sourceActiveID = active.ID
	}
	source.window.Panes = append(source.window.Panes[:source.paneIndex], source.window.Panes[source.paneIndex+1:]...)
	source.window.removePaneFromLayout(sourcePane.ID)
	reindexPanes(source.window)
	if !setActivePaneByID(source.window, sourceActiveID) {
		source.window.Active = clampedPaneIndex(source.window, source.window.Active)
	}

	insertIndex := target.paneIndex + 1
	if insertIndex > len(target.window.Panes) {
		insertIndex = len(target.window.Panes)
	}
	target.window.Panes = append(target.window.Panes, nil)
	copy(target.window.Panes[insertIndex+1:], target.window.Panes[insertIndex:])
	target.window.Panes[insertIndex] = sourcePane
	reindexPanes(target.window)
	target.window.splitLeaf(target.pane.ID, sourcePane.ID, orientation)
	target.window.recalculateLayout()
	if !detached {
		setActivePaneByID(target.window, sourcePane.ID)
	}

	if len(source.window.Panes) == 0 {
		removeWindowLocked(source.session, source.window)
	} else {
		source.window.recalculateLayout()
		source.window.Activity = time.Now()
	}
	if !detached {
		setActiveWindowByID(target.session, target.window.ID)
	}
	target.window.Activity = time.Now()
	source.session.Activity = time.Now()
	target.session.Activity = time.Now()
	return target.session, target.window, sourcePane, nil
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

func (s *Server) KillOtherWindows(sessionName string, windowIndex int) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	keepIndex := windowSliceIndex(session, windowIndex)
	if keepIndex == -1 {
		return nil, fmt.Errorf("can't find window: %d", windowIndex)
	}
	keepID := session.Windows[keepIndex].ID
	killed := make([]int, 0)
	kept := make([]*Window, 0, 1)
	for _, window := range session.Windows {
		if window.ID == keepID {
			kept = append(kept, window)
		} else {
			for _, pane := range window.Panes {
				killed = append(killed, pane.ID)
				killPane(pane)
			}
		}
	}
	session.Windows = kept
	session.Active = 0
	session.Activity = time.Now()
	return killed, nil
}

func (s *Server) SwapWindows(sourceSessionName string, sourceWindowIndex int, targetSessionName string, targetWindowIndex int, detached bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceSession := s.Sessions[sourceSessionName]
	if sourceSession == nil {
		return fmt.Errorf("can't find session: %s", sourceSessionName)
	}
	targetSession := s.Sessions[targetSessionName]
	if targetSession == nil {
		return fmt.Errorf("can't find session: %s", targetSessionName)
	}
	sourceIndex := windowSliceIndex(sourceSession, sourceWindowIndex)
	if sourceIndex == -1 {
		return fmt.Errorf("can't find window: %d", sourceWindowIndex)
	}
	targetIndex := windowSliceIndex(targetSession, targetWindowIndex)
	if targetIndex == -1 {
		return fmt.Errorf("can't find window: %d", targetWindowIndex)
	}
	if sourceSession.Windows[sourceIndex].ID == targetSession.Windows[targetIndex].ID {
		return nil
	}

	sourceActive := sourceSession.Active
	targetActive := targetSession.Active
	sourceSession.Windows[sourceIndex], targetSession.Windows[targetIndex] = targetSession.Windows[targetIndex], sourceSession.Windows[sourceIndex]
	reindexWindows(sourceSession)
	if sourceSession != targetSession {
		reindexWindows(targetSession)
	}
	if detached {
		if sourceSession == targetSession {
			sourceSession.Active = targetIndex
		} else {
			sourceSession.Active = sourceIndex
			targetSession.Active = targetIndex
		}
	} else {
		sourceSession.Active = clampedWindowIndex(sourceSession, sourceActive)
		if sourceSession != targetSession {
			targetSession.Active = clampedWindowIndex(targetSession, targetActive)
		}
	}
	sourceSession.Activity = time.Now()
	targetSession.Activity = time.Now()
	return nil
}

func (s *Server) MoveWindow(sourceSessionName string, sourceWindowIndex int, targetSessionName string, targetWindowIndex int, detached bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceSession := s.Sessions[sourceSessionName]
	if sourceSession == nil {
		return fmt.Errorf("can't find session: %s", sourceSessionName)
	}
	targetSession := s.Sessions[targetSessionName]
	if targetSession == nil {
		return fmt.Errorf("can't find session: %s", targetSessionName)
	}
	sourceIndex := windowSliceIndex(sourceSession, sourceWindowIndex)
	if sourceIndex == -1 {
		return fmt.Errorf("can't find window: %d", sourceWindowIndex)
	}
	if targetIndex := windowSliceIndex(targetSession, targetWindowIndex); targetIndex != -1 {
		if sourceSession.Windows[sourceIndex].ID == targetSession.Windows[targetIndex].ID {
			return nil
		}
		return fmt.Errorf("index in use: %d", targetWindowIndex)
	}

	sourceActiveID := activeWindowID(sourceSession)
	targetActiveID := activeWindowID(targetSession)
	window := sourceSession.Windows[sourceIndex]
	sourceSession.Windows = append(sourceSession.Windows[:sourceIndex], sourceSession.Windows[sourceIndex+1:]...)
	window.Index = targetWindowIndex
	targetSession.Windows = append(targetSession.Windows, window)
	sort.Slice(targetSession.Windows, func(i, j int) bool {
		return targetSession.Windows[i].Index < targetSession.Windows[j].Index
	})

	if detached {
		if !setActiveWindowByID(sourceSession, sourceActiveID) {
			sourceSession.Active = clampedWindowIndex(sourceSession, sourceSession.Active)
		}
		if sourceSession != targetSession && !setActiveWindowByID(targetSession, targetActiveID) {
			targetSession.Active = clampedWindowIndex(targetSession, targetSession.Active)
		}
	} else {
		setActiveWindowByID(targetSession, window.ID)
		if sourceSession != targetSession && !setActiveWindowByID(sourceSession, sourceActiveID) {
			sourceSession.Active = clampedWindowIndex(sourceSession, sourceSession.Active)
		}
	}
	sourceSession.Activity = time.Now()
	targetSession.Activity = time.Now()
	return nil
}

func (s *Server) RenumberWindows(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	activeID := activeWindowID(session)
	sort.Slice(session.Windows, func(i, j int) bool {
		return session.Windows[i].Index < session.Windows[j].Index
	})
	reindexWindows(session)
	if !setActiveWindowByID(session, activeID) {
		session.Active = clampedWindowIndex(session, session.Active)
	}
	session.Activity = time.Now()
	return nil
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
	sliceIndex := windowSliceIndex(session, index)
	if sliceIndex == -1 {
		return fmt.Errorf("can't find window: %d", index)
	}
	selectWindowSliceIndex(session, sliceIndex)
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
	selectWindowSliceIndex(session, (session.Active+delta+len(session.Windows))%len(session.Windows))
	session.Activity = time.Now()
	return nil
}

func (s *Server) SelectLastWindow(sessionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	if session.LastWindowID < 0 {
		return fmt.Errorf("no last window")
	}
	for index, window := range session.Windows {
		if window.ID == session.LastWindowID {
			selectWindowSliceIndex(session, index)
			session.Activity = time.Now()
			return nil
		}
	}
	return fmt.Errorf("no last window")
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
	selectPaneIndex(window, (window.Active+delta+len(window.Panes))%len(window.Panes))
	window.Activity = time.Now()
	session.Activity = time.Now()
	return nil
}

func (s *Server) SelectPaneByID(paneID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return fmt.Errorf("can't find pane: %d", paneID)
	}
	selectWindowSliceIndex(location.session, location.windowIndex)
	selectPaneIndex(location.window, location.paneIndex)
	location.window.Activity = time.Now()
	location.session.Activity = time.Now()
	return nil
}

func (s *Server) SelectPaneDirectionFrom(paneID int, direction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return fmt.Errorf("can't find pane: %d", paneID)
	}
	target := directionalPane(location.window, location.pane, direction)
	if target == nil {
		return nil
	}
	for paneIndex, pane := range location.window.Panes {
		if pane.ID == target.ID {
			selectWindowSliceIndex(location.session, location.windowIndex)
			selectPaneIndex(location.window, paneIndex)
			location.window.Activity = time.Now()
			location.session.Activity = time.Now()
			return nil
		}
	}
	return nil
}

func (s *Server) SelectLastPaneByIndex(sessionName string, windowIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	windowSlice := windowSliceIndex(session, windowIndex)
	if windowSlice == -1 {
		return fmt.Errorf("can't find window: %d", windowIndex)
	}
	window := session.Windows[windowSlice]
	targetIndex := -1
	for index, pane := range window.Panes {
		if pane.ID == window.LastPaneID {
			targetIndex = index
			break
		}
	}
	if targetIndex == -1 && len(window.Panes) == 2 {
		if window.Active == 0 {
			targetIndex = 1
		} else {
			targetIndex = 0
		}
	}
	if targetIndex == -1 {
		return fmt.Errorf("no last pane")
	}
	selectWindowSliceIndex(session, windowSlice)
	selectPaneIndex(window, targetIndex)
	window.Activity = time.Now()
	session.Activity = time.Now()
	return nil
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
	window.resizeTo(width, height)
}

func (s *Server) ResizeWindowByIndex(sessionName string, windowIndex int, width, height int, direction string, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	sliceIndex := windowSliceIndex(session, windowIndex)
	if sliceIndex == -1 {
		return fmt.Errorf("can't find window: %d", windowIndex)
	}
	window := session.Windows[sliceIndex]
	nextWidth := window.Width
	nextHeight := window.Height
	if width > 0 {
		nextWidth = width
	}
	if height > 0 {
		nextHeight = height
	}
	if amount <= 0 {
		amount = 1
	}
	switch direction {
	case "L":
		nextWidth = maxInt(1, nextWidth-amount)
	case "R":
		nextWidth += amount
	case "U":
		nextHeight = maxInt(1, nextHeight-amount)
	case "D":
		nextHeight += amount
	}
	window.resizeTo(nextWidth, nextHeight)
	window.Activity = time.Now()
	session.Activity = time.Now()
	return nil
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

func (s *Server) ResizePaneByID(paneID int, direction string, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if amount <= 0 {
		amount = 1
	}
	for _, session := range s.Sessions {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if pane.ID == paneID {
					if resizeLayout(window.Layout, pane.ID, direction, amount) {
						window.recalculateLayout()
					}
					return nil
				}
			}
		}
	}
	return fmt.Errorf("can't find pane: %d", paneID)
}

func (s *Server) SwapPanesByID(sourceID int, targetID int, detached bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, ok := s.paneLocationLocked(sourceID)
	if !ok {
		return fmt.Errorf("can't find pane: %d", sourceID)
	}
	target, ok := s.paneLocationLocked(targetID)
	if !ok {
		return fmt.Errorf("can't find pane: %d", targetID)
	}
	if source.pane.ID == target.pane.ID {
		return nil
	}

	sameWindow := source.window == target.window
	sourceActiveIndex := source.window.Active
	targetActiveIndex := target.window.Active

	source.window.Panes[source.paneIndex], target.window.Panes[target.paneIndex] = target.pane, source.pane
	swapLayoutPaneIDs(source.window.Layout, sourceID, targetID)
	if !sameWindow {
		swapLayoutPaneIDs(target.window.Layout, sourceID, targetID)
	}
	renumberWindowPanes(source.window)
	if !sameWindow {
		renumberWindowPanes(target.window)
	}

	if detached {
		source.window.Active = clampedPaneIndex(source.window, sourceActiveIndex)
		if !sameWindow {
			target.window.Active = clampedPaneIndex(target.window, targetActiveIndex)
		}
	} else if sameWindow {
		setActivePaneByID(source.window, targetID)
	} else {
		setActivePaneByID(source.window, targetID)
		setActivePaneByID(target.window, sourceID)
	}

	source.window.recalculateLayout()
	source.window.Activity = time.Now()
	source.session.Activity = time.Now()
	if !sameWindow {
		target.window.recalculateLayout()
		target.window.Activity = time.Now()
		target.session.Activity = time.Now()
	}
	return nil
}

func (s *Server) RotateWindow(sessionName string, reverse bool) error {
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
	rotateWindowLocked(window, reverse)
	window.Activity = time.Now()
	session.Activity = time.Now()
	return nil
}

func (s *Server) RotateWindowByIndex(sessionName string, windowIndex int, reverse bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			rotateWindowLocked(window, reverse)
			window.Activity = time.Now()
			session.Activity = time.Now()
			return nil
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
}

func (s *Server) WindowPanesContainingPane(paneID int) []*Pane {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.Sessions {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if pane.ID == paneID {
					out := make([]*Pane, len(window.Panes))
					copy(out, window.Panes)
					return out
				}
			}
		}
	}
	return nil
}

func (s *Server) SelectEvenLayout(sessionName, layout string) error {
	return s.SelectLayout(sessionName, layout)
}

func (s *Server) SelectLayout(sessionName, layout string) error {
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
	if err := s.applyBuiltinLayoutLocked(window, layout); err != nil {
		return err
	}
	return nil
}

func (s *Server) SelectEvenLayoutByIndex(sessionName string, windowIndex int, layout string) error {
	return s.SelectLayoutByIndex(sessionName, windowIndex, layout)
}

func (s *Server) SelectLayoutByIndex(sessionName string, windowIndex int, layout string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			return s.applyBuiltinLayoutLocked(window, layout)
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
}

func (s *Server) SelectLastLayout(sessionName string) error {
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
	if window.LastLayout == "" {
		return nil
	}
	return s.applyBuiltinLayoutLocked(window, window.LastLayout)
}

func (s *Server) SelectLastLayoutByIndex(sessionName string, windowIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			if window.LastLayout == "" {
				return nil
			}
			return s.applyBuiltinLayoutLocked(window, window.LastLayout)
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
}

func (s *Server) SelectNextLayout(sessionName string) error {
	return s.selectRelativeLayout(sessionName, 1)
}

func (s *Server) SelectPreviousLayout(sessionName string) error {
	return s.selectRelativeLayout(sessionName, -1)
}

func (s *Server) SelectNextLayoutByIndex(sessionName string, windowIndex int) error {
	return s.selectRelativeLayoutByIndex(sessionName, windowIndex, 1)
}

func (s *Server) SelectPreviousLayoutByIndex(sessionName string, windowIndex int) error {
	return s.selectRelativeLayoutByIndex(sessionName, windowIndex, -1)
}

func (s *Server) selectRelativeLayout(sessionName string, delta int) error {
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
	return s.applyBuiltinLayoutLocked(window, relativeLayoutName(window.LastLayout, delta))
}

func (s *Server) selectRelativeLayoutByIndex(sessionName string, windowIndex int, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return fmt.Errorf("can't find session: %s", sessionName)
	}
	for _, window := range session.Windows {
		if window.Index == windowIndex {
			return s.applyBuiltinLayoutLocked(window, relativeLayoutName(window.LastLayout, delta))
		}
	}
	return fmt.Errorf("can't find window: %d", windowIndex)
}

var builtinLayouts = []string{
	"even-horizontal",
	"even-vertical",
	"main-horizontal",
	"main-horizontal-mirrored",
	"main-vertical",
	"main-vertical-mirrored",
	"tiled",
}

func ResolveLayoutName(name string) (string, bool) {
	for _, layout := range builtinLayouts {
		if name == layout {
			return layout, true
		}
	}
	matched := ""
	for _, layout := range builtinLayouts {
		if strings.HasPrefix(layout, name) {
			if matched != "" {
				return "", false
			}
			matched = layout
		}
	}
	return matched, matched != ""
}

func builtinLayoutIndex(name string) int {
	for i, layout := range builtinLayouts {
		if name == layout {
			return i
		}
	}
	return -1
}

func relativeLayoutName(current string, delta int) string {
	index := builtinLayoutIndex(current)
	if index == -1 {
		if delta < 0 {
			return builtinLayouts[len(builtinLayouts)-1]
		}
		return builtinLayouts[0]
	}
	index += delta
	if index < 0 {
		index = len(builtinLayouts) - 1
	}
	if index >= len(builtinLayouts) {
		index = 0
	}
	return builtinLayouts[index]
}

func (s *Server) applyBuiltinLayoutLocked(window *Window, layout string) error {
	requested := layout
	layout, ok := ResolveLayoutName(layout)
	if !ok {
		return fmt.Errorf("unsupported layout: %s", requested)
	}
	option := func(name string) string {
		value := s.GlobalWindowOptions[name]
		if window.Options != nil {
			if override, exists := window.Options[name]; exists {
				value = override
			}
		}
		return value
	}
	switch layout {
	case "even-horizontal", "even-vertical":
		applyEvenLayout(window, layout)
	case "main-horizontal":
		applyMainHorizontalLayout(window, false, option)
	case "main-horizontal-mirrored":
		applyMainHorizontalLayout(window, true, option)
	case "main-vertical":
		applyMainVerticalLayout(window, false, option)
	case "main-vertical-mirrored":
		applyMainVerticalLayout(window, true, option)
	case "tiled":
		applyTiledLayout(window, option)
	}
	window.LastLayout = layout
	return nil
}

func applyEvenLayout(window *Window, layout string) {
	if len(window.Panes) == 0 {
		return
	}
	if len(window.Panes) == 1 {
		window.Layout = &LayoutNode{PaneID: window.Panes[0].ID}
		window.recalculateLayout()
		return
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
}

func applyMainHorizontalLayout(window *Window, mirrored bool, option func(string) string) {
	if len(window.Panes) == 0 {
		return
	}
	if len(window.Panes) == 1 {
		window.Layout = &LayoutNode{PaneID: window.Panes[0].ID}
		window.recalculateLayout()
		return
	}
	mainHeight, otherHeight := mainLayoutSizes(
		window.Height,
		option("main-pane-height"),
		option("other-pane-height"),
		24,
	)
	main := &LayoutNode{PaneID: window.Panes[0].ID, Height: mainHeight}
	other := horizontalPaneGroup(window.Panes[1:], otherHeight)
	children := []*LayoutNode{main, other}
	if mirrored {
		children = []*LayoutNode{other, main}
	}
	window.Layout = &LayoutNode{Orientation: "vertical", Children: children}
	window.recalculateLayout()
}

func applyMainVerticalLayout(window *Window, mirrored bool, option func(string) string) {
	if len(window.Panes) == 0 {
		return
	}
	if len(window.Panes) == 1 {
		window.Layout = &LayoutNode{PaneID: window.Panes[0].ID}
		window.recalculateLayout()
		return
	}
	mainWidth, otherWidth := mainLayoutSizes(
		window.Width,
		option("main-pane-width"),
		option("other-pane-width"),
		80,
	)
	main := &LayoutNode{PaneID: window.Panes[0].ID, Width: mainWidth}
	other := verticalPaneGroup(window.Panes[1:], otherWidth)
	children := []*LayoutNode{main, other}
	if mirrored {
		children = []*LayoutNode{other, main}
	}
	window.Layout = &LayoutNode{Orientation: "horizontal", Children: children}
	window.recalculateLayout()
}

func mainLayoutSizes(total int, mainOption string, otherOption string, fallback int) (int, int) {
	available := maxInt(0, total-1)
	mainSize := layoutOptionSize(mainOption, fallback, available)
	if mainSize+1 >= available {
		if available <= 2 {
			mainSize = 1
		} else {
			mainSize = available - 1
		}
		return maxInt(0, mainSize), 1
	}
	otherSize := layoutOptionSize(otherOption, 0, available)
	if otherSize <= 0 || otherSize > available || available-otherSize < mainSize {
		otherSize = available - mainSize
	} else {
		mainSize = available - otherSize
	}
	return maxInt(0, mainSize), maxInt(0, otherSize)
}

func layoutOptionSize(value string, fallback int, total int) int {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "%") {
		percent, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		if err == nil {
			return total * percent / 100
		}
		return fallback
	}
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func horizontalPaneGroup(panes []*Pane, height int) *LayoutNode {
	if len(panes) == 1 {
		return &LayoutNode{PaneID: panes[0].ID, Height: height}
	}
	node := &LayoutNode{Orientation: "horizontal", Height: height}
	for _, pane := range panes {
		node.Children = append(node.Children, &LayoutNode{PaneID: pane.ID})
	}
	return node
}

func verticalPaneGroup(panes []*Pane, width int) *LayoutNode {
	if len(panes) == 1 {
		return &LayoutNode{PaneID: panes[0].ID, Width: width}
	}
	node := &LayoutNode{Orientation: "vertical", Width: width}
	for _, pane := range panes {
		node.Children = append(node.Children, &LayoutNode{PaneID: pane.ID})
	}
	return node
}

func applyTiledLayout(window *Window, option func(string) string) {
	if len(window.Panes) == 0 {
		return
	}
	if len(window.Panes) == 1 {
		window.Layout = &LayoutNode{PaneID: window.Panes[0].ID}
		window.recalculateLayout()
		return
	}
	paneCount := len(window.Panes)
	maxColumns := layoutOptionSize(option("tiled-layout-max-columns"), 0, paneCount)
	rows, columns := 1, 1
	for rows*columns < paneCount {
		rows++
		if rows*columns < paneCount && (maxColumns == 0 || columns < maxColumns) {
			columns++
		}
	}
	cellWidth := maxInt(1, (window.Width-(columns-1))/columns)
	cellHeight := maxInt(1, (window.Height-(rows-1))/rows)

	rowNodes := make([]*LayoutNode, 0, rows)
	for start := 0; start < paneCount; start += columns {
		end := start + columns
		if end > paneCount {
			end = paneCount
		}
		rowNodes = append(rowNodes, fixedHorizontalCells(window.Panes[start:end], cellWidth))
	}
	window.Layout = fixedVerticalRows(rowNodes, cellHeight)
	window.recalculateLayout()
}

func fixedHorizontalCells(panes []*Pane, width int) *LayoutNode {
	if len(panes) == 1 {
		return &LayoutNode{PaneID: panes[0].ID}
	}
	first := &LayoutNode{PaneID: panes[0].ID, Width: width}
	rest := fixedHorizontalCells(panes[1:], width)
	return &LayoutNode{Orientation: "horizontal", Children: []*LayoutNode{first, rest}}
}

func fixedVerticalRows(rows []*LayoutNode, height int) *LayoutNode {
	if len(rows) == 1 {
		return rows[0]
	}
	first := rows[0]
	first.Height = height
	rest := fixedVerticalRows(rows[1:], height)
	return &LayoutNode{Orientation: "vertical", Children: []*LayoutNode{first, rest}}
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

func (s *Server) RenameBuffer(name string, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newName == "" {
		return fmt.Errorf("empty buffer name")
	}
	buffer := s.bufferLocked(name)
	if buffer == nil {
		return noBufferError(name)
	}
	if _, exists := s.Buffers[newName]; exists {
		return fmt.Errorf("buffer already exists: %s", newName)
	}
	delete(s.Buffers, buffer.Name)
	buffer.Name = newName
	s.Buffers[newName] = buffer
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

type paneLocation struct {
	session     *Session
	window      *Window
	windowIndex int
	pane        *Pane
	paneIndex   int
}

func (s *Server) paneLocationLocked(paneID int) (paneLocation, bool) {
	for _, session := range s.Sessions {
		for windowIndex, window := range session.Windows {
			for paneIndex, pane := range window.Panes {
				if pane.ID == paneID {
					return paneLocation{session: session, window: window, windowIndex: windowIndex, pane: pane, paneIndex: paneIndex}, true
				}
			}
		}
	}
	return paneLocation{}, false
}

func setActivePaneByID(window *Window, paneID int) bool {
	for index, pane := range window.Panes {
		if pane.ID == paneID {
			window.Active = index
			return true
		}
	}
	return false
}

func selectPaneByID(window *Window, paneID int) bool {
	for index, pane := range window.Panes {
		if pane.ID == paneID {
			selectPaneIndex(window, index)
			return true
		}
	}
	return false
}

func selectPaneIndex(window *Window, index int) {
	if index < 0 || index >= len(window.Panes) {
		return
	}
	if active := window.ActivePane(); active != nil && active.ID != window.Panes[index].ID {
		window.LastPaneID = active.ID
	}
	window.Active = index
}

func directionalPane(window *Window, active *Pane, direction string) *Pane {
	if window == nil || active == nil {
		return nil
	}
	bestPrimary := int(^uint(0) >> 1)
	bestSecondary := int(^uint(0) >> 1)
	var best *Pane
	for _, pane := range window.Panes {
		if pane.ID == active.ID {
			continue
		}
		primary, secondary, ok := paneDirectionScore(active, pane, direction)
		if !ok {
			continue
		}
		if primary < bestPrimary || (primary == bestPrimary && secondary < bestSecondary) ||
			(primary == bestPrimary && secondary == bestSecondary && (best == nil || pane.Index < best.Index)) {
			bestPrimary = primary
			bestSecondary = secondary
			best = pane
		}
	}
	return best
}

func paneDirectionScore(active *Pane, candidate *Pane, direction string) (int, int, bool) {
	switch direction {
	case "L":
		if candidate.Left+candidate.Width > active.Left {
			return 0, 0, false
		}
		overlap := intervalOverlap(active.Top, active.Top+active.Height, candidate.Top, candidate.Top+candidate.Height)
		if overlap <= 0 {
			return 0, 0, false
		}
		return active.Left - (candidate.Left + candidate.Width), absInt(paneCenterY(active) - paneCenterY(candidate)), true
	case "R":
		if candidate.Left < active.Left+active.Width {
			return 0, 0, false
		}
		overlap := intervalOverlap(active.Top, active.Top+active.Height, candidate.Top, candidate.Top+candidate.Height)
		if overlap <= 0 {
			return 0, 0, false
		}
		return candidate.Left - (active.Left + active.Width), absInt(paneCenterY(active) - paneCenterY(candidate)), true
	case "U":
		if candidate.Top+candidate.Height > active.Top {
			return 0, 0, false
		}
		overlap := intervalOverlap(active.Left, active.Left+active.Width, candidate.Left, candidate.Left+candidate.Width)
		if overlap <= 0 {
			return 0, 0, false
		}
		return active.Top - (candidate.Top + candidate.Height), absInt(paneCenterX(active) - paneCenterX(candidate)), true
	case "D":
		if candidate.Top < active.Top+active.Height {
			return 0, 0, false
		}
		overlap := intervalOverlap(active.Left, active.Left+active.Width, candidate.Left, candidate.Left+candidate.Width)
		if overlap <= 0 {
			return 0, 0, false
		}
		return candidate.Top - (active.Top + active.Height), absInt(paneCenterX(active) - paneCenterX(candidate)), true
	default:
		return 0, 0, false
	}
}

func paneCenterX(pane *Pane) int {
	return pane.Left + pane.Width/2
}

func paneCenterY(pane *Pane) int {
	return pane.Top + pane.Height/2
}

func intervalOverlap(a0, a1, b0, b1 int) int {
	start := maxInt(a0, b0)
	end := minInt(a1, b1)
	if end <= start {
		return 0
	}
	return end - start
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampedPaneIndex(window *Window, index int) int {
	if len(window.Panes) == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= len(window.Panes) {
		return len(window.Panes) - 1
	}
	return index
}

func setActiveWindowByID(session *Session, windowID int) bool {
	for index, window := range session.Windows {
		if window.ID == windowID {
			session.Active = index
			return true
		}
	}
	return false
}

func activeWindowID(session *Session) int {
	if window := session.ActiveWindow(); window != nil {
		return window.ID
	}
	return -1
}

func selectWindowSliceIndex(session *Session, index int) {
	if index < 0 || index >= len(session.Windows) {
		return
	}
	if active := session.ActiveWindow(); active != nil && active.ID != session.Windows[index].ID {
		session.LastWindowID = active.ID
	}
	session.Active = index
}

func windowSliceIndex(session *Session, windowIndex int) int {
	for index, window := range session.Windows {
		if window.Index == windowIndex {
			return index
		}
	}
	return -1
}

func removeWindowLocked(session *Session, window *Window) {
	for index, candidate := range session.Windows {
		if candidate.ID == window.ID {
			session.Windows = append(session.Windows[:index], session.Windows[index+1:]...)
			break
		}
	}
	reindexWindows(session)
	session.Active = clampedWindowIndex(session, session.Active)
}

func clampedWindowIndex(session *Session, index int) int {
	if len(session.Windows) == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= len(session.Windows) {
		return len(session.Windows) - 1
	}
	return index
}

func renumberWindowPanes(window *Window) {
	for index, pane := range window.Panes {
		pane.Index = index
	}
	if len(window.Panes) == 0 {
		window.Active = 0
		return
	}
	if window.Active < 0 {
		window.Active = 0
	}
	if window.Active >= len(window.Panes) {
		window.Active = len(window.Panes) - 1
	}
}

func rotateWindowLocked(window *Window, reverse bool) {
	if len(window.Panes) <= 1 {
		return
	}
	activeID := -1
	if pane := window.ActivePane(); pane != nil {
		activeID = pane.ID
	}
	if reverse {
		last := window.Panes[len(window.Panes)-1]
		copy(window.Panes[1:], window.Panes[:len(window.Panes)-1])
		window.Panes[0] = last
	} else {
		first := window.Panes[0]
		copy(window.Panes, window.Panes[1:])
		window.Panes[len(window.Panes)-1] = first
	}
	rotateLayoutPaneIDs(window.Layout, reverse)
	renumberWindowPanes(window)
	activeIndex := 0
	for index, pane := range window.Panes {
		if pane.ID == activeID {
			activeIndex = index
			break
		}
	}
	if reverse {
		window.Active = (activeIndex - 1 + len(window.Panes)) % len(window.Panes)
	} else {
		window.Active = (activeIndex + 1) % len(window.Panes)
	}
	window.recalculateLayout()
}

func (s *Server) newWindowLocked(session *Session, name string) *Window {
	if name == "" {
		name = DefaultShellName()
	}
	window := &Window{
		ID:         s.NextWindowID,
		Index:      len(session.Windows),
		Name:       name,
		LastPaneID: -1,
		Width:      80,
		Height:     24,
		CreatedAt:  time.Now(),
		Activity:   time.Now(),
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

func swapLayoutPaneIDs(node *LayoutNode, sourceID int, targetID int) {
	if node == nil {
		return
	}
	if node.isLeaf() {
		switch node.PaneID {
		case sourceID:
			node.PaneID = targetID
		case targetID:
			node.PaneID = sourceID
		}
		return
	}
	for _, child := range node.Children {
		swapLayoutPaneIDs(child, sourceID, targetID)
	}
}

func rotateLayoutPaneIDs(node *LayoutNode, reverse bool) {
	leaves := layoutLeaves(node)
	if len(leaves) <= 1 {
		return
	}
	ids := make([]int, len(leaves))
	for i, leaf := range leaves {
		ids[i] = leaf.PaneID
	}
	if reverse {
		last := ids[len(ids)-1]
		copy(ids[1:], ids[:len(ids)-1])
		ids[0] = last
	} else {
		first := ids[0]
		copy(ids, ids[1:])
		ids[len(ids)-1] = first
	}
	for i, leaf := range leaves {
		leaf.PaneID = ids[i]
	}
}

func layoutLeaves(node *LayoutNode) []*LayoutNode {
	if node == nil {
		return nil
	}
	if node.isLeaf() {
		return []*LayoutNode{node}
	}
	var out []*LayoutNode
	for _, child := range node.Children {
		out = append(out, layoutLeaves(child)...)
	}
	return out
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

func (w *Window) resizeTo(width, height int) {
	oldWidth := w.Width
	oldHeight := w.Height
	if width <= 0 {
		width = oldWidth
	}
	if height <= 0 {
		height = oldHeight
	}
	scaleLayoutDimensions(w.Layout, oldWidth, oldHeight, width, height)
	w.Width = width
	w.Height = height
	w.recalculateLayout()
}

func scaleLayoutDimensions(node *LayoutNode, oldWidth, oldHeight, newWidth, newHeight int) {
	if node == nil {
		return
	}
	if oldWidth > 0 && newWidth > 0 && node.Width > 0 {
		node.Width = scaleDimension(node.Width, oldWidth, newWidth)
	}
	if oldHeight > 0 && newHeight > 0 && node.Height > 0 {
		node.Height = scaleDimension(node.Height, oldHeight, newHeight)
	}
	for _, child := range node.Children {
		scaleLayoutDimensions(child, oldWidth, oldHeight, newWidth, newHeight)
	}
}

func scaleDimension(value, oldSize, newSize int) int {
	if value <= 0 || oldSize <= 0 {
		return value
	}
	scaled := (value*newSize + oldSize/2) / oldSize
	return maxInt(1, scaled)
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
