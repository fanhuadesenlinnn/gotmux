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

	mu      sync.RWMutex
	clients map[int64]*attachedClient
}

type attachedClient struct {
	id   int64
	conn *protocol.Conn
	done chan struct{}
}

func Run(ctx context.Context, socketPath string) error {
	if err := os.MkdirAll(parentDir(socketPath), 0o700); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	_ = os.Chmod(socketPath, 0o600)

	rt := &Runtime{
		state:   model.NewServer(socketPath),
		clients: make(map[int64]*attachedClient),
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
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
			rt.resizeActivePane(client.ID)
			rt.redrawStatus(client.ID)
		case protocol.TypeDetach:
			_ = conn.Write(protocol.Message{Type: protocol.TypeExit, Text: "detached"})
			return
		case protocol.TypeCommand:
			result := rt.execute(next.Command, rt.state.ActiveSessionName(client.ID), next.Width, next.Height)
			_ = conn.Write(result)
			rt.redrawClient(client.ID)
		}
	}
}

func (rt *Runtime) handleCommand(conn *protocol.Conn, msg protocol.Message) {
	result := rt.execute(msg.Command, msg.Session, msg.Width, msg.Height)
	_ = conn.Write(result)
}

func (rt *Runtime) startPane(pane *model.Pane, width, height int) error {
	if pane == nil || pane.PTY != nil || pane.Exited {
		return nil
	}
	if width <= 0 {
		width = 80
	}
	if height <= 1 {
		height = 24
	}
	args := model.NormalizeCommand(pane.Command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = pane.CWD
	cmd.Env = append(os.Environ(),
		"TERM=screen-256color",
		"GOTMUX=1",
		fmt.Sprintf("TMUX=%s,%d,%d", rt.state.SocketPath, os.Getpid(), pane.ID),
	)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(max(1, height-1)),
		Cols: uint16(max(1, width)),
	})
	if err != nil {
		pane.Exited = true
		pane.ExitState = err.Error()
		return err
	}
	pane.PTY = file
	pane.Process = cmd
	pane.Width = width
	pane.Height = height - 1

	go rt.readPane(pane)
	go func() {
		err := cmd.Wait()
		if err != nil {
			pane.ExitState = err.Error()
		} else {
			pane.ExitState = "exited"
		}
		pane.Exited = true
		rt.broadcastStatus()
	}()
	return nil
}

func (rt *Runtime) readPane(pane *model.Pane) {
	buf := make([]byte, 32*1024)
	for {
		n, err := pane.PTY.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			pane.History.Write(data)
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

func (rt *Runtime) broadcastPaneOutput(pane *model.Pane, data []byte) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	for id, client := range rt.clients {
		active := rt.state.ActivePane(rt.state.ActiveSessionName(id))
		if active == nil || active.ID != pane.ID {
			continue
		}
		_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: data})
	}
}

func (rt *Runtime) redrawClient(clientID int64) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil {
		return
	}
	pane := rt.state.ActivePane(rt.state.ActiveSessionName(clientID))
	_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: terminal.ClearScreen()})
	if pane != nil {
		if err := rt.startPane(pane, rt.clientWidth(clientID), rt.clientHeight(clientID)); err != nil {
			_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: []byte(err.Error() + "\r\n")})
		}
		_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: pane.History.Bytes()})
	}
	rt.redrawStatus(clientID)
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
		return fmt.Sprintf("[%s] %d:%s*%s | %d windows | prefix C-b",
			session.Name, window.Index, window.Name, paneText, len(session.Windows))
	}
	return "[gotmux]"
}

func (rt *Runtime) resizeActivePane(clientID int64) {
	pane := rt.state.ActivePane(rt.state.ActiveSessionName(clientID))
	if pane == nil || pane.PTY == nil {
		return
	}
	width, height := rt.state.ClientSize(clientID)
	_ = pty.Setsize(pane.PTY, &pty.Winsize{
		Rows: uint16(max(1, height-1)),
		Cols: uint16(max(1, width)),
	})
}

func (rt *Runtime) clientWidth(id int64) int {
	w, _ := rt.state.ClientSize(id)
	return w
}

func (rt *Runtime) clientHeight(id int64) int {
	_, h := rt.state.ClientSize(id)
	return h
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
