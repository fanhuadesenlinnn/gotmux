package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

type Runtime struct {
	state *model.Server

	mu        sync.RWMutex
	clients   map[int64]*attachedClient
	screensMu sync.RWMutex
	screens   map[int]*terminal.Screen
	waitMu    sync.Mutex
	waitChans map[string]*waitChannel
	pipeMu    sync.Mutex
	pipes     map[int]*panePipe

	stopServer func()
}

type attachedClient struct {
	id   int64
	conn *protocol.Conn
	done chan struct{}
}

type panePipe struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func Run(ctx context.Context, socketPath string, configFiles []string) error {
	if err := os.MkdirAll(parentDir(socketPath), 0o700); err != nil {
		return err
	}
	runCtx, stopServer := context.WithCancel(ctx)
	defer stopServer()
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	_ = os.Chmod(socketPath, 0o600)

	rt := &Runtime{
		state:      model.NewServer(socketPath),
		clients:    make(map[int64]*attachedClient),
		screens:    make(map[int]*terminal.Screen),
		stopServer: stopServer,
	}
	for _, file := range configFiles {
		result := rt.cmdSourceFile([]string{file}, "", 80, 24)
		if !result.OK {
			return fmt.Errorf("%s: %s", file, result.Text)
		}
	}

	go func() {
		<-runCtx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-runCtx.Done():
				return nil
			default:
				return err
			}
		}
		go rt.handleConn(conn)
	}
}

func (rt *Runtime) handleConn(raw net.Conn) {
	defer raw.Close()
	conn := protocol.NewConn(raw)
	first, err := conn.Read()
	if err != nil {
		return
	}
	switch first.Type {
	case protocol.TypeAttach:
		rt.handleAttach(conn, first)
	case protocol.TypeCommand:
		rt.handleCommand(conn, first)
	default:
		_ = conn.Write(protocol.Message{Type: protocol.TypeError, Text: "expected command or attach"})
	}
}

func (rt *Runtime) handleAttach(conn *protocol.Conn, msg protocol.Message) {
	client, session, err := rt.state.AttachClient(msg.Session, msg.Width, msg.Height)
	if err != nil {
		_ = conn.Write(protocol.Message{Type: protocol.TypeError, Text: err.Error(), Code: 1})
		return
	}
	rt.state.SetActiveWindowSize(session.Name, msg.Width, max(1, msg.Height-1))
	ac := &attachedClient{id: client.ID, conn: conn, done: make(chan struct{})}
	rt.mu.Lock()
	rt.clients[client.ID] = ac
	rt.mu.Unlock()
	defer func() {
		rt.mu.Lock()
		delete(rt.clients, client.ID)
		rt.mu.Unlock()
		rt.state.DetachClient(client.ID)
		close(ac.done)
	}()

	_ = conn.Write(protocol.Message{Type: protocol.TypeResult, OK: true, ID: client.ID, Session: session.Name})
	rt.redrawClient(client.ID)
	rt.resizeActivePane(client.ID)

	for {
		next, err := conn.Read()
		if err != nil {
			return
		}
		switch next.Type {
		case protocol.TypeInput:
			rt.handleInput(client.ID, next.Data)
		case protocol.TypeResize:
			rt.state.SetClientSize(client.ID, next.Width, next.Height)
			sessionName := rt.state.ActiveSessionName(client.ID)
			rt.state.SetActiveWindowSize(sessionName, next.Width, max(1, next.Height-1))
			rt.resizeSessionPanes(sessionName)
			rt.redrawClient(client.ID)
		case protocol.TypeDetach:
			_ = conn.Write(protocol.Message{Type: protocol.TypeExit, Text: "detached"})
			return
		case protocol.TypeCommand:
			result := rt.executeMessageForClient(next, rt.state.ActiveSessionName(client.ID), client.ID)
			rt.writeCommandResult(client.ID, result)
			rt.afterCommandMessage(next)
			if result.Type == protocol.TypeExit {
				return
			}
			if !sessionExists(rt.state, rt.state.ActiveSessionName(client.ID)) {
				return
			}
		}
	}
}

func (rt *Runtime) handleCommand(conn *protocol.Conn, msg protocol.Message) {
	result := rt.executeMessage(msg, msg.Session)
	_ = conn.Write(result)
	rt.afterCommandMessage(msg)
}

