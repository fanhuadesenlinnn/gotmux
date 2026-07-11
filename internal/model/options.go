package model

import (
	"fmt"
	osuser "os/user"
	"sort"
	"strings"
)

type KeyBinding struct {
	Table   string
	Key     string
	Command []string
	Note    string
	Repeat  bool
}

type ServerAccess struct {
	Name  string
	Write bool
}

type Hook struct {
	Name     string
	Commands []string
}

var knownHookNames = []string{
	"after-bind-key",
	"after-capture-pane",
	"after-copy-mode",
	"after-display-message",
	"after-display-panes",
	"after-kill-pane",
	"after-list-buffers",
	"after-list-clients",
	"after-list-keys",
	"after-list-panes",
	"after-list-sessions",
	"after-list-windows",
	"after-load-buffer",
	"after-lock-server",
	"after-new-session",
	"after-new-window",
	"after-paste-buffer",
	"after-pipe-pane",
	"after-queue",
	"after-refresh-client",
	"after-rename-session",
	"after-rename-window",
	"after-resize-pane",
	"after-resize-window",
	"after-save-buffer",
	"after-select-layout",
	"after-select-pane",
	"after-select-window",
	"after-send-keys",
	"after-set-buffer",
	"after-set-environment",
	"after-set-hook",
	"after-set-option",
	"after-show-environment",
	"after-show-messages",
	"after-show-options",
	"after-split-window",
	"after-unbind-key",
	"alert-activity",
	"alert-bell",
	"alert-silence",
	"client-active",
	"client-attached",
	"client-detached",
	"client-focus-in",
	"client-focus-out",
	"client-resized",
	"client-session-changed",
	"client-light-theme",
	"client-dark-theme",
	"command-error",
	"pane-died",
	"pane-exited",
	"pane-focus-in",
	"pane-focus-out",
	"pane-mode-changed",
	"pane-set-clipboard",
	"pane-title-changed",
	"session-closed",
	"session-created",
	"session-renamed",
	"session-window-changed",
	"window-layout-changed",
	"window-linked",
	"window-pane-changed",
	"window-renamed",
	"window-resized",
	"window-unlinked",
}

func defaultOptions() map[string]string {
	return map[string]string{
		"base-index":      "0",
		"default-command": "",
		"default-shell":   DefaultShell(),
		"prefix":          "C-b",
		"prefix2":         "None",
		"status":          "on",
		"status-left":     DefaultStatusLeft,
		"status-right":    DefaultStatusRight,
	}
}

func defaultServerOptions() map[string]string {
	return map[string]string{
		"escape-time":   "10",
		"message-limit": "1000",
	}
}

func defaultWindowOptions() map[string]string {
	return map[string]string{
		"history-limit":            "2000",
		"main-pane-height":         "24",
		"main-pane-width":          "80",
		"mode-keys":                "emacs",
		"other-pane-height":        "0",
		"other-pane-width":         "0",
		"pane-base-index":          "0",
		"tiled-layout-max-columns": "0",
	}
}

func defaultHooks() map[string][]string {
	hooks := make(map[string][]string, len(knownHookNames))
	for _, name := range knownHookNames {
		hooks[name] = nil
	}
	return hooks
}

func defaultKeyBindings() map[string]map[string]KeyBinding {
	bindings := make(map[string]map[string]KeyBinding)
	bindings["root"] = make(map[string]KeyBinding)
	add := func(table, key string, command ...string) {
		if bindings[table] == nil {
			bindings[table] = make(map[string]KeyBinding)
		}
		bindings[table][key] = KeyBinding{Table: table, Key: key, Command: command}
	}
	add("prefix", "C-b", "send-prefix")
	add("prefix", `"`, "split-window")
	add("prefix", "%", "split-window", "-h")
	add("prefix", ";", "last-pane")
	add("prefix", "[", "copy-mode")
	add("prefix", "=", "choose-buffer")
	add("prefix", "c", "new-window")
	add("prefix", "d", "detach-client")
	add("prefix", "l", "last-window")
	add("prefix", "n", "next-window")
	add("prefix", "p", "previous-window")
	add("prefix", "q", "display-panes")
	add("prefix", "o", "select-pane", "-t", ":.+")
	add("prefix", "r", "refresh-client")
	add("prefix", "s", "choose-tree", "-s")
	add("prefix", "t", "clock-mode")
	add("prefix", "w", "choose-tree", "-Zw")
	add("prefix", "x", "kill-pane")
	add("prefix", "z", "resize-pane", "-Z")
	for i := 0; i <= 9; i++ {
		key := fmt.Sprintf("%d", i)
		add("prefix", key, "select-window", "-t", ":"+key)
	}
	return bindings
}

