# gotmux tmux Compatibility Matrix

This file is the release gate for claiming tmux compatibility. A feature is not
considered compatible until it has an implementation and a regression test or a
manual verification note.

## Implemented in v0.1.1

| Area | Status |
| --- | --- |
| Client/server split over Unix socket | Implemented |
| Server-owned PTYs | Implemented |
| Sessions, windows, panes | Implemented |
| Detached sessions | Implemented |
| `new-session` output | Implemented for tmux-style quiet default and basic `-P`/`-F` print output |
| Attach/detach | Implemented |
| Common window and pane commands | Implemented, including basic targeted `select-window -t`, `select-window -n/-p/-l`, `last-window`, `next-window -t`, `previous-window -t`, `select-pane -t`, `select-pane -L/-R/-U/-D`, `select-pane -l`, `last-pane`, `kill-pane -t`/`-a`, `kill-window -t`/`-a`, `unlink-window -k`, `rename-window -t`, explicit-target `swap-window`, basic `new-window -d`/`-P`, basic floating `new-pane`/`newp`, basic `split-window -d`/`-P`, basic `resize-window`, basic `move-window`/renumber, and basic `respawn-pane`/`respawn-window -k` |
| Common `C-b` prefix bindings | Implemented |
| Common command aliases | Implemented for covered commands, including `display`, `rename`, `renamew`, `send`, and `detach` |
| Basic format expansion for `list-*` and `display-message` | Implemented for the fields, basic `list-windows -a`, `list-panes -a/-s`, `list-panes -t session:window`, basic `list-sessions`/`list-windows`/`list-panes -f` truthy filters, basic `display-message -p message`, and basic `display-message -t pane` targets covered by `scripts/compat_probe.sh` |
| Basic client listing | Implemented for `list-clients`/`lsc` over attached gotmux clients with basic format fields; the probe covers tmux-compatible empty detached-server output |
| Basic command sequences | Implemented for semicolon-separated command sequences covered by `scripts/compat_probe.sh` |
| Basic command metadata | Implemented for `list-commands`/`lscm` over currently supported commands plus `start-server`; the probe covers formatted command-list fields and alias lookup |
| Basic shell/config commands | Implemented for `run-shell`/`run` synchronous output, `-b`, `-E`, simple `-C`, `-c`, and `-d`, plus basic `if-shell`/`if` true/false and `-F` branches; the probe covers stdout, stderr opt-in, background mode, shell exit status, and conditional branch execution |
| Basic pane piping | Implemented for `pipe-pane`/`pipep` output piping to a shell command, empty-command close, `-o` toggle behavior, and basic `-I` command-output-to-pane input |
| Basic synchronization commands | Implemented for `wait-for`/`wait` signal, wait, lock, and unlock channels; the probe covers signal-before-wait and lock/unlock output, with goroutine tests for blocking wakeups |
| Basic lock commands | Implemented for `lock-server`/`lock`, `lock-session`/`locks` no-output success and `lock-client` no-current-client errors in the detached CLI subset |
| Basic client refresh command | Implemented for `refresh-client`/`refresh` metadata, no-current-client CLI errors, and attached-client no-op success for redraw bindings |
| Basic prompt history commands | Implemented for empty `show-prompt-history`/`showphist` output, `-T` filtering, and stateless `clear-prompt-history`/`clearphist`; the probe covers empty histories and invalid type errors |
| Basic message inspection commands | Implemented for `show-messages`/`showmsgs` command metadata and empty `-J`/`-T` jobs/terminal output |
| `source-file` | Implemented for simple command files and line continuations |
| Explicit `-f` startup config | Implemented for server startup |
| Default `.tmux.conf` discovery | Implemented for `$HOME/.tmux.conf` when starting a new server |
| Basic options | Implemented for string-backed `set-option`, `set-window-option`, `show-options`, and `show-window-options`/`showw` |
| Basic environment commands | Implemented for `set-environment`, `show-environment`, `-g`, `-u`, and `-s` in the probe subset |
| Basic key bindings | Implemented for `bind-key`, `unbind-key`, `list-keys`, prefix dispatch, root table dispatch for simple keys, and `send-prefix` |
| Basic key sending | Implemented for targeted `send-keys -t` and simple `-N` repeats in the probe subset |
| Basic pane geometry | Implemented for simple horizontal/vertical splits, nested splits, targeted `resize-pane -t`, targeted `select-layout -t` built-in layouts, basic next/previous layout cycling, basic same-window `swap-pane`, basic `rotate-window`, basic `break-pane`, and basic `join-pane`/`move-pane` in the tested subset |
| Basic terminal screen grid | Implemented for common printable output, cursor movement, clear line/screen, insertion/deletion, scrolling, and alternate-screen escape sequences |
| Basic multi-pane redraw | Implemented for screen-backed pane snapshots with simple ASCII borders; full tmux-style redraw is not complete |
| Basic `capture-pane` | Implemented for `-p`, visible screen lines, simple `-S`/`-E` ranges, simple pane targets in the probe subset, basic `-N`/`-T` whitespace handling, basic `-J` wrapped-line joining, visible-line `-F`/`-L` prefixes, and basic `-C` visible text escaping |
| Basic `clear-history` | Implemented for clearing gotmux's pane history ring; the visible screen snapshot is left unchanged |
| Basic paste buffers | Implemented for `set-buffer`, `set-buffer -n`, `show-buffer`, `list-buffers`, `delete-buffer`, `paste-buffer`, `load-buffer`, `save-buffer`, and `capture-pane -b`; the probe covers set/show/list/rename/delete, file load/save, and capture-to-buffer |
| tmux/gotmux automated behavior probe | Implemented for the first CLI, format, option, binding, environment, source-file, default-config, command-sequence, pane-geometry, `capture-pane`, and buffer subsets |
| macOS/Linux static Go builds | Implemented |

