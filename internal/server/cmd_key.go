package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdBindKey(args []string) protocol.Message {
	table := optionValue(args, "-T", "prefix")
	if hasAny(args, "-n") {
		table = "root"
	}
	note := optionValue(args, "-N", "")
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing key")
	}
	key := values[0]
	boundCommand := []string{"send-prefix"}
	if len(values) > 1 {
		boundCommand = values[1:]
	}
	rt.state.BindKey(table, key, boundCommand, note, hasAny(args, "-r"))
	return ok("")
}

func (rt *Runtime) cmdUnbindKey(args []string) protocol.Message {
	table := optionValue(args, "-T", "prefix")
	if hasAny(args, "-n") {
		table = "root"
	}
	if hasAny(args, "-a") {
		rt.state.UnbindKeyTable(table)
		return ok("")
	}
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing key")
	}
	rt.state.UnbindKey(table, values[0])
	return ok("")
}

func (rt *Runtime) cmdListKeys(args []string) protocol.Message {
	table := optionValue(args, "-T", "")
	format := optionValue(args, "-F", "")
	filterKeys := optionOperands(args)
	if table != "" && !rt.state.KeyTableExists(table) {
		return fail(fmt.Sprintf("table %s doesn't exist", table))
	}
	bindings := rt.state.ListKeyBindings(table)
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Table == bindings[j].Table {
			return bindings[i].Key < bindings[j].Key
		}
		return bindings[i].Table < bindings[j].Table
	})
	if hasAny(args, "-N") {
		return ok(rt.listKeyNotes(bindings))
	}
	var lines []string
	for _, binding := range bindings {
		if len(filterKeys) > 0 && binding.Key != filterKeys[0] {
			continue
		}
		if format != "" {
			lines = append(lines, formatKeyBinding(format, binding))
		} else {
			lines = append(lines, fmt.Sprintf("bind-key -T %s %s %s", binding.Table, binding.Key, strings.Join(binding.Command, " ")))
		}
		if hasAny(args, "-1") && len(lines) > 0 {
			break
		}
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) listKeyNotes(bindings []model.KeyBinding) string {
	prefix := "C-b"
	if options, err := rt.state.OptionsTarget("global", "", 0, false, true); err == nil {
		if value := options["prefix"]; value != "" {
			prefix = value
		}
	}
	lines := make([]string, 0)
	for _, binding := range bindings {
		if binding.Note == "" {
			continue
		}
		key := binding.Key
		if binding.Table == "prefix" {
			key = prefix + " " + key
		}
		lines = append(lines, fmt.Sprintf("%-12s %s", key, binding.Note))
	}
	return strings.Join(lines, "\n")
}

func (rt *Runtime) cmdSendPrefix(args []string, currentSession string) protocol.Message {
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil || pane.PTY == nil {
		return ok("")
	}
	prefix := rt.prefixByte()
	if hasAny(args, "-2") {
		var found bool
		prefix, found = rt.prefix2Byte()
		if !found {
			return ok("")
		}
	}
	_, _ = pane.PTY.Write([]byte{prefix})
	return ok("")
}

func (rt *Runtime) cmdSendKeys(args []string, currentSession string) protocol.Message {
	pane := rt.targetLivePane(optionValue(args, "-t", currentSession), currentSession)
	if pane == nil {
		return fail("can't find pane")
	}
	repeat := 1
	if value := optionValue(args, "-N", ""); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return fail("repeat count invalid")
		}
		repeat = parsed
	}
	keys := nonOptionArgs(args)
	for i := 0; i < repeat; i++ {
		sendKeysToPane(pane, keys, hasAny(args, "-l"))
	}
	return ok("")
}

func sendKeysToPane(pane *model.Pane, keys []string, literal bool) {
	if pane == nil || pane.PTY == nil {
		return
	}
	for _, key := range keys {
		if literal {
			_, _ = pane.PTY.Write([]byte(key))
			continue
		}
		switch key {
		case "":
			continue
		default:
			_, _ = pane.PTY.Write(sendKeyBytes(key))
		}
	}
}

func sendKeyBytes(key string) []byte {
	switch key {
	case "Enter", "C-m":
		return []byte{'\r'}
	case "Escape", "Esc", "C-[":
		return []byte{0x1b}
	case "Space":
		return []byte(" ")
	case "Tab", "C-i":
		return []byte{'\t'}
	case "BSpace", "Backspace", "C-?":
		return []byte{0x7f}
	case "Up":
		return []byte("\x1b[A")
	case "Down":
		return []byte("\x1b[B")
	case "Right":
		return []byte("\x1b[C")
	case "Left":
		return []byte("\x1b[D")
	case "Home":
		return []byte("\x1b[H")
	case "End":
		return []byte("\x1b[F")
	case "Insert", "IC":
		return []byte("\x1b[2~")
	case "Delete", "DC":
		return []byte("\x1b[3~")
	case "PageUp", "PPage":
		return []byte("\x1b[5~")
	case "PageDown", "NPage":
		return []byte("\x1b[6~")
	case "F1":
		return []byte("\x1bOP")
	case "F2":
		return []byte("\x1bOQ")
	case "F3":
		return []byte("\x1bOR")
	case "F4":
		return []byte("\x1bOS")
	case "F5":
		return []byte("\x1b[15~")
	case "F6":
		return []byte("\x1b[17~")
	case "F7":
		return []byte("\x1b[18~")
	case "F8":
		return []byte("\x1b[19~")
	case "F9":
		return []byte("\x1b[20~")
	case "F10":
		return []byte("\x1b[21~")
	case "F11":
		return []byte("\x1b[23~")
	case "F12":
		return []byte("\x1b[24~")
	}
	if strings.HasPrefix(key, "M-") && len(key) > 2 {
		return append([]byte{0x1b}, sendKeyBytes(key[2:])...)
	}
	if strings.HasPrefix(key, "C-") && len(key) == 3 {
		r := key[2]
		if r >= 'a' && r <= 'z' {
			return []byte{r - 'a' + 1}
		}
		if r >= 'A' && r <= 'Z' {
			return []byte{r - 'A' + 1}
		}
		switch r {
		case '@', ' ':
			return []byte{0}
		case '\\':
			return []byte{0x1c}
		case ']':
			return []byte{0x1d}
		case '^':
			return []byte{0x1e}
		case '_':
			return []byte{0x1f}
		}
	}
	return []byte(key)
}