func (s *Server) SetOption(scope, sessionName, name, value string, appendValue, unset, setOnce bool) error {
	return s.SetOptionTarget(scope, sessionName, 0, false, name, value, appendValue, unset, setOnce)
}

func (s *Server) SetOptionTarget(scope, sessionName string, windowIndex int, hasWindow bool, name, value string, appendValue, unset, setOnce bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	storageScope := optionStorageScope(scope, name)
	options, err := s.optionsForScopeLocked(storageScope, sessionName, windowIndex, hasWindow)
	if err != nil {
		return err
	}
	if unset {
		switch storageScope {
		case "server":
			if value, ok := defaultServerOptions()[name]; ok {
				options[name] = value
			} else {
				delete(options, name)
			}
		case "global":
			if value, ok := defaultOptions()[name]; ok {
				options[name] = value
			} else {
				delete(options, name)
			}
		case "global-window":
			if value, ok := defaultWindowOptions()[name]; ok {
				options[name] = value
			} else {
				delete(options, name)
			}
		default:
			delete(options, name)
		}
		return nil
	}
	if setOnce {
		if _, exists := options[name]; exists {
			return fmt.Errorf("already set: %s", name)
		}
	}
	if appendValue {
		value = options[name] + value
	}
	options[name] = value
	return nil
}

func (s *Server) optionsForScopeLocked(scope, sessionName string, windowIndex int, hasWindow bool) (map[string]string, error) {
	switch scope {
	case "server":
		return s.ServerOptions, nil
	case "global":
		return s.GlobalOptions, nil
	case "global-window":
		return s.GlobalWindowOptions, nil
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		if session.Options == nil {
			session.Options = make(map[string]string)
		}
		return session.Options, nil
	case "window":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if hasWindow {
			window = nil
			for _, candidate := range session.Windows {
				if candidate.Index == windowIndex {
					window = candidate
					break
				}
			}
		}
		if window == nil {
			return nil, fmt.Errorf("session has no active window")
		}
		if window.Options == nil {
			window.Options = make(map[string]string)
		}
		return window.Options, nil
	default:
		return nil, fmt.Errorf("unknown option scope: %s", scope)
	}
}

func (s *Server) Options(scope, sessionName string, includeInherited bool) (map[string]string, error) {
	return s.OptionsTarget(scope, sessionName, 0, false, includeInherited)
}

func (s *Server) OptionsTarget(scope, sessionName string, windowIndex int, hasWindow bool, includeInherited bool) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string)
	copyOptions := func(src map[string]string) {
		for k, v := range src {
			out[k] = v
		}
	}
	switch scope {
	case "server":
		copyOptions(s.ServerOptions)
	case "global":
		copyOptions(s.ServerOptions)
		copyOptions(s.GlobalOptions)
	case "global-window":
		copyOptions(s.GlobalWindowOptions)
	case "session":
		if includeInherited {
			copyOptions(s.GlobalOptions)
		}
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		copyOptions(session.Options)
	case "window":
		if includeInherited {
			copyOptions(s.GlobalWindowOptions)
		}
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if hasWindow {
			window = nil
			for _, candidate := range session.Windows {
				if candidate.Index == windowIndex {
					window = candidate
					break
				}
			}
		}
		if window == nil {
			return nil, fmt.Errorf("session has no active window")
		}
		copyOptions(window.Options)
	default:
		return nil, fmt.Errorf("unknown option scope: %s", scope)
	}
	return out, nil
}

func optionStorageScope(scope, name string) string {
	if scope == "global" {
		if _, ok := defaultServerOptions()[name]; ok {
			return "server"
		}
		if _, ok := defaultWindowOptions()[name]; ok {
			return "global-window"
		}
	}
	return scope
}

func (s *Server) SetHook(scope, sessionName, name, command string, appendValue, unset bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isKnownHook(name) {
		return fmt.Errorf("invalid option: %s", name)
	}
	hooks, err := s.hooksForScopeLocked(scope, sessionName)
	if err != nil {
		return err
	}
	if *hooks == nil {
		if scope == "global" {
			*hooks = defaultHooks()
		} else {
			*hooks = make(map[string][]string)
		}
	}
	if unset {
		if scope == "global" {
			(*hooks)[name] = nil
		} else {
			delete(*hooks, name)
		}
		return nil
	}
	if command == "" {
		(*hooks)[name] = nil
		return nil
	}
	if appendValue {
		(*hooks)[name] = append(append([]string(nil), (*hooks)[name]...), command)
		return nil
	}
	(*hooks)[name] = []string{command}
	return nil
}

