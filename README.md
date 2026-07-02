# gotmux

gotmux is a Go reimplementation of the core tmux terminal multiplexer model:
a small client talks to a long-running local server over a Unix socket, and the
server owns sessions, windows, panes, and PTYs.

The project is intentionally scoped to tmux compatibility. It does not add
features that tmux does not have.

## Current Compatibility

This first implementation supports:

- `new-session` / `new`
- `attach-session` / `attach` / `at`
- `list-sessions` / `ls`
- `new-window` / `neww`
- `split-window` / `splitw`
- `list-windows` / `lsw`
- `list-panes` / `lsp`
- `select-window`, `next-window`, `previous-window`
- `select-pane`
- `send-keys`
- `rename-session`, `rename-window`
- `kill-pane`, `kill-window`, `kill-session`, `kill-server`
- interactive prefix key `C-b` for common window and pane operations
- detached sessions with shell processes running in PTYs
- tmux-style default pane cleanup when shell processes exit
- basic tmux format expansion for `list-sessions`, `list-windows`,
  `list-panes`, and `display-message`
- basic command sequences separated by `;`
- `source-file`, explicit `-f` startup config loading, and a first subset of
  `set-option`, `show-options`, `set-window-option`, `bind-key`,
  `unbind-key`, and `list-keys`
- default `$HOME/.tmux.conf` discovery when starting a new server
- a first subset of `set-environment` / `show-environment`
- pane geometry tracking for basic horizontal/vertical splits, `resize-pane`,
  and `select-layout even-horizontal`
- basic multi-pane redraw with borders from pane history snapshots
- macOS and Linux builds with `CGO_ENABLED=0`

Full tmux parity is not complete yet. See [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

## Install

```sh
go install github.com/fanhuadesenlinnn/gotmux/cmd/gotmux@latest
```

## Examples

```sh
gotmux new-session -s work
gotmux new-session -d -s build
gotmux attach -t work
gotmux ls
gotmux new-window -t work -n logs
gotmux split-window -t work
gotmux -f ./tmux.conf new-session -d -s configured
gotmux new-session -d -s demo \; new-window -t demo -n logs
```

Inside an attached client, the default prefix is `C-b`:

- `C-b c`: new window
- `C-b "`: split window
- `C-b %`: horizontal split command path
- `C-b n` / `C-b p`: next or previous window
- `C-b 0`..`9`: select window
- `C-b o`: select next pane
- `C-b x`: kill active pane
- `C-b d`: detach

## Release

Pushing a tag like `v0.1.0` builds Linux and macOS artifacts through GitHub
Actions and publishes them to a GitHub Release.

## Compatibility Probe

Use the local tmux binary as the behavior oracle for the currently automated
compatibility subset:

```sh
scripts/compat_probe.sh
```

The probe starts isolated tmux and gotmux servers, creates matching sessions,
windows, and panes, then compares format-driven `list-*`, `display-message`,
basic options, key bindings, shell exit pane cleanup, `source-file`, and command sequence behavior.
It also checks the first pane geometry subset for split, resize, and layout
commands.