func (rt *Runtime) startPane(pane *model.Pane, width, height int) error {
	if pane == nil || pane.PTY != nil {
		return nil
	}
	if pane.Exited {
		pane.Exited = false
		pane.ExitState = ""
	}
	if width <= 0 {
		width = 80
	}
	if height <= 1 {
		height = 24
	}
	paneWidth := pane.Width
	if paneWidth <= 0 {
		paneWidth = width
	}
	paneHeight := pane.Height
	if paneHeight <= 0 {
		paneHeight = height
	}
	args := model.NormalizeCommand(pane.Command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = pane.CWD
	env := pane.Env
	if len(env) == 0 {
		env = os.Environ()
	}
	cmd.Env = append(env,
		"TERM=screen-256color",
		"GOTMUX=1",
		fmt.Sprintf("TMUX=%s,%d,%d", rt.state.SocketPath, os.Getpid(), pane.ID),
	)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(max(1, paneHeight)),
		Cols: uint16(max(1, paneWidth)),
	})
	if err != nil {
		pane.Exited = true
		pane.ExitState = err.Error()
		return err
	}
	pane.PTY = file
	pane.Process = cmd
	pane.Width = paneWidth
	pane.Height = paneHeight
	pane.Generation++
	generation := pane.Generation
	rt.ensurePaneScreen(pane, paneWidth, paneHeight)

	go rt.readPane(pane, file, generation)
	go func() {
		err := cmd.Wait()
		if pane.Generation != generation {
			return
		}
		exitState := "exited"
		if err != nil {
			exitState = err.Error()
		}
		rt.closePaneAfterExit(pane.ID, exitState)
	}()
	return nil
}

func (rt *Runtime) closePaneAfterExit(paneID int, exitState string) {
	result := rt.state.ClosePaneOnExit(paneID, exitState)
	if !result.Removed {
		return
	}
	rt.screensMu.Lock()
	delete(rt.screens, paneID)
	rt.screensMu.Unlock()
	rt.closePanePipe(paneID)
	if result.SessionClosed {
		for _, clientID := range result.ClientIDs {
			rt.detachClient(clientID, "session closed")
		}
		rt.stopIfEmptySoon()
		return
	}
	rt.resizeSessionPanes(result.SessionName)
	for _, clientID := range result.ClientIDs {
		rt.redrawClient(clientID)
	}
	rt.broadcastStatus()
}

func (rt *Runtime) afterCommandMessage(msg protocol.Message) {
	rt.detachOrphanedClients("session closed")
	if commandMessageMayEmptyServer(msg) {
		rt.stopIfEmptySoon()
	}
}

func (rt *Runtime) detachOrphanedClients(text string) {
	for _, client := range rt.state.ListClients() {
		if !sessionExists(rt.state, client.SessionName) {
			rt.detachClient(client.ID, text)
		}
	}
}

func (rt *Runtime) stopIfEmptySoon() {
	if rt.stopServer == nil {
		return
	}
	time.AfterFunc(100*time.Millisecond, func() {
		if rt.hasSessions() {
			return
		}
		rt.stopServer()
	})
}

func (rt *Runtime) hasSessions() bool {
	sessions, _ := rt.state.Snapshot()
	return len(sessions) > 0
}

func commandMessageMayEmptyServer(msg protocol.Message) bool {
	commands := msg.Commands
	if len(commands) == 0 && len(msg.Command) > 0 {
		commands = [][]string{msg.Command}
	}
	for _, argv := range commands {
		if len(argv) == 0 {
			continue
		}
		switch normalizeCommandName(argv[0]) {
		case "kill-pane", "kill-session", "kill-window", "join-pane", "move-pane", "unlink-window":
			return true
		}
	}
	return false
}

func (rt *Runtime) readPane(pane *model.Pane, file *os.File, generation int) {
	buf := make([]byte, 32*1024)
	for {
		n, err := file.Read(buf)
		if pane.Generation != generation {
			return
		}
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			pane.History.Write(data)
			if screen := rt.ensurePaneScreen(pane, pane.Width, pane.Height); screen != nil {
				screen.Write(data)
			}
			rt.writePanePipe(pane.ID, data)
			pane.Activity = time.Now()
			rt.broadcastPaneOutput(pane, data)
		}
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "input/output error") {
				pane.ExitState = err.Error()
			}
			return
		}
	}
}

