# gotmux Full tmux Parity Phases

The goal is full tmux behavior parity, verified against upstream tmux and the
local `tmux` binary. This is the working phase map; it should grow more
specific as each area gets implemented.

## Phase 1: Observable CLI Baseline

- Build a repeatable tmux/gotmux comparison probe.
- Match basic session/window/pane lifecycle commands.
- Match simple format output for `list-sessions`, `list-windows`,
  `list-panes`, and `display-message`.

Status: in progress. The current automated probe passes for this subset.

## Phase 2: Command Language and Configuration

- Implement tmux-style command parsing, command sequences, quoting, escaping,
  aliases, and target resolution.
- Implement `.tmux.conf` discovery and `source-file`.
- Implement `set-option`, `show-options`, `set-environment`,
  `show-environment`, `bind-key`, `unbind-key`, and `list-keys`.

## Phase 3: Terminal Screen Model

- Implement a real grid/screen model, alternate screen handling, scrollback,
  redraw diffing, cursor state, styles, colors, and terminal capability
  negotiation.
- Replace raw active-pane passthrough with tmux-style rendered panes.

## Phase 4: Layouts and Panes

- Implement tmux layout tree behavior, pane borders, tiled rendering, resize
  commands, zoom, select-layout, rotate/swap/join/break pane behavior.

## Phase 5: Modes, Buffers, and Input

- Implement copy-mode, view-mode, paste buffers, capture-pane, choose-tree,
  prompts, menus, mouse handling, and key tables.

## Phase 6: Advanced tmux Surfaces

- Implement hooks, control mode, popups, session groups, linked windows,
  alerts, jobs, run-shell, pipe-pane, wait-for, locks, and remaining command
  families.

## Completion Gate

gotmux is not considered a full tmux clone until the compatibility probe and
manual verification cover every tmux command, default key binding, documented
option, format expression category, control-mode behavior, and interactive
terminal mode across macOS and Linux.
