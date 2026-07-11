package model

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Pane struct {
	ID         int
	Index      int
	Command    []string
	Env        []string
	CWD        string
	Left       int
	Top        int
	Width      int
	Height     int
	Floating   bool
	CreatedAt  time.Time
	Activity   time.Time
	PTY        *os.File
	Process    *exec.Cmd
	History    *Ring
	Hooks      map[string][]string
	Exited     bool
	ExitState  string
	Generation int
}

type PaneExitResult struct {
	Removed       bool
	SessionName   string
	SessionClosed bool
	ClientIDs     []int64
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

func (s *Server) NewFloatingPaneDetached(sessionName string, windowIndex int, hasWindow bool, cwd string, command []string, detached bool, left, top, width, height int) (*Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
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
		return nil, fmt.Errorf("can't find window: %d", windowIndex)
	}
	if cwd == "" {
		cwd = session.CWD
	}
	activeID := -1
	if active := window.ActivePane(); active != nil {
		activeID = active.ID
	}
	pane := s.newPaneLocked(session, window, cwd, command)
	pane.Floating = true
	pane.Width = clampSize(width, window.Width, maxInt(1, window.Width/2))
	pane.Height = clampSize(height, window.Height, maxInt(1, window.Height/4))
	pane.Left = clampPosition(left, window.Width-pane.Width, 0)
	pane.Top = clampPosition(top, window.Height-pane.Height, 0)
	if !detached {
		selectPaneByID(window, pane.ID)
	} else if activeID != -1 {
		setActivePaneByID(window, activeID)
	}
	window.Activity = time.Now()
	session.Activity = time.Now()
	return pane, nil
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

func (s *Server) ClosePaneOnExit(paneID int, exitState string) PaneExitResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return PaneExitResult{}
	}
	result := PaneExitResult{
		Removed:       true,
		SessionName:   location.session.Name,
		SessionClosed: len(location.session.Windows) == 1 && len(location.window.Panes) == 1,
	}
	for _, client := range s.Clients {
		if client.SessionName == location.session.Name {
			result.ClientIDs = append(result.ClientIDs, client.ID)
		}
	}
	location.pane.Exited = true
	location.pane.ExitState = exitState
	s.killPaneAtLocked(location.session, location.window, location.paneIndex)
	return result
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

func (s *Server) RespawnPaneByID(paneID int, cwd string, command []string, killActive bool) (*Pane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return nil, fmt.Errorf("can't find pane: %d", paneID)
	}
	if location.pane.PTY != nil && !location.pane.Exited && !killActive {
		return nil, fmt.Errorf("pane still active")
	}
	respawnPaneLocked(location.session, location.window, location.pane, cwd, command, killActive)
	return location.pane, nil
}

func (s *Server) RespawnWindowByIndex(sessionName string, windowIndex int, cwd string, command []string, killActive bool) (*Pane, []int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.Sessions[sessionName]
	if session == nil {
		return nil, nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	var window *Window
	for _, candidate := range session.Windows {
		if candidate.Index == windowIndex {
			window = candidate
			break
		}
	}
	if window == nil {
		return nil, nil, fmt.Errorf("can't find window: %d", windowIndex)
	}
	pane := window.ActivePane()
	if pane == nil {
		return nil, nil, fmt.Errorf("window has no active pane")
	}
	if pane.PTY != nil && !pane.Exited && !killActive {
		return nil, nil, fmt.Errorf("window still active")
	}
	killed := make([]int, 0, len(window.Panes)-1)
	for _, other := range window.Panes {
		if other.ID == pane.ID {
			continue
		}
		killed = append(killed, other.ID)
		killPane(other)
	}
	window.Panes = []*Pane{pane}
	window.Active = 0
	window.LastPaneID = 0
	pane.Index = 0
	window.Layout = &LayoutNode{PaneID: pane.ID}
	respawnPaneLocked(session, window, pane, cwd, command, killActive)
	window.recalculateLayout()
	return pane, killed, nil
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
	if window.Zoomed {
		if pane := window.paneByID(window.ZoomedPaneID); pane != nil {
			return []*Pane{pane}
		}
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

func (s *Server) TogglePaneZoom(paneID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	location, ok := s.paneLocationLocked(paneID)
	if !ok {
		return fmt.Errorf("can't find pane: %d", paneID)
	}
	if location.window.Zoomed {
		location.window.Zoomed = false
		location.window.ZoomedPaneID = -1
		location.window.recalculateLayout()
	} else {
		selectPaneIndex(location.window, location.paneIndex)
		location.window.Zoomed = true
		location.window.ZoomedPaneID = paneID
		location.window.recalculateLayout()
	}
	location.window.Activity = time.Now()
	location.session.Activity = time.Now()
	return nil
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
					if window.Zoomed {
						if zoomed := window.paneByID(window.ZoomedPaneID); zoomed != nil {
							return []*Pane{zoomed}
						}
					}
					out := make([]*Pane, len(window.Panes))
					copy(out, window.Panes)
					return out
				}
			}
		}
	}
	return nil
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

func clampSize(value, total, fallback int) int {
	if total <= 0 {
		total = fallback
	}
	if value <= 0 {
		value = fallback
	}
	if value < 1 {
		return 1
	}
	if total > 0 && value > total {
		return total
	}
	return value
}

func clampPosition(value, maxPosition, fallback int) int {
	if value < 0 {
		value = fallback
	}
	if value < 0 {
		return 0
	}
	if maxPosition < 0 {
		return 0
	}
	if value > maxPosition {
		return maxPosition
	}
	return value
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
