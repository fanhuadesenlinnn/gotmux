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
| macOS/Linux static Go builds | Implemented |

## Not Yet Compatible With tmux

| Area | Gap |
| --- | --- |
| Full command parser | tmux command language, quoting, formats, command queues, and target resolution are incomplete. |
| Layout rendering | Multiple panes exist, but tiled split rendering is not equivalent to tmux yet. |
| Screen model | Full grid, scrollback, alternate screen, redraw diffing, and terminal escape interpretation are incomplete. |
| Copy mode | Not implemented. |
| Mouse support | Not implemented. |
| Buffers and paste buffers | Not implemented. |
| Options | Global/session/window/user options are not implemented. |
| Key tables and custom bindings | Only built-in prefix bindings are implemented. |
| `.tmux.conf` / `source-file` | Not implemented. |
| Status format language | Only a simple status line is implemented. |
| Hooks and formats | Not implemented. |
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
