# gotmux Full tmux Parity Phases

The goal is full tmux behavior parity, verified against upstream tmux and the
local `tmux` binary. This is the working phase map; it should grow more
specific as each area gets implemented.

## Phase 1: Observable CLI Baseline

- Build a repeatable tmux/gotmux comparison probe.
- Match basic session/window/pane lifecycle commands.
- Match simple format output for `list-sessions`, `list-windows`,
  `list-panes`, and `display-message`.

Status: complete for the current probe subset.

## Phase 2: Command Language and Configuration

- Implement tmux-style command parsing, command sequences, quoting, escaping,
  aliases, and target resolution.
- Implement `.tmux.conf` discovery and `source-file`.
- Implement `set-option`, `show-options`, `set-environment`,
  `show-environment`, `bind-key`, `unbind-key`, and `list-keys`.

Status: in progress. Basic command sequences, explicit `-f`, default
`$HOME/.tmux.conf` discovery, `source-file`, string-backed options, basic
environment commands, prefix/root key bindings, and `send-prefix` are
implemented and covered by `scripts/compat_probe.sh`. Full tmux syntax,
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
`rotate-window`, basic non-floating `break-pane`, and screen-backed multi-pane
redraw with simple borders are implemented. Basic targeted
`select-window -t`, `select-pane -t`, `kill-pane -t`, and `kill-window -t`
operate on explicit targets, drop related screen snapshots where needed, and
collapse pane layout leaves; targeted `rename-window -t` renames non-active
target windows. Geometry is compared with tmux through `scripts/compat_probe.sh`;
custom layout strings, old-layout restore, marked pane defaults, floating panes,
target-index pane moves, tmux-style border rendering, zoom, and pane movement
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
`paste-buffer`, `load-buffer`, `save-buffer`, and `capture-pane -b`. Basic
`clear-history` clears gotmux's pane history ring. History capture, buffer
chooser, copy mode, mode screens, complete joined/wrapped output, style/escape
output, and complete target resolution remain.

## Phase 6: Advanced tmux Surfaces

- Implement hooks, control mode, popups, session groups, linked windows,
  alerts, jobs, run-shell, pipe-pane, wait-for, locks, and remaining command
  families.

## Completion Gate

gotmux is not considered a full tmux clone until the compatibility probe and
manual verification cover every tmux command, default key binding, documented
option, format expression category, control-mode behavior, and interactive
terminal mode across macOS and Linux.
