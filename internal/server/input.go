package server

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
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
	rt.redrawClient(clientID)
	if result.Text != "" {
		rt.writeClientStatusMessage(clientID, result.Text)
	}
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

func (rt *Runtime) writeClientStatusMessage(clientID int64, text string) {
	rt.mu.RLock()
	client := rt.clients[clientID]
	rt.mu.RUnlock()
	if client == nil || text == "" {
		return
	}
	width, height := rt.state.ClientSize(clientID)
	_ = client.conn.Write(protocol.Message{
		Type: protocol.TypeStatus,
		Data: terminal.StatusLine(width, height, text),
	})
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
	if len(data) >= 3 && data[0] == '\x1b' {
		switch data[1] {
		case '[':
			if key, consumed, ok := csiInputKeyName(data); ok {
				return key, consumed
			}
		case 'O':
			if key, ok := ss3InputKeyName(data[2]); ok {
				return key, 3
			}
		}
	}
	if data[0] >= 1 && data[0] <= 26 {
		return fmt.Sprintf("C-%c", data[0]+'a'-1), 1
	}
	return string(data[0]), 1
}

func csiInputKeyName(data []byte) (string, int, bool) {
	for i := 2; i < len(data); i++ {
		final := data[i]
		if final < 0x40 || final > 0x7e {
			continue
		}
		raw := data[2:i]
		switch final {
		case 'A', 'B', 'C', 'D', 'F', 'H':
			base := map[byte]string{
				'A': "Up",
				'B': "Down",
				'C': "Right",
				'D': "Left",
				'F': "End",
				'H': "Home",
			}[final]
			return csiModifierPrefix(raw) + base, i + 1, true
		case '~':
			if key := tildeInputKeyName(csiBaseParam(raw)); key != "" {
				return csiModifierPrefix(raw) + key, i + 1, true
			}
		}
		return "", 0, false
	}
	return "", 0, false
}

func ss3InputKeyName(final byte) (string, bool) {
	switch final {
	case 'F':
		return "End", true
	case 'H':
		return "Home", true
	case 'P':
		return "F1", true
	case 'Q':
		return "F2", true
	case 'R':
		return "F3", true
	case 'S':
		return "F4", true
	default:
		return "", false
	}
}

func tildeInputKeyName(code string) string {
	switch code {
	case "1", "7":
		return "Home"
	case "2":
		return "Insert"
	case "3":
		return "Delete"
	case "4", "8":
		return "End"
	case "5":
		return "PageUp"
	case "6":
		return "PageDown"
	case "11":
		return "F1"
	case "12":
		return "F2"
	case "13":
		return "F3"
	case "14":
		return "F4"
	case "15":
		return "F5"
	case "17":
		return "F6"
	case "18":
		return "F7"
	case "19":
		return "F8"
	case "20":
		return "F9"
	case "21":
		return "F10"
	case "23":
		return "F11"
	case "24":
		return "F12"
	default:
		return ""
	}
}

func csiBaseParam(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if idx := bytes.IndexAny(raw, ";:"); idx >= 0 {
		raw = raw[:idx]
	}
	return string(raw)
}

func csiModifierPrefix(raw []byte) string {
	idx := bytes.LastIndexAny(raw, ";:")
	if idx < 0 || idx+1 >= len(raw) {
		return ""
	}
	modifier, err := strconv.Atoi(string(raw[idx+1:]))
	if err != nil {
		return ""
	}
	switch modifier {
	case 2:
		return "S-"
	case 3:
		return "M-"
	case 4:
		return "M-S-"
	case 5:
		return "C-"
	case 6:
		return "C-S-"
	case 7:
		return "C-M-"
	case 8:
		return "C-M-S-"
	default:
		return ""
	}
}
