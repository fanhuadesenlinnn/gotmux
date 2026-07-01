package server

import (
	"bytes"
	"fmt"

	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

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
		key, consumed := inputKeyName(data)
		if binding, ok := rt.state.KeyBinding("root", key); ok {
			rt.executeBinding(clientID, binding.Command)
			if consumed <= 0 {
				consumed = 1
			}
			data = data[consumed:]
			continue
		}
		idx := bytes.IndexByte(data, rt.prefixByte())
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
	key, consumed := inputKeyName(data)
	if key == "?" {
		rt.showKeys(clientID)
		return consumed
	}
	if key == rt.state.GlobalOption("prefix") {
		rt.writeActivePane(clientID, []byte{rt.prefixByte()})
		return consumed
	}
	if binding, ok := rt.state.KeyBinding("prefix", key); ok {
		rt.executeBinding(clientID, binding.Command)
	}
	return consumed
}

func (rt *Runtime) executeBinding(clientID int64, command []string) {
	session := rt.state.ActiveSessionName(clientID)
	result := rt.executeWithClient(command, session, rt.clientWidth(clientID), rt.clientContentHeight(clientID), clientID)
	rt.writeCommandResult(clientID, result)
}

func (rt *Runtime) writeActivePane(clientID int64, data []byte) {
	pane := rt.state.ActivePane(rt.state.ActiveSessionName(clientID))
	if pane == nil || pane.PTY == nil || len(data) == 0 {
		return
	}
	_, _ = pane.PTY.Write(data)
}

func (rt *Runtime) writeCommandResult(clientID int64, result protocol.Message) {
	if result.Type == protocol.TypeExit {
		rt.detachClient(clientID, result.Text)
		return
	}
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
	keys := "\r\n" + rt.cmdListKeys([]string{"-T", "prefix"}).Text + "\r\n"
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

func (rt *Runtime) prefixByte() byte {
	prefix := rt.state.GlobalOption("prefix")
	if len(prefix) == 3 && prefix[0] == 'C' && prefix[1] == '-' {
		return prefix[2] & 0x1f
	}
	if len(prefix) == 1 {
		return prefix[0]
	}
	return 0x02
}

func inputKeyName(data []byte) (string, int) {
	if len(data) >= 3 && data[0] == '\x1b' && data[1] == '[' {
		switch data[2] {
		case 'A':
			return "Up", 3
		case 'B':
			return "Down", 3
		case 'C':
			return "Right", 3
		case 'D':
			return "Left", 3
		}
	}
	if data[0] >= 1 && data[0] <= 26 {
		return fmt.Sprintf("C-%c", data[0]+'a'-1), 1
	}
	return string(data[0]), 1
}