func (rt *Runtime) openPanePipe(pane *model.Pane, shellCommand string, in, out, toggle bool) error {
	rt.pipeMu.Lock()
	if rt.pipes == nil {
		rt.pipes = make(map[int]*panePipe)
	}
	if existing := rt.pipes[pane.ID]; existing != nil {
		rt.closePanePipeLocked(pane.ID, existing)
		if toggle {
			rt.pipeMu.Unlock()
			return nil
		}
	}
	rt.pipeMu.Unlock()

	cmd := exec.Command("/bin/sh", "-c", shellCommand)
	var stdin io.WriteCloser
	var stdout io.ReadCloser
	var err error
	if out {
		stdin, err = cmd.StdinPipe()
		if err != nil {
			return err
		}
	}
	if in {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			if stdin != nil {
				_ = stdin.Close()
			}
			return err
		}
	} else {
		cmd.Stdout = io.Discard
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		if stdin != nil {
			_ = stdin.Close()
		}
		if stdout != nil {
			_ = stdout.Close()
		}
		return err
	}
	pipe := &panePipe{cmd: cmd, stdin: stdin, stdout: stdout}

	rt.pipeMu.Lock()
	if rt.pipes == nil {
		rt.pipes = make(map[int]*panePipe)
	}
	rt.pipes[pane.ID] = pipe
	rt.pipeMu.Unlock()

	if stdout != nil {
		go rt.copyPipeToPane(pane, pipe)
	}
	go func() {
		_ = cmd.Wait()
		rt.pipeMu.Lock()
		if rt.pipes[pane.ID] == pipe {
			delete(rt.pipes, pane.ID)
		}
		rt.pipeMu.Unlock()
	}()
	return nil
}

func (rt *Runtime) closePanePipe(paneID int) {
	rt.pipeMu.Lock()
	if pipe := rt.pipes[paneID]; pipe != nil {
		rt.closePanePipeLocked(paneID, pipe)
	}
	rt.pipeMu.Unlock()
}

func (rt *Runtime) closePanePipeLocked(paneID int, pipe *panePipe) {
	delete(rt.pipes, paneID)
	if pipe.stdin != nil {
		_ = pipe.stdin.Close()
	}
	if pipe.stdout != nil {
		_ = pipe.stdout.Close()
	}
}

func (rt *Runtime) writePanePipe(paneID int, data []byte) {
	rt.pipeMu.Lock()
	pipe := rt.pipes[paneID]
	if pipe == nil || pipe.stdin == nil {
		rt.pipeMu.Unlock()
		return
	}
	if _, err := pipe.stdin.Write(data); err != nil {
		rt.closePanePipeLocked(paneID, pipe)
	}
	rt.pipeMu.Unlock()
}

func (rt *Runtime) copyPipeToPane(pane *model.Pane, pipe *panePipe) {
	buf := make([]byte, 32*1024)
	for {
		n, err := pipe.stdout.Read(buf)
		if n > 0 && pane.PTY != nil {
			_, _ = pane.PTY.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (rt *Runtime) broadcastPaneOutput(pane *model.Pane, data []byte) {
	rt.mu.RLock()
	clients := make(map[int64]*attachedClient, len(rt.clients))
	for id, client := range rt.clients {
		clients[id] = client
	}
	rt.mu.RUnlock()
	for id, client := range clients {
		sessionName := rt.state.ActiveSessionName(id)
		active := rt.state.ActivePane(sessionName)
		if active == nil {
			continue
		}
		panes := rt.state.ActiveWindowPanes(sessionName)
		if !containsRuntimePane(panes, pane.ID) {
			continue
		}
		if len(panes) <= 1 && active.ID == pane.ID {
			_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: data})
			continue
		}
		rt.renderClientContent(id, client, panes)
	}
}

func (rt *Runtime) redrawClient(clientID int64) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil {
		return
	}
	sessionName := rt.state.ActiveSessionName(clientID)
	panes := rt.state.ActiveWindowPanes(sessionName)
	if len(panes) == 0 {
		_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: terminal.ClearScreen()})
	} else if len(panes) == 1 {
		pane := panes[0]
		_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: terminal.ClearScreen()})
		if err := rt.startPane(pane, rt.clientWidth(clientID), rt.clientContentHeight(clientID)); err != nil {
			_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: []byte(err.Error() + "\r\n")})
		}
		_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: pane.History.Bytes()})
	} else {
		rt.renderClientContent(clientID, client, panes)
	}
	rt.redrawStatus(clientID)
}

func (rt *Runtime) renderClientContent(clientID int64, client *attachedClient, panes []*model.Pane) {
	width, height := rt.state.ClientSize(clientID)
	contentHeight := max(1, height-1)
	sessionName := rt.state.ActiveSessionName(clientID)
	rt.state.SetActiveWindowSize(sessionName, width, contentHeight)
	panes = rt.state.ActiveWindowPanes(sessionName)
	for _, pane := range panes {
		if err := rt.startPane(pane, width, contentHeight); err != nil {
			_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: []byte(err.Error() + "\r\n")})
		}
	}
	_ = client.conn.Write(protocol.Message{
		Type: protocol.TypeOutput,
		Data: rt.renderPanes(width, contentHeight, panes),
	})
}