func (s *Server) Hooks(scope, sessionName, name string) ([]Hook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name != "" && !isKnownHook(name) {
		return nil, fmt.Errorf("invalid option: %s", name)
	}
	hooks, err := s.hooksForScopeLocked(scope, sessionName)
	if err != nil {
		return nil, err
	}
	out := make([]Hook, 0)
	addHook := func(hookName string, commands []string) {
		out = append(out, Hook{Name: hookName, Commands: append([]string(nil), commands...)})
	}
	if name != "" {
		if commands, ok := (*hooks)[name]; ok {
			addHook(name, commands)
		}
		return out, nil
	}
	if scope == "global" {
		for _, hookName := range knownHookNames {
			addHook(hookName, (*hooks)[hookName])
		}
		return out, nil
	}
	for _, hookName := range knownHookNames {
		if commands, ok := (*hooks)[hookName]; ok {
			addHook(hookName, commands)
		}
	}
	return out, nil
}

func (s *Server) hooksForScopeLocked(scope, sessionName string) (*map[string][]string, error) {
	switch scope {
	case "global":
		return &s.GlobalHooks, nil
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		return &session.Hooks, nil
	case "window":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if window == nil {
			return nil, fmt.Errorf("session has no active window")
		}
		return &window.Hooks, nil
	case "pane":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		window := session.ActiveWindow()
		if window == nil {
			return nil, fmt.Errorf("session has no active window")
		}
		pane := window.ActivePane()
		if pane == nil {
			return nil, fmt.Errorf("window has no active pane")
		}
		return &pane.Hooks, nil
	default:
		return nil, fmt.Errorf("unknown hook scope: %s", scope)
	}
}

func isKnownHook(name string) bool {
	for _, hookName := range knownHookNames {
		if hookName == name {
			return true
		}
	}
	return false
}

func (s *Server) ListServerAccess() []ServerAccess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ownerName, _ := currentServerUser()
	out := []ServerAccess{{Name: ownerName, Write: true}}
	names := make([]string, 0, len(s.Access))
	for name := range s.Access {
		if name != ownerName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, s.Access[name])
	}
	return out
}

