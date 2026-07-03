package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const (
	TypeCommand = "command"
	TypeAttach  = "attach"
	TypeInput   = "input"
	TypeResize  = "resize"
	TypeOutput  = "output"
	TypeStatus  = "status"
	TypeResult  = "result"
	TypeDetach  = "detach"
	TypeError   = "error"
	TypeExit    = "exit"
)

type Message struct {
	Type         string     `json:"type"`
	ID           int64      `json:"id,omitempty"`
	Command      []string   `json:"command,omitempty"`
	Commands     [][]string `json:"commands,omitempty"`
	Session      string     `json:"session,omitempty"`
	Width        int        `json:"width,omitempty"`
	Height       int        `json:"height,omitempty"`
	Data         []byte     `json:"data,omitempty"`
	Text         string     `json:"text,omitempty"`
	StatusText   string     `json:"statusText,omitempty"`
	OK           bool       `json:"ok,omitempty"`
	Code         int        `json:"code,omitempty"`
	DetachOthers bool       `json:"detachOthers,omitempty"`
}

type Conn struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
}

func NewConn(rw io.ReadWriter) *Conn {
	return &Conn{r: bufio.NewReader(rw), w: rw}
}

func (c *Conn) Read() (Message, error) {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return Message{}, err
	}
	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return Message{}, fmt.Errorf("decode protocol message: %w", err)
	}
	return msg, nil
}

func (c *Conn) Write(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode protocol message: %w", err)
	}
	data = append(data, '\n')
	_, err = c.w.Write(data)
	return err
}
