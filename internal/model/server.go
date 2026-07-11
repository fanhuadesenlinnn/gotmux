package model

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultStatusLeft  = "[#{session_name}] "
	DefaultStatusRight = "#{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}\"#{=21:pane_title}\" %H:%M %d-%b-%y"
)

type Server struct {
	mu                  sync.RWMutex
	Sessions            map[string]*Session
	Clients             map[int64]*Client
	Buffers             map[string]*Buffer
	Messages            []Message
	Access              map[string]ServerAccess
	ServerOptions       map[string]string
	GlobalOptions       map[string]string
	GlobalWindowOptions map[string]string
	GlobalEnvironment   map[string]string
	GlobalHiddenEnv     map[string]string
	GlobalRemovedEnv    map[string]bool
	GlobalHooks         map[string][]string
	KeyBindings         map[string]map[string]KeyBinding
	NextSessionID       int
	NextWindowID        int
	NextPaneID          int
	NextClientID        int64
	NextBufferID        int
	NextBufferOrder     int64
	NextMessageNumber   int64
	SocketPath          string
	StartedAt           time.Time
}

type Message struct {
	Number int64
	Time   time.Time
	Text   string
}

func NewServer(socketPath string) *Server {
	return &Server{
		Sessions:            make(map[string]*Session),
		Clients:             make(map[int64]*Client),
		Buffers:             make(map[string]*Buffer),
		Access:              make(map[string]ServerAccess),
		ServerOptions:       defaultServerOptions(),
		GlobalOptions:       defaultOptions(),
		GlobalWindowOptions: defaultWindowOptions(),
		GlobalEnvironment:   environmentMap(os.Environ()),
		GlobalHiddenEnv:     make(map[string]string),
		GlobalRemovedEnv:    make(map[string]bool),
		GlobalHooks:         defaultHooks(),
		KeyBindings:         defaultKeyBindings(),
		NextClientID:        1,
		SocketPath:          socketPath,
		StartedAt:           time.Now(),
	}
}

func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" && filepath.IsAbs(shell) {
		if st, err := os.Stat(shell); err == nil && !st.IsDir() {
			return shell
		}
	}
	return "/bin/sh"
}

func NormalizeCommand(args []string) []string {
	if len(args) == 0 {
		return []string{DefaultShell()}
	}
	return append([]string(nil), args...)
}

func CommandString(args []string) string {
	if len(args) == 0 {
		return DefaultShell()
	}
	return strings.Join(args, " ")
}

func (s *Server) AddMessage(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := 1000
	if value := s.ServerOptions["message-limit"]; value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			limit = parsed
		}
	}
	s.NextMessageNumber++
	if limit == 0 {
		return
	}
	s.Messages = append(s.Messages, Message{
		Number: s.NextMessageNumber,
		Time:   time.Now(),
		Text:   text,
	})
	if len(s.Messages) > limit {
		s.Messages = append([]Message(nil), s.Messages[len(s.Messages)-limit:]...)
	}
}

func (s *Server) MessageLog() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Message, len(s.Messages))
	copy(out, s.Messages)
	return out
}

func DefaultShellName() string {
	base := filepath.Base(DefaultShell())
	if base == "." || base == "/" || base == "" {
		return "shell"
	}
	return strings.TrimPrefix(base, "-")
}
