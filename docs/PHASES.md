# gotmux Full tmux Parity Phases

The goal is full tmux behavior parity, verified against upstream tmux and the
local `tmux` binary. This is the working phase map; it should grow more
specific as each area gets implemented.

## Phase 1: Observable CLI Baseline

- Build a repeatable tmux/gotmux comparison probe.
- Match basic session/window/pane lifecycle commands.
- Match simple format output for `list-sessions`, `list-windows`,
  `list-panes`, and `display-message`.

Status: complete for the current probe subset, including basic truthy
`list-sessions`/`list-windows`/`list-panes -f` filters over implemented format
fields.

## Phase 2: Command Language and Configuration

- Implement tmux-style command parsing, command sequences, quoting, escaping,
  aliases, and target resolution.
- Implement `.tmux.conf` discovery and `source-file`.
- Implement `set-option`, `show-options`, `set-environment`,
  `show-environment`, `bind-key`, `unbind-key`, and `list-keys`.

Status: in progress. Basic command sequences, explicit `-f`, default
`$HOME/.tmux.conf` discovery, `source-file`, basic `list-commands` metadata for
all local tmux command names, basic `list-clients`, string-backed options with
global/local unset, string append, `show-options -A`, and basic `-t` targets, basic
`set-hook`/`show-hooks` hook option storage and display, basic environment
commands, prefix/root key bindings, `send-prefix`, basic `run-shell` shell
execution, basic `if-shell` conditionals, and basic `wait-for` synchronization
are implemented and covered by `scripts/compat_probe.sh`. Basic empty prompt
history commands are also covered. Full tmux syntax, hook firing side effects,
environment edge cases, and complete option/key semantics remain.

## Phase 3: Terminal Screen Model

- Implement a real grid/screen model, alternate screen handling, scrollback,
  redraw diffing, cursor state, styles, colors, and terminal capability
  negotiation.
- Replace raw active-pane passthrough with tmux-style rendered panes.

Status: in progress. A basic pane screen grid now tracks printable output,
cursor movement, clear line/screen, insertion/deletion, and scrolling escape
sequences. It also supports basic alternate-screen switching for full-screen
programs, and multi-pane redraw uses screen snapshots when available. Full
scrollback integration, styles/colors, wide characters, redraw diffing, and
terminal capability negotiation remain.

## Phase 4: Layouts and Panes

- Implement tmux layout tree behavior, pane borders, tiled rendering, resize
  commands, zoom, select-layout, rotate/swap/join/break pane behavior.

Status: in progress. Basic split geometry, nested split geometry, targeted
`resize-pane -t`, targeted `select-layout -t` for tmux built-in layouts, basic
next/previous layout cycling, basic same-window `swap-pane`, basic
`rotate-window`, basic non-floating `break-pane`, basic tiled `join-pane` and
`move-pane`, and screen-backed multi-pane redraw with simple borders are
implemented. Basic targeted
`select-window -t`, `select-pane -t`, `kill-pane -t`, and `kill-window -t`
operate on explicit targets, drop related screen snapshots where needed, and
collapse pane layout leaves; targeted `rename-window -t` renames non-active
target windows, explicit-target `swap-window` swaps window positions, and basic
`move-window` moves and renumbers windows. `last-window` and `select-window -l`
track and switch back to the previous window in the probe subset. `new-window -d`
and `split-window -d` keep the existing active window or pane, and basic `-P`
creation output is compared against tmux. Basic floating `new-pane`/`newp`
creates tmux-style default floating panes and supports explicit size/position
values in the probe subset. `select-pane -L/-R/-U/-D`,
`select-pane -l`, and `last-pane` cover direction and last-pane selection in
the tested geometry subset. Basic `resize-window -x/-y` and directional size
adjustments recalculate pane layouts. `kill-pane -a` and `kill-window -a` keep
the target pane or window and remove the rest in the probed subset. Basic
`unlink-window -k` removes single-link windows while plain `unlink-window`
returns tmux's single-link error. Basic `link-window` links a window into
another session or index, supports `-d` and `-k` in the probed subset, and
plain `unlink-window` now removes linked windows without killing the shared
pane state. Basic `respawn-pane -k` and `respawn-window -k` restart targeted
panes/windows.
Geometry is compared with tmux through `scripts/compat_probe.sh`;
custom layout strings, old-layout restore, marked pane defaults, complete
floating pane movement and mode behavior, target-index pane moves, join/move
size and placement flags, tmux-style border
rendering, zoom, full winlink/session-group semantics, and pane movement
commands remain.

