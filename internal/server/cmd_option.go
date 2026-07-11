package server

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/model"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func (rt *Runtime) cmdSetOption(args []string, currentSession string, defaultScope string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing option")
	}
	name := values[0]
	value := ""
	if len(values) > 1 {
		value = strings.Join(values[1:], " ")
	}
	scope := defaultScope
	if hasAny(args, "-g") {
		if defaultScope == "window" {
			scope = "global-window"
		} else {
			scope = "global"
		}
	}
	if hasAny(args, "-w") {
		scope = "window"
	}
	if hasAny(args, "-s") {
		scope = "server"
	}
	targetSession, targetWindow, hasWindow, targetErr := rt.optionCommandTarget(args, currentSession, scope)
	if targetErr != "" {
		return fail(targetErr)
	}
	if err := rt.state.SetOptionTarget(scope, targetSession, targetWindow, hasWindow, name, value, hasAny(args, "-a"), hasAny(args, "-u"), hasAny(args, "-o")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdShowOptions(args []string, currentSession string) protocol.Message {
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	if hasAny(args, "-w") {
		if hasAny(args, "-g") {
			scope = "global-window"
		} else {
			scope = "window"
		}
	}
	if hasAny(args, "-s") {
		scope = "server"
	}
	includeInherited := hasAny(args, "-A")
	targetSession, targetWindow, hasWindow, targetErr := rt.optionCommandTarget(args, currentSession, scope)
	if targetErr != "" {
		return fail(targetErr)
	}
	options, err := rt.state.OptionsTarget(scope, targetSession, targetWindow, hasWindow, includeInherited)
	if err != nil {
		return fail(err.Error())
	}
	names := optionOperands(args)
	valueOnly := hasAny(args, "-v")
	if len(names) > 0 {
		value, exists := options[names[0]]
		if !exists {
			if hasAny(args, "-H") {
				hooks, err := rt.state.Hooks(optionHookScope(scope), targetSession, names[0])
				if err == nil {
					return ok(strings.Join(formatHooks(hooks, valueOnly), "\n"))
				}
			}
			if scope == "server" && rt.optionKnownOutsideServer(names[0], targetSession) {
				return ok("")
			}
			if !includeInherited && (scope == "session" || scope == "window") {
				inherited, err := rt.state.OptionsTarget(scope, targetSession, targetWindow, hasWindow, true)
				if err == nil {
					if _, known := inherited[names[0]]; known {
						return ok("")
					}
				}
			}
			if hasAny(args, "-q") {
				return ok("")
			}
			return fail(fmt.Sprintf("invalid option: %s", names[0]))
		}
		if valueOnly {
			return ok(value)
		}
		return ok(fmt.Sprintf("%s %s", names[0], value))
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		if valueOnly {
			lines = append(lines, options[key])
		} else {
			lines = append(lines, fmt.Sprintf("%s %s", key, options[key]))
		}
	}
	if hasAny(args, "-H") {
		hooks, err := rt.state.Hooks(optionHookScope(scope), targetSession, "")
		if err != nil {
			return fail(err.Error())
		}
		lines = append(lines, formatHooks(hooks, valueOnly)...)
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) optionCommandTarget(args []string, currentSession string, scope string) (string, int, bool, string) {
	if scope == "server" || scope == "global" || scope == "global-window" {
		return "", 0, false, ""
	}
	target := optionValue(args, "-t", currentSession)
	if target == "" {
		target = firstSessionName(rt.state)
	}
	if scope == "window" {
		sessionName, windowIndex, _, _, found := rt.targetWindowInfo(target, currentSession)
		if found {
			return sessionName, windowIndex, true, ""
		}
		if sessionName == "" || !sessionExists(rt.state, sessionName) {
			return "", 0, false, fmt.Sprintf("no such session: %s", sessionName)
		}
		windowTarget := cleanSessionTarget(optionValue(args, "-t", target))
		return "", 0, false, fmt.Sprintf("no such window: %s", windowTarget)
	}
	sessionName, _, _, _, _ := parsePaneTarget(target)
	if sessionName == "" {
		sessionName = currentSession
	}
	if sessionName == "" {
		sessionName = firstSessionName(rt.state)
	}
	if !sessionExists(rt.state, sessionName) {
		return "", 0, false, fmt.Sprintf("no such session: %s", sessionName)
	}
	return sessionName, 0, false, ""
}

func (rt *Runtime) optionKnownOutsideServer(name string, currentSession string) bool {
	global, err := rt.state.OptionsTarget("global", "", 0, false, true)
	if err == nil {
		if _, ok := global[name]; ok {
			return true
		}
	}
	window, err := rt.state.OptionsTarget("global-window", currentSession, 0, false, true)
	if err == nil {
		if _, ok := window[name]; ok {
			return true
		}
	}
	return false
}

func optionHookScope(optionScope string) string {
	switch optionScope {
	case "global", "global-window", "server":
		return "global"
	case "window":
		return "window"
	default:
		return "session"
	}
}

func formatHooks(hooks []model.Hook, valueOnly bool) []string {
	lines := make([]string, 0)
	for _, hook := range hooks {
		if len(hook.Commands) == 0 {
			if !valueOnly {
				lines = append(lines, hook.Name)
			}
			continue
		}
		for index, commandValue := range hook.Commands {
			if valueOnly {
				lines = append(lines, commandValue)
			} else {
				lines = append(lines, fmt.Sprintf("%s[%d] %s", hook.Name, index, commandValue))
			}
		}
	}
	return lines
}

func (rt *Runtime) cmdSetHook(args []string, currentSession string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing hook")
	}
	name := values[0]
	commandValue := ""
	if len(values) > 1 {
		commandValue = strings.Join(values[1:], " ")
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	scope := hookScope(args)
	if hasAny(args, "-R") {
		if _, err := rt.state.Hooks(scope, currentSession, name); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	if err := rt.state.SetHook(scope, currentSession, name, commandValue, hasAny(args, "-a"), hasAny(args, "-u")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdShowHooks(args []string, currentSession string) protocol.Message {
	values := optionOperands(args)
	name := ""
	if len(values) > 0 {
		name = values[0]
	}
	if currentSession == "" {
		currentSession = firstSessionName(rt.state)
	}
	hooks, err := rt.state.Hooks(hookScope(args), currentSession, name)
	if err != nil {
		return fail(err.Error())
	}
	return ok(strings.Join(formatHooks(hooks, false), "\n"))
}

func hookScope(args []string) string {
	switch {
	case hasAny(args, "-g"):
		return "global"
	case hasAny(args, "-w"):
		return "window"
	case hasAny(args, "-p"):
		return "pane"
	default:
		return "session"
	}
}

func (rt *Runtime) cmdSetEnvironment(args []string, currentSession string) protocol.Message {
	values := optionOperands(args)
	if len(values) == 0 {
		return fail("missing variable")
	}
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	targetSession, targetErr := rt.environmentCommandTarget(args, currentSession, scope)
	if targetErr != "" {
		return fail(targetErr)
	}
	name := values[0]
	if hasAny(args, "-u") {
		if err := rt.state.UnsetEnvironment(scope, targetSession, name); err != nil {
			return fail(err.Error())
		}
		return ok("")
	}
	value := ""
	if len(values) > 1 {
		value = strings.Join(values[1:], " ")
	}
	if err := rt.state.SetEnvironment(scope, targetSession, name, value, hasAny(args, "-h"), hasAny(args, "-r")); err != nil {
		return fail(err.Error())
	}
	return ok("")
}

func (rt *Runtime) cmdShowEnvironment(args []string, currentSession string) protocol.Message {
	scope := "session"
	if hasAny(args, "-g") {
		scope = "global"
	}
	targetSession, targetErr := rt.environmentCommandTarget(args, currentSession, scope)
	if targetErr != "" {
		return fail(targetErr)
	}
	showHidden := hasAny(args, "-h")
	env, err := rt.state.Environment(scope, targetSession, showHidden)
	if err != nil {
		return fail(err.Error())
	}
	removals := make(map[string]bool)
	if !showHidden {
		var err error
		removals, err = rt.state.EnvironmentRemovals(scope, targetSession)
		if err != nil {
			return fail(err.Error())
		}
		for name := range removals {
			delete(env, name)
		}
	}
	names := optionOperands(args)
	shellFormat := hasAny(args, "-s")
	if len(names) > 0 {
		if removals[names[0]] {
			if shellFormat {
				return ok(fmt.Sprintf("unset %s;", names[0]))
			}
			return ok("-" + names[0])
		}
		value, exists := env[names[0]]
		if !exists {
			otherEnv, otherErr := rt.state.Environment(scope, targetSession, !showHidden)
			if otherErr == nil {
				if _, existsOther := otherEnv[names[0]]; existsOther {
					return ok("")
				}
			}
			return fail(fmt.Sprintf("unknown variable: %s", names[0]))
		}
		if shellFormat {
			return ok(fmt.Sprintf("%s=%s; export %s;", names[0], shellQuote(value), names[0]))
		}
		return ok(fmt.Sprintf("%s=%s", names[0], value))
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	for key := range removals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		if removals[key] {
			if shellFormat {
				lines = append(lines, fmt.Sprintf("unset %s;", key))
			} else {
				lines = append(lines, "-"+key)
			}
			continue
		}
		if shellFormat {
			lines = append(lines, fmt.Sprintf("%s=%s; export %s;", key, shellQuote(env[key]), key))
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, env[key]))
		}
	}
	return ok(strings.Join(lines, "\n"))
}

func (rt *Runtime) environmentCommandTarget(args []string, currentSession string, scope string) (string, string) {
	if scope == "global" {
		return "", ""
	}
	target := optionValue(args, "-t", currentSession)
	sessionName, _, _, _, _ := parsePaneTarget(target)
	if sessionName == "" {
		sessionName = currentSession
	}
	if sessionName == "" {
		sessionName = firstSessionName(rt.state)
	}
	if !sessionExists(rt.state, sessionName) {
		return "", fmt.Sprintf("no such session: %s", sessionName)
	}
	return sessionName, ""
}
