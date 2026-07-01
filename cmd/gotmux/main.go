package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fanhuadesenlinnn/gotmux/internal/client"
	cmdparse "github.com/fanhuadesenlinnn/gotmux/internal/command"
	"github.com/fanhuadesenlinnn/gotmux/internal/config"
	"github.com/fanhuadesenlinnn/gotmux/internal/daemon"
	"github.com/fanhuadesenlinnn/gotmux/internal/server"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

const version = "gotmux 0.1.1"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	global, command, err := parseGlobal(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	socketPath, err := config.SocketPath(global.label, global.socket)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if global.version {
		fmt.Println(version)
		return 0
	}
	if global.help {
		usage()
		return 0
	}
	configFiles := global.configFiles
	if len(configFiles) == 0 {
		configFiles = config.DefaultConfigFiles()
	}
	if global.server {
		if err := server.Run(context.Background(), socketPath, configFiles); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if len(command) == 0 {
		command = []string{"new-session"}
	}
	commands, err := cmdparse.ParseArgv(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(commands) == 0 {
		commands = [][]string{{"new-session"}}
	}
	name := normalize(commands[0][0])
	switch name {
	case "new-session":
		if err := daemon.Ensure(socketPath, configFiles); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		width, height := terminal.Size(int(os.Stdout.Fd()))
		result, err := daemon.SendCommands(socketPath, commands, "", width, height)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !result.OK {
			fmt.Fprintln(os.Stderr, result.Text)
			return code(result.Code)
		}
		if detached(commands[0]) || len(commands) > 1 {
			if result.Text != "" {
				fmt.Println(result.Text)
			}
			return 0
		}
		if err := client.Attach(socketPath, result.Session); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "attach-session":
		if _, err := daemon.Dial(socketPath); err != nil {
			fmt.Fprintf(os.Stderr, "no server running on %s\n", socketPath)
			return 1
		}
		target := optionValue(command[1:], "-t", "")
		if err := client.Attach(socketPath, cleanTarget(target)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		if _, err := daemon.Dial(socketPath); err != nil {
			fmt.Fprintf(os.Stderr, "no server running on %s\n", socketPath)
			return 1
		}
		width, height := terminal.Size(int(os.Stdout.Fd()))
		result, err := daemon.SendCommands(socketPath, commands, cleanTarget(optionValue(commands[0][1:], "-t", "")), width, height)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if result.Text != "" {
			fmt.Println(result.Text)
		}
		if !result.OK {
			return code(result.Code)
		}
		return 0
	}
}

type globals struct {
	socket      string
	label       string
	configFiles []string
	server      bool
	version     bool
	help        bool
}

func parseGlobal(args []string) (globals, []string, error) {
	var g globals
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--server":
			g.server = true
		case "-S":
			i++
			if i >= len(args) {
				return g, nil, fmt.Errorf("-S requires a socket path")
			}
			g.socket = args[i]
		case "-L":
			i++
			if i >= len(args) {
				return g, nil, fmt.Errorf("-L requires a socket label")
			}
			g.label = args[i]
		case "-f":
			i++
			if i >= len(args) {
				return g, nil, fmt.Errorf("-f requires a file")
			}
			g.configFiles = append(g.configFiles, args[i])
		case "-V":
			g.version = true
		case "-h", "--help":
			g.help = true
		default:
			return g, args[i:], nil
		}
	}
	return g, nil, nil
}

func normalize(name string) string {
	switch name {
	case "new":
		return "new-session"
	case "attach", "at":
		return "attach-session"
	default:
		return name
	}
}

func detached(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "-d" {
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "d") {
			return true
		}
	}
	return false
}

func optionValue(args []string, name string, fallback string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return fallback
}

func cleanTarget(target string) string {
	target = strings.TrimPrefix(target, "=")
	target = strings.TrimPrefix(target, "$")
	return target
}

func code(n int) int {
	if n == 0 {
		return 1
	}
	return n
}

func usage() {
	fmt.Println("usage: gotmux [-L socket-name] [-S socket-path] [command [flags]]")
	fmt.Println("commands: new-session, attach-session, list-sessions, new-window, split-window, select-window, select-pane, kill-pane, kill-window, kill-session, kill-server")
}
