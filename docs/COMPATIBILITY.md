# gotmux tmux Compatibility Matrix

This file is the release gate for claiming tmux compatibility. A feature is not
considered compatible until it has an implementation and a regression test or a
manual verification note.

## Implemented in v0.1.0

| Area | Status |
| --- | --- |
| Client/server split over Unix socket | Implemented |
| Server-owned PTYs | Implemented |
| Sessions, windows, panes | Implemented |
| Detached sessions | Implemented |
| Attach/detach | Implemented |
| Common window and pane commands | Implemented |
| Common `C-b` prefix bindings | Implemented |
| Basic format expansion for `list-*` and `display-message` | Implemented for the fields covered by `scripts/compat_probe.sh` |
| Basic command sequences | Implemented for semicolon-separated command sequences covered by `scripts/compat_probe.sh` |
| `source-file` | Implemented for simple command files and line continuations |
| Explicit `-f` startup config | Implemented for server startup, not yet default `.tmux.conf` discovery |
| Basic options | Implemented for string-backed `set-option`, `set-window-option`, and `show-options` |
| Basic key bindings | Implemented for `bind-key`, `unbind-key`, `list-keys`, and prefix dispatch |
| tmux/gotmux automated behavior probe | Implemented for the first CLI, format, option, binding, source-file, and command-sequence subsets |
| macOS/Linux static Go builds | Implemented |

## Not Yet Compatible With tmux

| Area | Gap |
| --- | --- |
| Full command parser | Advanced tmux quoting, parse-time formats, command queues, `%if`, includes, and full target resolution are incomplete. |
| Full format language | Only a small set of session/window/pane fields is implemented. Modifiers, conditionals, expressions, time formats, loops, and style expansion are not implemented. |
| Layout rendering | Multiple panes exist, but tiled split rendering is not equivalent to tmux yet. |
| Screen model | Full grid, scrollback, alternate screen, redraw diffing, and terminal escape interpretation are incomplete. |
| Copy mode | Not implemented. |
| Mouse support | Not implemented. |
| Buffers and paste buffers | Not implemented. |
| Full option semantics | Only a small string-backed subset exists. Most documented options and option side effects are not implemented. |
| Key tables and custom bindings | Prefix/root table storage exists, but full key tables, repeat behavior, notes, mode tables, and multi-command bindings are incomplete. |
| Default `.tmux.conf` discovery | Not implemented. |
| Status format language | Only a simple status line is implemented. |
| Hooks | Not implemented. |
| Control mode | Not implemented. |
| Popups, menus, choose tree, command prompt | Not implemented. |
| Session groups and linked windows | Not implemented. |
| tmux terminfo feature negotiation | Minimal `TERM=screen-256color` only. |

## Source Notes

The first architecture pass follows tmux's main separation of concerns:

- `tmux.c`, `client.c`, `proc.c`: client startup and local IPC.
- `server.c`, `server-client.c`: server event loop and attached client handling.
- `session.c`, `window.c`, `layout.c`: session/window/pane ownership.
- `cmd-*.c`: command table.
- `key-bindings.c`: default prefix key behavior.
- `tty.c`, `input.c`, `screen*.c`, `status.c`: terminal I/O and rendering.

gotmux mirrors that shape with Go packages under `internal/`.
