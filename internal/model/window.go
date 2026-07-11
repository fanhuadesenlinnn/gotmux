package model

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Window struct {
	ID           int
	Index        int
	Name         string
	CreatedAt    time.Time
	Activity     time.Time
	Panes        []*Pane
	Active       int
	LastPaneID   int
	Width        int
	Height       int
	Layout       *LayoutNode
	LastLayout   string
	Zoomed       bool
	ZoomedPaneID int
	Options      map[string]string
	Hooks        map[string][]string
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

func (s *Server) UnlinkWindow(sessionName string, windowIndex int, force bool) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	index := windowSliceIndex(session, windowIndex)
	if index == -1 {
		return nil, fmt.Errorf("can't find window: %d", windowIndex)
	}
	if !force && s.windowLinkCountLocked(session.Windows[index].ID) <= 1 {
		return nil, fmt.Errorf("window only linked to one session")
	}
	killed := s.unlinkWindowAtLocked(session, index, true, true)
	session.Activity = time.Now()
	return killed, nil
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

func (s *Server) LinkWindow(sourceSessionName string, sourceWindowIndex int, targetSessionName string, targetWindowIndex int, detached bool, killTarget bool) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceSession := s.Sessions[sourceSessionName]
	if sourceSession == nil {
		return nil, fmt.Errorf("can't find session: %s", sourceSessionName)
	}
	targetSession := s.Sessions[targetSessionName]
	if targetSession == nil {
		return nil, fmt.Errorf("can't find session: %s", targetSessionName)
	}
	sourceIndex := windowSliceIndex(sourceSession, sourceWindowIndex)
	if sourceIndex == -1 {
		return nil, fmt.Errorf("can't find window: %d", sourceWindowIndex)
	}
	if targetIndex := windowSliceIndex(targetSession, targetWindowIndex); targetIndex != -1 {
		if sourceSession.Windows[sourceIndex].ID == targetSession.Windows[targetIndex].ID {
			return nil, nil
		}
		if !killTarget {
			return nil, fmt.Errorf("index in use: %d", targetWindowIndex)
		}
		killed := s.unlinkWindowAtLocked(targetSession, targetIndex, false, false)
		sourceActiveID := activeWindowID(sourceSession)
		targetActiveID := activeWindowID(targetSession)
		link := linkedWindowCopy(sourceSession.Windows[sourceIndex], targetWindowIndex)
		targetSession.Windows = append(targetSession.Windows, link)
		sort.Slice(targetSession.Windows, func(i, j int) bool {
			return targetSession.Windows[i].Index < targetSession.Windows[j].Index
		})
		if detached {
			if !setActiveWindowByID(targetSession, targetActiveID) {
				setActiveWindowByWindowIndex(targetSession, targetWindowIndex)
			}
		} else {
			setActiveWindowByWindowIndex(targetSession, targetWindowIndex)
		}
		if sourceSession != targetSession && !setActiveWindowByID(sourceSession, sourceActiveID) {
			sourceSession.Active = clampedWindowIndex(sourceSession, sourceSession.Active)
		}
		sourceSession.Activity = time.Now()
		targetSession.Activity = time.Now()
		return killed, nil
	}

	sourceActiveID := activeWindowID(sourceSession)
	targetActiveID := activeWindowID(targetSession)
	link := linkedWindowCopy(sourceSession.Windows[sourceIndex], targetWindowIndex)
	targetSession.Windows = append(targetSession.Windows, link)
	sort.Slice(targetSession.Windows, func(i, j int) bool {
		return targetSession.Windows[i].Index < targetSession.Windows[j].Index
	})
	if detached {
		if !setActiveWindowByID(targetSession, targetActiveID) {
			targetSession.Active = clampedWindowIndex(targetSession, targetSession.Active)
		}
	} else {
		setActiveWindowByWindowIndex(targetSession, targetWindowIndex)
	}
	if sourceSession != targetSession && !setActiveWindowByID(sourceSession, sourceActiveID) {
		sourceSession.Active = clampedWindowIndex(sourceSession, sourceSession.Active)
	}
	sourceSession.Activity = time.Now()
	targetSession.Activity = time.Now()
	return nil, nil
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
	if windowIndex < 0 || windowIndex >= len(session.Windows) {
		return
	}
	s.killWindowLinksLocked(session.Windows[windowIndex].ID)
}

func (s *Server) killWindowLinksLocked(windowID int) []int {
	var killed []int
	killedPanes := false
	for _, session := range s.Sessions {
		for index := 0; index < len(session.Windows); {
			window := session.Windows[index]
			if window.ID != windowID {
				index++
				continue
			}
			if !killedPanes {
				for _, pane := range window.Panes {
					killed = append(killed, pane.ID)
					killPane(pane)
				}
				killedPanes = true
			}
			session.Windows = append(session.Windows[:index], session.Windows[index+1:]...)
			if session.Active >= len(session.Windows) {
				session.Active = len(session.Windows) - 1
			}
			reindexWindows(session)
		}
		session.Activity = time.Now()
	}
	for name, session := range s.Sessions {
		if len(session.Windows) == 0 {
			delete(s.Sessions, name)
		}
	}
	return killed
}

func (s *Server) unlinkWindowAtLocked(session *Session, windowIndex int, deleteEmpty bool, reindex bool) []int {
	window := session.Windows[windowIndex]
	session.Windows = append(session.Windows[:windowIndex], session.Windows[windowIndex+1:]...)
	killed := make([]int, 0)
	if s.windowLinkCountLocked(window.ID) == 0 {
		for _, pane := range window.Panes {
			killed = append(killed, pane.ID)
			killPane(pane)
		}
	}
	if reindex {
		reindexWindows(session)
	}
	if session.Active >= len(session.Windows) {
		session.Active = len(session.Windows) - 1
	}
	if len(session.Windows) == 0 && deleteEmpty {
		delete(s.Sessions, session.Name)
	}
	return killed
}

func (s *Server) windowLinkCountLocked(windowID int) int {
	count := 0
	for _, session := range s.Sessions {
		for _, window := range session.Windows {
			if window.ID == windowID {
				count++
			}
		}
	}
	return count
}

func linkedWindowCopy(window *Window, index int) *Window {
	link := *window
	link.Index = index
	return &link
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

func setActiveWindowByWindowIndex(session *Session, windowIndex int) bool {
	for index, window := range session.Windows {
		if window.Index == windowIndex {
			selectWindowSliceIndex(session, index)
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
		ID:           s.NextWindowID,
		Index:        len(session.Windows),
		Name:         name,
		LastPaneID:   -1,
		ZoomedPaneID: -1,
		Width:        80,
		Height:       24,
		CreatedAt:    time.Now(),
		Activity:     time.Now(),
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

func reindexWindows(session *Session) {
	for i, window := range session.Windows {
		window.Index = i
	}
}