func (s *Server) ChangeServerAccess(name string, add, remove, readOnly, write bool) error {
	target, err := osuser.Lookup(name)
	if err != nil {
		return fmt.Errorf("unknown user: %s", name)
	}
	_, ownerUID := currentServerUser()
	if target.Uid == "0" || target.Uid == ownerUID {
		return fmt.Errorf("%s owns the server, can't change access", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.Access[name]
	if remove {
		if !exists {
			return fmt.Errorf("user %s not found", name)
		}
		delete(s.Access, name)
		return nil
	}
	if add {
		if exists {
			return fmt.Errorf("user %s is already added", name)
		}
		s.Access[name] = ServerAccess{Name: name, Write: true}
		exists = true
	}
	if readOnly || write {
		entry, ok := s.Access[name]
		if !ok {
			entry = ServerAccess{Name: name, Write: true}
		}
		if readOnly {
			entry.Write = false
		}
		if write {
			entry.Write = true
		}
		s.Access[name] = entry
	}
	return nil
}

func currentServerUser() (string, string) {
	current, err := osuser.Current()
	if err != nil || current == nil {
		return "unknown", ""
	}
	name := current.Username
	if idx := strings.LastIndexAny(name, `\`); idx >= 0 {
		name = name[idx+1:]
	}
	return name, current.Uid
}

func (s *Server) GlobalOption(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.GlobalOptions[name]
}

func (s *Server) BindKey(table, key string, command []string, note string, repeat bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if table == "" {
		table = "prefix"
	}
	if s.KeyBindings[table] == nil {
		s.KeyBindings[table] = make(map[string]KeyBinding)
	}
	s.KeyBindings[table][key] = KeyBinding{
		Table:   table,
		Key:     key,
		Command: append([]string(nil), command...),
		Note:    note,
		Repeat:  repeat,
	}
}

func (s *Server) UnbindKey(table, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if table == "" {
		table = "prefix"
	}
	if s.KeyBindings[table] != nil {
		delete(s.KeyBindings[table], key)
	}
}

func (s *Server) UnbindKeyTable(table string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if table == "" {
		table = "prefix"
	}
	if table == "root" {
		s.KeyBindings[table] = make(map[string]KeyBinding)
		return
	}
	delete(s.KeyBindings, table)
}

func (s *Server) KeyBinding(table, key string) (KeyBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if table == "" {
		table = "prefix"
	}
	binding, ok := s.KeyBindings[table][key]
	return binding, ok
}

func (s *Server) KeyTableExists(table string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if table == "" {
		table = "prefix"
	}
	_, ok := s.KeyBindings[table]
	return ok
}

func (s *Server) ListKeyBindings(table string) []KeyBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []KeyBinding
	appendTable := func(tableName string, tableBindings map[string]KeyBinding) {
		for _, binding := range tableBindings {
			binding.Table = tableName
			out = append(out, binding)
		}
	}
	if table != "" {
		appendTable(table, s.KeyBindings[table])
	} else {
		for tableName, tableBindings := range s.KeyBindings {
			appendTable(tableName, tableBindings)
		}
	}
	return out
}

func (s *Server) SetEnvironment(scope, sessionName, name, value string, hidden, remove bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case "global":
		if remove {
			delete(s.GlobalEnvironment, name)
			delete(s.GlobalHiddenEnv, name)
			s.GlobalRemovedEnv[name] = true
		} else if hidden {
			delete(s.GlobalRemovedEnv, name)
			delete(s.GlobalEnvironment, name)
			s.GlobalHiddenEnv[name] = value
		} else {
			delete(s.GlobalRemovedEnv, name)
			delete(s.GlobalHiddenEnv, name)
			s.GlobalEnvironment[name] = value
		}
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		if remove {
			delete(session.Environment, name)
			delete(session.HiddenEnv, name)
			if session.RemovedEnv == nil {
				session.RemovedEnv = make(map[string]bool)
			}
			session.RemovedEnv[name] = true
		} else if hidden {
			delete(session.RemovedEnv, name)
			delete(session.Environment, name)
			if session.HiddenEnv == nil {
				session.HiddenEnv = make(map[string]string)
			}
			session.HiddenEnv[name] = value
		} else {
			delete(session.RemovedEnv, name)
			delete(session.HiddenEnv, name)
			if session.Environment == nil {
				session.Environment = make(map[string]string)
			}
			session.Environment[name] = value
		}
	default:
		return fmt.Errorf("unknown environment scope: %s", scope)
	}
	return nil
}

func (s *Server) UnsetEnvironment(scope, sessionName, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case "global":
		delete(s.GlobalEnvironment, name)
		delete(s.GlobalHiddenEnv, name)
		delete(s.GlobalRemovedEnv, name)
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return fmt.Errorf("can't find session: %s", sessionName)
		}
		delete(session.Environment, name)
		delete(session.HiddenEnv, name)
		delete(session.RemovedEnv, name)
	default:
		return fmt.Errorf("unknown environment scope: %s", scope)
	}
	return nil
}

func (s *Server) Environment(scope, sessionName string, hidden bool) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string)
	globalEnv := s.GlobalEnvironment
	if hidden {
		globalEnv = s.GlobalHiddenEnv
	}
	if scope == "global" {
		for key, value := range globalEnv {
			out[key] = value
		}
		return out, nil
	}
	session := s.Sessions[sessionName]
	if session == nil {
		return nil, fmt.Errorf("can't find session: %s", sessionName)
	}
	sessionEnv := session.Environment
	if hidden {
		sessionEnv = session.HiddenEnv
	}
	for key, value := range sessionEnv {
		out[key] = value
	}
	return out, nil
}

func (s *Server) EnvironmentRemovals(scope, sessionName string) (map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]bool)
	switch scope {
	case "global":
		for key, value := range s.GlobalRemovedEnv {
			out[key] = value
		}
	case "session":
		session := s.Sessions[sessionName]
		if session == nil {
			return nil, fmt.Errorf("can't find session: %s", sessionName)
		}
		for key, value := range session.RemovedEnv {
			out[key] = value
		}
	default:
		return nil, fmt.Errorf("unknown environment scope: %s", scope)
	}
	return out, nil
}

func environmentMap(values []string) map[string]string {
	env := make(map[string]string, len(values))
	for _, item := range values {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			env[name] = value
		}
	}
	return env
}

func environmentList(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}
