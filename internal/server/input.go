package server

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

const prefixKey = byte(0x02)

func (rt *Runtime) handleInput(clientID int64, data []byte) {
	for len(data) > 0 {
		if rt.state.ClientPrefix(clientID) {
			consumed := rt.handlePrefixKey(clientID, data)
			rt.state.SetClientPrefix(clientID, false)
			if consumed <= 0 {
				consumed = 1
			}
			data = data[consumed:]
			continue
		}
		idx := bytes.IndexByte(data, prefixKey)
		if idx == -1 {
			rt.writeActivePane(clientID, data)
			return
		}
		if idx > 0 {
			rt.writeActivePane(clientID, data[:idx])
		}
		rt.state.SetClientPrefix(clientID, true)
		data = data[idx+1:]
	}
}

func (rt *Runtime) handlePrefixKey(clientID int64, data []byte) int {
	if len(data) == 0 {
		return 0
	}
	session := rt.state.ActiveSessionName(clientID)
	switch data[0] {
	case prefixKey:
		rt.writeActivePane(clientID, []byte{prefixKey})
	case 'c':
		result := rt.execute([]string{"new-window"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case '"':
		result := rt.execute([]string{"split-window", "-v"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case '%':
		result := rt.execute([]string{"split-window", "-h"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case 'n':
		result := rt.execute([]string{"next-window"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case 'p':
		result := rt.execute([]string{"previous-window"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case 'o', ';':
		result := rt.execute([]string{"select-pane", "-t", ":.+"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case 'x':
		result := rt.execute([]string{"kill-pane"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
		rt.writeCommandResult(clientID, result)
	case 'd':
		rt.detachClient(clientID, "detached")
	case '?':
		rt.showKeys(clientID)
	default:
		if data[0] >= '0' && data[0] <= '9' {
			result := rt.execute([]string{"select-window", "-t", ":" + string(data[0])}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
			rt.writeCommandResult(clientID, result)
			return 1
		}
		if len(data) >= 3 && data[0] == '\x1b' && data[1] == '[' {
			switch data[2] {
			case 'A', 'D':
				result := rt.execute([]string{"select-pane", "-t", ":.-"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
				rt.writeCommandResult(clientID, result)
			case 'B', 'C':
				result := rt.execute([]string{"select-pane", "-t", ":.+"}, session, rt.clientWidth(clientID), rt.clientHeight(clientID))
				rt.writeCommandResult(clientID, result)
			}
			return 3
		}
	}
	return 1
}

func (rt *Runtime) writeActivePane(clientID int64, data []byte) {
	pane := rt.state.ActivePane(rt.state.ActiveSessionName(clientID))
	if pane == nil || pane.PTY == nil || len(data) == 0 {
		return
	}
	_, _ = pane.PTY.Write(data)
}

func (rt *Runtime) writeCommandResult(clientID int64, result protocol.Message) {
	if !result.OK && result.Text != "" {
		rt.writeClientOutput(clientID, []byte(fmt.Sprintf("\r\n%s\r\n", result.Text)))
	}
	rt.redrawClient(clientID)
}

func (rt *Runtime) detachClient(clientID int64, text string) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil {
		return
	}
	_ = client.conn.Write(protocol.Message{Type: protocol.TypeResult, OK: true, Text: text})
	_ = client.conn.Write(protocol.Message{Type: protocol.TypeExit, Text: text})
}

func (rt *Runtime) showKeys(clientID int64) {
	keys := "\r\nC-b c new-window | C-b \" split-window | C-b % split-window -h | C-b n/p next/previous-window | C-b 0..9 select-window | C-b o select-pane | C-b x kill-pane | C-b d detach\r\n"
	rt.writeClientOutput(clientID, []byte(keys))
	rt.redrawStatus(clientID)
}

func (rt *Runtime) writeClientOutput(clientID int64, data []byte) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil {
		return
	}
	_ = client.conn.Write(protocol.Message{Type: protocol.TypeOutput, Data: data})
}

func parseWindowTarget(s string) (int, bool) {
	if len(s) > 0 && s[0] == ':' {
		s = s[1:]
	}
	if len(s) > 0 && s[0] == '=' {
		s = s[1:]
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
