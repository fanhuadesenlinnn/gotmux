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
