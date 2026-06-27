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
tmux_config_sock="gotmux-compat-tmux-config-$$"
gotmux_config_sock="/tmp/gotmux-compat-gotmux-config-$$.sock"

cleanup() {
  "${TMUX_BIN}" -L "${tmux_sock}" kill-server >/dev/null 2>&1 || true
  "${TMUX_BIN}" -L "${tmux_config_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_config_sock}" kill-server >/dev/null 2>&1 || true
  rm -f "${gotmux_sock}"
  rm -f "${gotmux_config_sock}"
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

compare_normalized() {
  local name="$1"
  shift
  local tmux_output gotmux_output
  tmux_output="$("${tmux_cmd[@]}" "$@" | sed -E 's/[[:space:]]+/ /g')"
  gotmux_output="$("${BIN}" -S "${gotmux_sock}" "$@" | sed -E 's/[[:space:]]+/ /g')"
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

compare_key_line() {
  local name="$1"
  local key="$2"
  local tmux_output gotmux_output
  tmux_output="$("${tmux_cmd[@]}" list-keys -T prefix | grep -E "[[:space:]]${key}[[:space:]]" | sed -E 's/[[:space:]]+/ /g')"
  gotmux_output="$("${BIN}" -S "${gotmux_sock}" list-keys -T prefix | grep -E "[[:space:]]${key}[[:space:]]" | sed -E 's/[[:space:]]+/ /g')"
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

"${tmux_cmd[@]}" set -g status off
"${BIN}" -S "${gotmux_sock}" set -g status off >/dev/null
compare "show global option" show -g status
compare "show global option value" show -gqv status

"${tmux_cmd[@]}" setw -g mode-keys vi
"${BIN}" -S "${gotmux_sock}" setw -g mode-keys vi >/dev/null
compare "show global window option" show -gw mode-keys

"${tmux_cmd[@]}" bind-key C-a send-prefix
"${BIN}" -S "${gotmux_sock}" bind-key C-a send-prefix >/dev/null
compare_key_line "list custom key" C-a

source_file="$(mktemp)"
printf 'set -g status on\nnew-window -n sourced /bin/sh\n' > "${source_file}"
"${tmux_cmd[@]}" source-file "${source_file}"
"${BIN}" -S "${gotmux_sock}" source-file "${source_file}" >/dev/null
rm -f "${source_file}"
compare "source-file option" show -gqv status
compare "source-file window" list-windows -t compat -F "#{window_index}:#{window_name}:#{window_panes}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s seq -n first /bin/sh \; new-window -t seq -n second /bin/sh
"${BIN}" -S "${gotmux_sock}" new-session -d -s seq -n first /bin/sh \; new-window -t seq -n second /bin/sh >/dev/null
compare "command sequence" list-windows -t seq -F "#{window_index}:#{window_name}"

config_file="$(mktemp)"
printf 'set -g status off\nbind-key C-a send-prefix\n' > "${config_file}"
tmux_config_cmd=("${TMUX_BIN}" -f "${config_file}" -L "${tmux_config_sock}")
"${tmux_config_cmd[@]}" new-session -d -s configured -n first /bin/sh
"${BIN}" -S "${gotmux_config_sock}" -f "${config_file}" new-session -d -s configured -n first /bin/sh >/dev/null
tmux_config_status="$("${tmux_config_cmd[@]}" show -gqv status)"
gotmux_config_status="$("${BIN}" -S "${gotmux_config_sock}" show -gqv status)"
rm -f "${config_file}"
if [[ "${tmux_config_status}" != "${gotmux_config_status}" ]]; then
  echo "compat probe failed: startup config" >&2
  echo "--- tmux" >&2
  printf '%s\n' "${tmux_config_status}" >&2
  echo "--- gotmux" >&2
  printf '%s\n' "${gotmux_config_status}" >&2
  exit 1
fi
printf 'ok startup config\n'
