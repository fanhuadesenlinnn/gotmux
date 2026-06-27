#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${ROOT}/dist/gotmux-compat"
TMUX_BIN="${TMUX_BIN:-tmux}"

if ! command -v "${TMUX_BIN}" >/dev/null 2>&1; then
  echo "tmux not found; set TMUX_BIN to a tmux executable" >&2
  exit 1
fi

mkdir -p "${ROOT}/dist"
go build -o "${BIN}" "${ROOT}/cmd/gotmux"

tmux_sock="gotmux-compat-tmux-$$"
gotmux_sock="/tmp/gotmux-compat-gotmux-$$.sock"

cleanup() {
  "${TMUX_BIN}" -L "${tmux_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_sock}" kill-server >/dev/null 2>&1 || true
  rm -f "${gotmux_sock}"
}
trap cleanup EXIT

tmux_cmd=("${TMUX_BIN}" -f /dev/null -L "${tmux_sock}")

"${tmux_cmd[@]}" new-session -d -s compat -n first -x 80 -y 24 /bin/sh
"${tmux_cmd[@]}" new-window -t compat -n second /bin/sh
"${tmux_cmd[@]}" split-window -t compat -h /bin/sh

"${BIN}" -S "${gotmux_sock}" new-session -d -s compat -n first /bin/sh >/dev/null
"${BIN}" -S "${gotmux_sock}" new-window -t compat -n second /bin/sh >/dev/null
"${BIN}" -S "${gotmux_sock}" split-window -t compat -h /bin/sh >/dev/null

compare() {
  local name="$1"
  shift
  local tmux_output gotmux_output
  tmux_output="$("${tmux_cmd[@]}" "$@")"
  gotmux_output="$("${BIN}" -S "${gotmux_sock}" "$@")"
  if [[ "${tmux_output}" != "${gotmux_output}" ]]; then
    echo "compat probe failed: ${name}" >&2
    echo "--- tmux" >&2
    printf '%s\n' "${tmux_output}" >&2
    echo "--- gotmux" >&2
    printf '%s\n' "${gotmux_output}" >&2
    exit 1
  fi
  printf 'ok %s\n' "${name}"
}

compare "list-sessions formats" list-sessions -F "#{session_name}:#{session_windows}:#{session_attached}"
compare "list-windows formats" list-windows -t compat -F "#{window_index}:#{window_name}:#{window_panes}:#{window_active}"
compare "list-panes formats" list-panes -t compat -F "#{pane_index}:#{pane_active}"
compare "display-message formats" display-message -p -t compat -F "#{session_name}:#{window_index}:#{window_name}:#{pane_index}"
