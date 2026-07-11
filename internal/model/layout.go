package model

import (
	"fmt"
	"strconv"
	"strings"
)

type LayoutNode struct {
	Orientation string
	PaneID      int
	Children    []*LayoutNode
	Left        int
	Top         int
	Width       int
	Height      int
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
	if w.Zoomed && w.ZoomedPaneID == paneID {
		w.Zoomed = false
		w.ZoomedPaneID = -1
	}
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
	w.applyZoomedGeometry()
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

func (w *Window) applyZoomedGeometry() {
	if !w.Zoomed {
		return
	}
	pane := w.paneByID(w.ZoomedPaneID)
	if pane == nil {
		w.Zoomed = false
		w.ZoomedPaneID = -1
		return
	}
	pane.Left = 0
	pane.Top = 0
	pane.Width = w.Width
	pane.Height = w.Height
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