## Not Yet Compatible With tmux

| Area | Gap |
| --- | --- |
| Full command parser | Advanced tmux quoting, parse-time formats, command queues, `%if`, includes, and full target resolution are incomplete. |
| Full format language | Only a small set of session/window/pane fields is implemented. Modifiers, conditionals, expressions, time formats, loops, and style expansion are not implemented. |
| Full list filtering | Basic truthy filters over implemented format fields exist, but tmux's full format expression language for `-f` filters is incomplete. |
| Full layout rendering | Pane geometry and basic redraw exist for a small subset, but full layout algorithms, complete floating pane behavior, tmux-style border cells, zoom, custom layouts, and pane movement are incomplete. |
| Screen model | A basic grid, common CSI parser, alternate-screen switching, and a separate byte history ring exist, but full tmux-style scrollback grid, styles/colors, wide-character handling, redraw diffing, and terminal capability negotiation are incomplete. |
| Copy mode | Not implemented. |
| Mouse support | Not implemented. |
| Full buffers and paste buffers | Basic in-memory buffers and file load/save exist, but `choose-buffer`, buffer limits, stack pruning, copy-mode integration, and complete paste options are incomplete. |
| Full `capture-pane` semantics | History ranges, alternate screen selection, mode screen capture, paste buffers, complete `-J`/`-T` behavior across history and mode grids, full escape/style output, hyperlinks, all tmux line flags, and complete target resolution are incomplete. |
| Full option semantics | Only a small string-backed subset exists. Most documented options and option side effects are not implemented. |
| Full environment semantics | Hidden variables, remove markers, update-environment integration, and complete session/global behavior are incomplete. |
| Key tables and custom bindings | Prefix/root table dispatch exists for simple bindings, but full key tables, repeat behavior, notes, mode tables, and robust multi-command bindings are incomplete. |
| Status format language | Only a simple status line is implemented. |
| Hooks and jobs | Hooks are not implemented; jobs are limited to the basic `run-shell` subset. |
| Full `pipe-pane` semantics | Basic output piping is implemented, but complete bidirectional lifecycle edge cases, offset replay, format expansion, pane destruction integration, and all failure modes remain incomplete. |
| Message log | `show-messages` does not yet keep tmux-style timestamped command/message history. |
| Control mode | Not implemented. |
| Full `refresh-client` semantics | Basic attached-client success exists, but client panning, control-mode subscriptions, reports, size updates, and flag updates remain incomplete. |
| Lock screen behavior | Lock commands are recognized for the basic CLI subset, but gotmux does not yet provide tmux-style lock screens. |
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