## Phase 5: Modes, Buffers, and Input

- Implement copy-mode, view-mode, paste buffers, capture-pane, choose-tree,
  prompts, menus, mouse handling, and key tables.

Status: in progress. Basic `capture-pane -p` now captures visible screen
lines with simple `-S`/`-E` ranges and is compared with tmux in
`scripts/compat_probe.sh`. Screen cells also track written positions for basic
`capture-pane -N`/`-T` whitespace handling and tmux-style wrapped-line flags
for basic `capture-pane -J` joining. Visible-line `capture-pane -F` flags and
`-L` line numbers are implemented in the same probe subset, along with basic
`capture-pane -C` escaping for visible backslashes. Basic in-memory paste buffers
now cover `set-buffer`, `show-buffer`, `list-buffers`, `delete-buffer`,
`paste-buffer`, `load-buffer`, `save-buffer`, `capture-pane -b`, and
`set-buffer -n` buffer renames. `list-buffers` now also supports basic truthy
`-f` filters and `-O name`/`-O size` sort orders in the probed subset. Basic
`clear-history` clears gotmux's pane history ring. Prompt-history commands expose
the tmux-compatible empty-history shape, and `show-messages` covers command
metadata plus empty `-J`/`-T` jobs/terminal output. Basic mode entry commands
now recognize `clock-mode`, `copy-mode`, `choose-buffer`, `choose-client`,
`choose-tree`, `customize-mode`, and `find-window` with tmux-compatible empty
CLI success in the probed subset; `command-prompt`, `display-panes`, and
`suspend-client` match detached CLI no-current-client errors. `display-menu`,
`display-popup`, and `confirm-before` now match the same detached-client error
shape. Basic `server-access` covers owner listing and in-memory user ACL
changes in the probed subset. History capture, message log, buffer chooser UI,
copy mode internals, mode screens, complete joined/wrapped output, style/escape
output, and complete target resolution remain.

## Phase 6: Advanced tmux Surfaces

- Implement hooks, control mode, popups, session groups, linked windows,
  alerts, jobs, run-shell, pipe-pane, wait-for, locks, and remaining command
  families.

Status: in progress. `run-shell`, `if-shell`, and `wait-for` cover the tested
command/job subset. Basic `pipe-pane` output piping now writes pane output to a
shell command, closes on empty commands, and supports the common `-o` toggle
path in the compatibility probe. Full bidirectional pipe lifecycle behavior,
`lock-server` and `lock-session` cover the basic detached CLI no-op behavior.
`refresh-client` covers no-current-client CLI errors and attached-client redraw
entry points. Basic `switch-client` covers current-client requirements,
missing `-c target-client` errors, attached-client `-t` target-session
switching, `-n`/`-p` relative switching, `-l` last-session switching, and
basic `-c client-N` target-client switching. Full lock screens, bidirectional
pipe lifecycle behavior, refresh offset/control behavior, full switch-client
key-table/read-only/sorted-order behavior, hook execution ordering and context,
control mode, popups, and linked windows remain.

## Completion Gate

gotmux is not considered a full tmux clone until the compatibility probe and
manual verification cover every tmux command, default key binding, documented
option, format expression category, control-mode behavior, and interactive
terminal mode across macOS and Linux.
