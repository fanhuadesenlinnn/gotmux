package server

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdChooseBuffer() protocol.Message {
	buffers := rt.state.ListBuffers()
	if len(buffers) == 0 {
		return status("choose-buffer: empty")
	}
	parts := make([]string, 0, len(buffers))
	for _, buffer := range buffers {
		parts = append(parts, fmt.Sprintf("%s:%d:%s", buffer.Name, len(buffer.Data), quoteBufferSample(buffer.Data)))
	}
	return status("choose-buffer: " + strings.Join(parts, " "))
}

func (rt *Runtime) cmdSetBuffer(args []string) protocol.Message {
	if newName := optionValue(args, "-n", ""); newName != "" {
		if err := rt.state.RenameBuffer(optionValue(args, "-b", ""), newName); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing buffer data")
	}
	rt.state.SetBuffer(optionValue(args, "-b", ""), strings.Join(values, " "), hasAny(args, "-a"))
	return ok("")
}

func (rt *Runtime) cmdShowBuffer(args []string) protocol.Message {
	data, err := rt.state.ShowBuffer(optionValue(args, "-b", ""))
	if err != nil {
		return fail(err.Error())
	}
	return ok(data)
}

func (rt *Runtime) cmdListBuffers(args []string) protocol.Message {
	format := optionValue(args, "-F", "")
	filter := optionValue(args, "-f", "")
	buffers := rt.state.ListBuffers()
	switch order := optionValue(args, "-O", ""); order {
	case "", "time":
	case "name":
		sort.Slice(buffers, func(i, j int) bool {
			return buffers[i].Name < buffers[j].Name
		})
	case "size":
		sort.Slice(buffers, func(i, j int) bool {
			if len(buffers[i].Data) == len(buffers[j].Data) {
				return buffers[i].Name < buffers[j].Name
			}
			return len(buffers[i].Data) < len(buffers[j].Data)
		})
	default:
		return fail("invalid sort order")
	}
	lines := make([]string, 0, len(buffers))
	for _, buffer := range buffers {
		if filter != "" && !formatBufferTruthy(filter, buffer) {
			continue
		}
		if format != "" {
			lines = append(lines, formatBuffer(format, buffer))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %d bytes: %s", buffer.Name, len(buffer.Data), quoteBufferSample(buffer.Data)))
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) cmdDeleteBuffer(args []string) protocol.Message {
	if err := rt.state.DeleteBuffer(optionValue(args, "-b", "")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdLoadBuffer(args []string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing path")
	}
	path := expandPath(values[len(values)-1])
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(fmt.Sprintf("No such file or directory: %s", path))
		}
		return fail(err.Error())
	}
	rt.state.SetBuffer(optionValue(args, "-b", ""), string(data), false)
	return ok("")
}

func (rt *Runtime) cmdSaveBuffer(args []string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing path")
	}
	data, err := rt.state.ShowBuffer(optionValue(args, "-b", ""))
	if err != nil {
		return fail(err.Error())
	}
	path := expandPath(values[len(values)-1])
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if hasAny(args, "-a") {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	file, err := os.OpenFile(path, flag, 0o666)
	if err != nil {
		return fail(err.Error())
	}
	defer file.Close()
	if _, err := file.WriteString(data); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func formatBuffer(template string, buffer model.Buffer) string {
	out := template
	replacements := map[string]string{
		"#{buffer_name}":   buffer.Name,
		"#{buffer_size}":   strconv.Itoa(len(buffer.Data)),
		"#{buffer_sample}": bufferSample(buffer.Data),
	}
	for old, newValue := range replacements {
		out = strings.ReplaceAll(out, old, newValue)
	}
	return out
}

func formatBufferTruthy(template string, buffer model.Buffer) bool {
	value := strings.TrimSpace(formatBuffer(template, buffer))
	return value != "" && value != "0"
}

func bufferSample(data string) string {
	data = strings.ReplaceAll(data, "\\", "\\\\")
	data = strings.ReplaceAll(data, "\r", "\\r")
	data = strings.ReplaceAll(data, "\n", "\\n")
	if len(data) > 50 {
		data = data[:50]
	}
	return data
}

func quoteBufferSample(data string) string {
	return `"` + strings.ReplaceAll(bufferSample(data), `"`, `\"`) + `"`
}