func (rt *Runtime) redrawStatus(clientID int64) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil {
		return
	}
	width, height := rt.state.ClientSize(clientID)
	_ = client.conn.Write(protocol.Message{
		Type: protocol.TypeStatus,
		Data: terminal.StatusLine(width, height, rt.statusText(clientID)),
	})
}

func (rt *Runtime) broadcastStatus() {
	rt.mu.RLock()
	ids := make([]int64, 0, len(rt.clients))
	for id := range rt.clients {
		ids = append(ids, id)
	}
	rt.mu.RUnlock()
	for _, id := range ids {
		rt.redrawStatus(id)
	}
}

func (rt *Runtime) statusText(clientID int64) string {
	sessions, _ := rt.state.Snapshot()
	name := rt.state.ActiveSessionName(clientID)
	for _, session := range sessions {
		if session.Name != name {
			continue
		}
		window := session.ActiveWindow()
		if window == nil {
			return fmt.Sprintf("[%s]", session.Name)
		}
		pane := window.ActivePane()
		paneText := ""
		if pane != nil {
			paneText = fmt.Sprintf(" pane %d/%d", pane.Index, len(window.Panes))
			if pane.Exited {
				paneText += " exited"
			}
		}
		prefix := rt.state.GlobalOption("prefix")
		if prefix == "" {
			prefix = "C-b"
		}
		return fmt.Sprintf("[%s] %d:%s*%s | %d windows | prefix %s",
			session.Name, window.Index, window.Name, paneText, len(session.Windows), prefix)
	}
	return "[gotmux]"
}

func (rt *Runtime) resizeActivePane(clientID int64) {
	pane := rt.state.ActivePane(rt.state.ActiveSessionName(clientID))
	if pane == nil || pane.PTY == nil {
		return
	}
	width, height := rt.state.ClientSize(clientID)
	if pane.Width > 0 && pane.Height > 0 {
		rt.ensurePaneScreen(pane, pane.Width, pane.Height)
	}
	_ = pty.Setsize(pane.PTY, &pty.Winsize{
		Rows: uint16(max(1, height)),
		Cols: uint16(max(1, width)),
	})
}

func (rt *Runtime) resizeSessionPanes(sessionName string) {
	rt.resizePanes(rt.state.ActiveWindowPanes(sessionName))
}

func (rt *Runtime) resizePanes(panes []*model.Pane) {
	for _, pane := range panes {
		rt.ensurePaneScreen(pane, pane.Width, pane.Height)
		if pane == nil || pane.PTY == nil {
			continue
		}
		_ = pty.Setsize(pane.PTY, &pty.Winsize{
			Rows: uint16(max(1, pane.Height)),
			Cols: uint16(max(1, pane.Width)),
		})
	}
}

func (rt *Runtime) renderPanes(width, height int, panes []*model.Pane) []byte {
	return renderPanes(width, height, panes, rt.paneScreenLines(panes))
}

func (rt *Runtime) ensurePaneScreen(pane *model.Pane, width, height int) *terminal.Screen {
	if pane == nil {
		return nil
	}
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	rt.screensMu.Lock()
	defer rt.screensMu.Unlock()
	if rt.screens == nil {
		rt.screens = make(map[int]*terminal.Screen)
	}
	screen := rt.screens[pane.ID]
	if screen == nil {
		screen = terminal.NewScreen(width, height)
		rt.screens[pane.ID] = screen
		return screen
	}
	screen.Resize(width, height)
	return screen
}

func (rt *Runtime) paneScreenLines(panes []*model.Pane) map[int][]string {
	rt.screensMu.RLock()
	defer rt.screensMu.RUnlock()
	if len(rt.screens) == 0 {
		return nil
	}
	lines := make(map[int][]string, len(panes))
	for _, pane := range panes {
		if pane == nil {
			continue
		}
		screen := rt.screens[pane.ID]
		if screen == nil {
			continue
		}
		lines[pane.ID] = screen.Lines()
	}
	return lines
}

func (rt *Runtime) clientWidth(id int64) int {
	w, _ := rt.state.ClientSize(id)
	return w
}

func (rt *Runtime) clientHeight(id int64) int {
	_, h := rt.state.ClientSize(id)
	return h
}

func (rt *Runtime) clientContentHeight(id int64) int {
	return max(1, rt.clientHeight(id)-1)
}

func containsRuntimePane(panes []*model.Pane, paneID int) bool {
	for _, pane := range panes {
		if pane != nil && pane.ID == paneID {
			return true
		}
	}
	return false
}

func parentDir(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx <= 0 {
		return "."
	}
	return path[:idx]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
