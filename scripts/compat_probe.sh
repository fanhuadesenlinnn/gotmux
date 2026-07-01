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
tmux_default_config_sock="gotmux-compat-tmux-default-config-$$"
gotmux_default_config_sock="/tmp/gotmux-compat-gotmux-default-config-$$.sock"

cleanup() {
  "${TMUX_BIN}" -L "${tmux_sock}" kill-server >/dev/null 2>&1 || true
  "${TMUX_BIN}" -L "${tmux_config_sock}" kill-server >/dev/null 2>&1 || true
  "${TMUX_BIN}" -L "${tmux_default_config_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_config_sock}" kill-server >/dev/null 2>&1 || true
  "${BIN}" -S "${gotmux_default_config_sock}" kill-server >/dev/null 2>&1 || true
  rm -f "${gotmux_sock}"
  rm -f "${gotmux_config_sock}"
  rm -f "${gotmux_default_config_sock}"
}
trap cleanup EXIT

tmux_cmd=("${TMUX_BIN}" -f /dev/null -L "${tmux_sock}")
gotmux_cmd=("${BIN}" -S "${gotmux_sock}" -f /dev/null)

"${tmux_cmd[@]}" new-session -d -s compat -n first -x 80 -y 24 /bin/sh
"${tmux_cmd[@]}" new-window -t compat -n second /bin/sh
"${tmux_cmd[@]}" split-window -t compat -h /bin/sh

"${gotmux_cmd[@]}" new-session -d -s compat -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t compat -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t compat -h /bin/sh >/dev/null

compare() {
  local name="$1"
  shift
  local tmux_output gotmux_output
  tmux_output="$("${tmux_cmd[@]}" "$@")"
  gotmux_output="$("${gotmux_cmd[@]}" "$@")"
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

compare_status() {
  local name="$1"
  shift
  local tmux_output gotmux_output tmux_status gotmux_status
  set +e
  tmux_output="$("${tmux_cmd[@]}" "$@" 2>&1)"
  tmux_status=$?
  gotmux_output="$("${gotmux_cmd[@]}" "$@" 2>&1)"
  gotmux_status=$?
  set -e
  if [[ "${tmux_status}" != "${gotmux_status}" || "${tmux_output}" != "${gotmux_output}" ]]; then
    echo "compat probe failed: ${name}" >&2
    echo "--- tmux status ${tmux_status}" >&2
    printf '%s\n' "${tmux_output}" >&2
    echo "--- gotmux status ${gotmux_status}" >&2
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
  gotmux_output="$("${gotmux_cmd[@]}" "$@" | sed -E 's/[[:space:]]+/ /g')"
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
  tmux_output="$("${tmux_cmd[@]}" list-keys -T prefix | sed -E 's/[[:space:]]+/ /g' | awk -v key="${key}" '$1 == "bind-key" && $2 == "-T" && $3 == "prefix" && $4 == key { print }')"
  gotmux_output="$("${gotmux_cmd[@]}" list-keys -T prefix | sed -E 's/[[:space:]]+/ /g' | awk -v key="${key}" '$1 == "bind-key" && $2 == "-T" && $3 == "prefix" && $4 == key { print }')"
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

compare_note_line() {
  local name="$1"
  local key="$2"
  local tmux_output gotmux_output
  tmux_output="$("${tmux_cmd[@]}" list-keys -N | sed -E 's/[[:space:]]+/ /g' | awk -v key="${key}" '$1 == "C-b" && $2 == key { print }')"
  gotmux_output="$("${gotmux_cmd[@]}" list-keys -N | sed -E 's/[[:space:]]+/ /g' | awk -v key="${key}" '$1 == "C-b" && $2 == key { print }')"
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

wait_file_contains() {
  local file="$1"
  local needle="$2"
  local i
  for i in {1..80}; do
    if [[ -f "${file}" ]] && grep -q "${needle}" "${file}"; then
      return 0
    fi
    sleep 0.05
  done
  return 1
}

compare "new-session detached output" new-session -d -s newsout -n first /bin/sh
compare "new-session print output" new-session -d -P -F "#{session_name}:#{window_index}.#{pane_index}" -s newsp -n first /bin/sh

compare "list-sessions formats" list-sessions -F "#{session_name}:#{session_windows}:#{session_attached}"
compare "list-windows formats" list-windows -t compat -F "#{window_index}:#{window_name}:#{window_panes}:#{window_active}"
compare "list-panes formats" list-panes -t compat -F "#{pane_index}:#{pane_active}"
compare "list-clients empty" list-clients -F "#{client_name}:#{session_name}:#{client_width}:#{client_height}"
compare "list-commands new-session format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" new-session
compare "list-commands new-pane format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" newp
compare "list-commands alias query" lscm -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" display
compare "list-commands start-server format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" start
compare "list-commands lock-server format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" lock
compare "list-commands lock-session format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" locks
compare "list-commands lock-client format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" lockc
compare "list-commands refresh-client format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" refresh
compare "list-commands link-window format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" linkw
compare "list-commands switch-client format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" switchc
compare "list-commands copy-mode format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" copy-mode
compare "list-commands clock-mode format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" clock-mode
compare "list-commands choose-tree format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" choose-tree
compare "list-commands choose-buffer format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" choose-buffer
compare "list-commands choose-client format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" choose-client
compare "list-commands customize-mode format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" customize-mode
compare "list-commands find-window format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" findw
compare "list-commands confirm-before format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" confirm
compare "list-commands display-menu format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" menu
compare "list-commands display-panes format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" displayp
compare "list-commands display-popup format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" popup
compare "list-commands command-prompt format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" command-prompt
compare "list-commands suspend-client format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" suspendc
compare "list-commands server-access format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" server-access
compare "list-commands set-hook format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" set-hook
compare "list-commands show-hooks format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" show-hooks
compare "list-commands wait-for format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" wait
compare "list-commands prompt history format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" showphist
compare "list-commands list-buffers format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" lsb
compare "list-commands pipe-pane format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" pipep
compare "list-commands respawn-pane format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" respawnp
compare "list-commands respawn-window format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" respawnw
compare "list-commands unlink-window format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" unlinkw
compare "list-commands show-messages format" list-commands -F "#{command_list_name}:#{command_list_alias}:#{command_list_usage}" showmsgs
compare "start-server command" start-server
compare "lock-server command" lock-server
compare "lock alias command" lock
compare "lock-session command" lock-session -t compat
compare "lock-session alias command" locks -t compat
compare_status "lock-session missing" lock-session -t missing
compare_status "lock-client no current client" lock-client
compare_status "refresh-client no current client" refresh-client
compare_status "switch-client no current client" switch-client -t compat
compare_status "switch-client missing client" switch-client -c missing -t compat
compare "clock-mode command" clock-mode
compare "copy-mode command" copy-mode
compare "choose-buffer command" choose-buffer
compare "choose-client command" choose-client
compare "choose-tree command" choose-tree
compare "customize-mode command" customize-mode
compare "find-window command" find-window missing-pattern
compare_status "display-panes no current client" display-panes
compare_status "display-menu no current client" display-menu item i true
compare_status "display-popup no current client" display-popup
compare_status "confirm-before no current client" confirm-before true
compare_status "command-prompt no current client" command-prompt
compare_status "suspend-client no current client" suspend-client
compare "server-access list" server-access -l
compare_status "server-access missing user" server-access
compare_status "server-access owner user" server-access "$(id -un)"
compare_status "server-access unknown user" server-access gotmux-no-such-user
if id nobody >/dev/null 2>&1; then
  compare "server-access add user" server-access -a nobody
  compare "server-access read-only user" server-access -r nobody
  compare "server-access list read-only user" server-access -l
  compare "server-access delete user" server-access -d nobody
fi
compare "show-hooks empty global hook" show-hooks -g after-new-window
compare "show-options hooks empty global hook" show -H -g after-new-window
compare "set-hook global command" set-hook -g after-new-window "display-message hi"
compare "set-hook global append command" set-hook -ga after-new-window "display-message there"
compare "show-hooks global hook" show-hooks -g after-new-window
compare "show-options hooks global hook" show -H -g after-new-window
compare "show-options hooks global values" show -H -gv after-new-window
compare "set-hook local command" set-hook after-new-window "display-message local"
compare "show-hooks local hook" show-hooks after-new-window
compare "show-options hooks local hook" show -H after-new-window
compare "unset global hook command" set-hook -gu after-new-window
compare "show-hooks unset global hook" show-hooks -g after-new-window
compare_status "show-hooks invalid hook" show-hooks -g missing-hook
compare "run-shell stdout" run-shell "printf alpha"
compare "run-shell alias stderr" run -E "printf err >&2"
compare "run-shell background" run-shell -b "printf beta"
compare_status "run-shell exit status" run-shell "exit 7"
compare "if-shell true branch" if-shell "true" "display-message -p yes" "display-message -p no"
compare "if-shell false branch" if-shell "false" "display-message -p yes" "display-message -p no"
compare "if-shell format branch" if -F "1" "display-message -p fmt-yes" "display-message -p fmt-no"
compare "wait-for signal command" wait-for -S ready
compare "wait-for alias wait" wait ready
compare "wait-for lock command" wait-for -L lock
compare "wait-for unlock command" wait-for -U lock
compare_status "wait-for unlock missing" wait-for -U missing
compare "show prompt history" show-prompt-history
compare "show prompt history type" showphist -T command
compare "clear prompt history" clearphist -T command
compare_status "show prompt history invalid" show-prompt-history -T nope
compare "show messages jobs" show-messages -J
compare "show messages terminals" showmsgs -T
compare "display-message formats" display-message -p -t compat -F "#{session_name}:#{window_index}:#{window_name}:#{pane_index}"
compare "display-message alias" display -p -t compat -F "#{session_name}:#{window_index}:#{window_name}:#{pane_index}"
compare "display-message target pane" display-message -p -t compat:.0 -F "#{pane_index}:#{pane_active}"
compare "display-message message" display-message -p -t compat "hello #{session_name}"

"${tmux_cmd[@]}" new-session -d -s lsta -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t lsta -n second /bin/sh
"${tmux_cmd[@]}" split-window -t lsta:0 -h /bin/sh
"${tmux_cmd[@]}" new-session -d -s lstb -x 80 -y 24 -n only /bin/sh
"${gotmux_cmd[@]}" new-session -d -s lsta -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t lsta -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t lsta:0 -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-session -d -s lstb -x 80 -y 24 -n only /bin/sh >/dev/null
compare "list-windows all sessions" list-windows -a -F "#{session_name}:#{window_index}:#{window_name}"
compare "list-panes session scope" list-panes -s -t lsta -F "#{session_name}:#{window_index}:#{pane_index}"
compare "list-panes all sessions" list-panes -a -F "#{session_name}:#{window_index}:#{pane_index}"
compare "list-windows active filter" list-windows -t lsta -f "#{window_active}" -F "#{window_index}:#{window_name}:#{window_active}"
compare "list-panes active filter" list-panes -t lsta -f "#{pane_active}" -F "#{pane_index}:#{pane_active}"
compare "list-sessions attached filter" list-sessions -f "#{session_attached}" -F "#{session_name}:#{session_attached}"

"${tmux_cmd[@]}" new-session -d -s sendt -x 20 -y 4 /bin/sh
"${tmux_cmd[@]}" split-window -t sendt -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s sendt -x 20 -y 4 /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t sendt -h /bin/sh >/dev/null
"${tmux_cmd[@]}" send-keys -t sendt:.0 "printf '\\033[H\\033[2Jleft\\n'" Enter
"${tmux_cmd[@]}" send-keys -t sendt:.1 "printf '\\033[H\\033[2Jright\\n'" Enter
"${gotmux_cmd[@]}" send-keys -t sendt:.0 "printf '\\033[H\\033[2Jleft\\n'" Enter >/dev/null
"${gotmux_cmd[@]}" send-keys -t sendt:.1 "printf '\\033[H\\033[2Jright\\n'" Enter >/dev/null
sleep 0.4
compare "send-keys target left pane" capture-pane -p -t sendt:.0 -S 0 -E 0
compare "send-keys target right pane" capture-pane -p -t sendt:.1 -S 0 -E 0

"${tmux_cmd[@]}" new-session -d -s cap -x 20 -y 5 /bin/sh
"${gotmux_cmd[@]}" new-session -d -s cap -x 20 -y 5 /bin/sh >/dev/null
"${tmux_cmd[@]}" send-keys -t cap "printf '\\033[H\\033[2Jone  \\ntwo\\nthree\\n'" Enter
"${gotmux_cmd[@]}" send-keys -t cap "printf '\\033[H\\033[2Jone  \\ntwo\\nthree\\n'" Enter >/dev/null
sleep 0.4
compare "capture-pane visible text" capture-pane -p -t cap -S 0 -E 2
compare "capture-pane full empty cells" capture-pane -p -N -t cap -S 0 -E 0
compare "capture-pane used trailing cells" capture-pane -p -N -T -t cap -S 0 -E 0
"${tmux_cmd[@]}" capture-pane -b capbuf -t cap -S 0 -E 2
"${gotmux_cmd[@]}" capture-pane -b capbuf -t cap -S 0 -E 2 >/dev/null
compare "capture-pane buffer" list-buffers -F "#{buffer_name}:#{buffer_size}:#{buffer_sample}"

"${tmux_cmd[@]}" new-session -d -s capj -x 5 -y 5 /bin/sh
"${gotmux_cmd[@]}" new-session -d -s capj -x 5 -y 5 /bin/sh >/dev/null
"${tmux_cmd[@]}" send-keys -t capj "printf '\\033[H\\033[2Jabcdefgh\\nxy\\n'" Enter
"${gotmux_cmd[@]}" send-keys -t capj "printf '\\033[H\\033[2Jabcdefgh\\nxy\\n'" Enter >/dev/null
sleep 0.4
compare "capture-pane wrapped visible text" capture-pane -p -t capj -S 0 -E 2
compare "capture-pane joins wrapped lines" capture-pane -p -J -t capj -S 0 -E 2
compare "capture-pane line flags" capture-pane -p -F -t capj -S 0 -E 2
compare "capture-pane line numbers" capture-pane -p -L -t capj -S 0 -E 2
compare "capture-pane joined flags and numbers" capture-pane -p -L -F -J -t capj -S 0 -E 2

"${tmux_cmd[@]}" new-session -d -s capc -x 20 -y 4 /bin/sh
"${gotmux_cmd[@]}" new-session -d -s capc -x 20 -y 4 /bin/sh >/dev/null
"${tmux_cmd[@]}" send-keys -t capc "printf '\\033[H\\033[2Ja\\\\\\\\b\\n'" Enter
"${gotmux_cmd[@]}" send-keys -t capc "printf '\\033[H\\033[2Ja\\\\\\\\b\\n'" Enter >/dev/null
sleep 0.4
compare "capture-pane escaped backslash" capture-pane -p -C -t capc -S 0 -E 0

compare "clear history command" clear-history -t cap
compare "clear history alias" clearhist -t cap

"${tmux_cmd[@]}" set-buffer -b named "hello world"
"${gotmux_cmd[@]}" set-buffer -b named "hello world" >/dev/null
compare "show named buffer" show-buffer -b named
compare "list buffers format" list-buffers -F "#{buffer_name}:#{buffer_size}:#{buffer_sample}"
compare "rename named buffer" set-buffer -b named -n renamed
compare "show renamed buffer" show-buffer -b renamed
"${tmux_cmd[@]}" delete-buffer -b renamed
"${gotmux_cmd[@]}" delete-buffer -b renamed >/dev/null
compare "delete renamed buffer" list-buffers -F "#{buffer_name}:#{buffer_size}:#{buffer_sample}"

"${tmux_cmd[@]}" set-buffer -b zed "zed"
"${tmux_cmd[@]}" set-buffer -b alpha "alpha"
"${gotmux_cmd[@]}" set-buffer -b zed "zed" >/dev/null
"${gotmux_cmd[@]}" set-buffer -b alpha "alpha" >/dev/null
compare "list buffers order name" list-buffers -O name -F "#{buffer_name}"
compare "list buffers truthy filter" list-buffers -f "#{buffer_name}" -F "#{buffer_name}"
compare "list buffers false filter" list-buffers -f "0" -F "#{buffer_name}"
compare_status "list buffers invalid order" list-buffers -O nope

buffer_file="$(mktemp)"
tmux_saved_file="$(mktemp)"
gotmux_saved_file="$(mktemp)"
printf 'alpha\nbeta\n' > "${buffer_file}"
"${tmux_cmd[@]}" load-buffer -b loaded "${buffer_file}"
"${gotmux_cmd[@]}" load-buffer -b loaded "${buffer_file}" >/dev/null
compare "load buffer file" list-buffers -F "#{buffer_name}:#{buffer_size}:#{buffer_sample}"
"${tmux_cmd[@]}" save-buffer -b loaded "${tmux_saved_file}"
"${gotmux_cmd[@]}" save-buffer -b loaded "${gotmux_saved_file}" >/dev/null
if ! cmp -s "${tmux_saved_file}" "${gotmux_saved_file}"; then
  echo "compat probe failed: save buffer file" >&2
  echo "--- tmux" >&2
  od -An -t x1 "${tmux_saved_file}" >&2
  echo "--- gotmux" >&2
  od -An -t x1 "${gotmux_saved_file}" >&2
  exit 1
fi
printf 'ok save buffer file\n'
rm -f "${buffer_file}" "${tmux_saved_file}" "${gotmux_saved_file}"

tmux_pipe_file="$(mktemp)"
gotmux_pipe_file="$(mktemp)"
pane_script="$(mktemp)"
printf '#!/bin/sh\nstty -echo\nexec cat\n' > "${pane_script}"
chmod +x "${pane_script}"
"${tmux_cmd[@]}" new-session -d -s pipep -x 80 -y 24 -n first "${pane_script}"
"${gotmux_cmd[@]}" new-session -d -s pipep -x 80 -y 24 -n first "${pane_script}" >/dev/null
"${tmux_cmd[@]}" pipep -o -t pipep:0.0 "cat > '${tmux_pipe_file}'"
"${gotmux_cmd[@]}" pipep -o -t pipep:0.0 "cat > '${gotmux_pipe_file}'" >/dev/null
"${tmux_cmd[@]}" send-keys -t pipep:0.0 pipe-alpha Enter
"${gotmux_cmd[@]}" send-keys -t pipep:0.0 pipe-alpha Enter >/dev/null
if ! wait_file_contains "${tmux_pipe_file}" pipe-alpha || ! wait_file_contains "${gotmux_pipe_file}" pipe-alpha; then
  echo "compat probe failed: pipe-pane output file" >&2
  echo "--- tmux" >&2
  cat "${tmux_pipe_file}" >&2 || true
  echo "--- gotmux" >&2
  cat "${gotmux_pipe_file}" >&2 || true
  exit 1
fi
printf 'ok pipe-pane output file\n'
"${tmux_cmd[@]}" pipep -o -t pipep:0.0 "cat > '${tmux_pipe_file}'"
"${gotmux_cmd[@]}" pipep -o -t pipep:0.0 "cat > '${gotmux_pipe_file}'" >/dev/null
"${tmux_cmd[@]}" send-keys -t pipep:0.0 pipe-beta Enter
"${gotmux_cmd[@]}" send-keys -t pipep:0.0 pipe-beta Enter >/dev/null
sleep 0.3
if grep -q pipe-beta "${tmux_pipe_file}" || grep -q pipe-beta "${gotmux_pipe_file}"; then
  echo "compat probe failed: pipe-pane toggle close" >&2
  echo "--- tmux" >&2
  cat "${tmux_pipe_file}" >&2 || true
  echo "--- gotmux" >&2
  cat "${gotmux_pipe_file}" >&2 || true
  exit 1
fi
printf 'ok pipe-pane toggle close\n'
rm -f "${tmux_pipe_file}" "${gotmux_pipe_file}" "${pane_script}"

"${tmux_cmd[@]}" new-session -d -s layh -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layh -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s layh -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layh -h /bin/sh >/dev/null
compare "horizontal pane geometry" list-panes -t layh -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s layv -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layv -v /bin/sh
"${gotmux_cmd[@]}" new-session -d -s layv -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layv -v /bin/sh >/dev/null
compare "vertical pane geometry" list-panes -t layv -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s lay3 -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t lay3 -h /bin/sh
"${tmux_cmd[@]}" split-window -t lay3 -v /bin/sh
"${gotmux_cmd[@]}" new-session -d -s lay3 -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t lay3 -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t lay3 -v /bin/sh >/dev/null
compare "nested pane geometry" list-panes -t lay3 -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s layresize -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layresize -h /bin/sh
"${tmux_cmd[@]}" resize-pane -t layresize -L 5
"${gotmux_cmd[@]}" new-session -d -s layresize -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layresize -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" resize-pane -t layresize -L 5 >/dev/null
compare "resize pane geometry" list-panes -t layresize -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s layresizetarget -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layresizetarget -h /bin/sh
"${tmux_cmd[@]}" resize-pane -t layresizetarget:.0 -R 5
"${gotmux_cmd[@]}" new-session -d -s layresizetarget -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layresizetarget -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" resize-pane -t layresizetarget:.0 -R 5 >/dev/null
compare "targeted resize pane geometry" list-panes -t layresizetarget -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s resizew -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t resizew -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s resizew -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t resizew -h /bin/sh >/dev/null
compare "resize-window exact command" resize-window -x 100 -y 30 -t resizew
compare "resize-window exact size" list-windows -t resizew -F "#{window_width}:#{window_height}"
compare "resize-window exact panes" list-panes -t resizew -F "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_height}"
compare "resize-window left command" resizew -L -t resizew 10
compare "resize-window left size" list-windows -t resizew -F "#{window_width}:#{window_height}"
compare "resize-window left panes" list-panes -t resizew -F "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_height}"

"${tmux_cmd[@]}" new-session -d -s layselect -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layselect -h /bin/sh
"${tmux_cmd[@]}" split-window -t layselect -h /bin/sh
"${tmux_cmd[@]}" select-layout -t layselect even-horizontal
"${gotmux_cmd[@]}" new-session -d -s layselect -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layselect -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layselect -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" select-layout -t layselect even-horizontal >/dev/null
compare "select-layout even-horizontal geometry" list-panes -t layselect -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

for layout in main-horizontal main-horizontal-mirrored main-vertical main-vertical-mirrored tiled; do
  session="lay_${layout//-/_}"
  "${tmux_cmd[@]}" new-session -d -s "${session}" -x 80 -y 24 -n first /bin/sh
  "${gotmux_cmd[@]}" new-session -d -s "${session}" -x 80 -y 24 -n first /bin/sh >/dev/null
  for _ in 1 2 3 4; do
    "${tmux_cmd[@]}" split-window -t "${session}" -h /bin/sh
    "${gotmux_cmd[@]}" split-window -t "${session}" -h /bin/sh >/dev/null
  done
  "${tmux_cmd[@]}" select-layout -t "${session}" "${layout}"
  "${gotmux_cmd[@]}" select-layout -t "${session}" "${layout}" >/dev/null
  compare "select-layout ${layout} geometry" list-panes -t "${session}" -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
done

"${tmux_cmd[@]}" new-session -d -s laycycle -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t laycycle -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s laycycle -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t laycycle -h /bin/sh >/dev/null
compare "select-layout no-arg command" select-layout -t laycycle
compare "select-layout no-arg geometry" list-panes -t laycycle -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "previous-layout command" previous-layout -t laycycle
compare "previous-layout geometry" list-panes -t laycycle -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "next-layout command" next-layout -t laycycle
compare "next-layout geometry" list-panes -t laycycle -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "select-layout previous flag command" select-layout -t laycycle -p
compare "select-layout previous flag geometry" list-panes -t laycycle -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "select-layout next flag command" select-layout -t laycycle -n
compare "select-layout next flag geometry" list-panes -t laycycle -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s layselecttarget -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t layselecttarget:0 -h /bin/sh
"${tmux_cmd[@]}" new-window -t layselecttarget -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s layselecttarget -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t layselecttarget:0 -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t layselecttarget -n second /bin/sh >/dev/null
compare "split-window target windows" list-windows -t layselecttarget -F "#{window_index}:#{window_active}:#{window_panes}"
compare "select-layout target command" select-layout -t layselecttarget:0 even-vertical
compare "select-layout target keeps active window" list-windows -t layselecttarget -F "#{window_index}:#{window_active}"
"${tmux_cmd[@]}" select-window -t layselecttarget:0
"${gotmux_cmd[@]}" select-window -t layselecttarget:0 >/dev/null
compare "select-layout target geometry" list-panes -t layselecttarget -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s selp -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t selp -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selp -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t selp -h /bin/sh >/dev/null
compare "select-pane target command" select-pane -t selp:.0
compare "select-pane target panes" list-panes -t selp -F "#{pane_index}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s selpdir -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t selpdir -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selpdir -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t selpdir -h /bin/sh >/dev/null
compare "select-pane left command" select-pane -L -t selpdir
compare "select-pane left panes" list-panes -t selpdir -F "#{pane_index}:#{pane_active}"
compare "select-pane last flag command" select-pane -l -t selpdir
compare "select-pane last flag panes" list-panes -t selpdir -F "#{pane_index}:#{pane_active}"
compare "last-pane command" last-pane -t selpdir
compare "last-pane panes" list-panes -t selpdir -F "#{pane_index}:#{pane_active}"
compare "last-pane alias command" lastp -t selpdir
compare "last-pane alias panes" list-panes -t selpdir -F "#{pane_index}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s selpup -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t selpup /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selpup -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t selpup /bin/sh >/dev/null
compare "select-pane up command" select-pane -U -t selpup
compare "select-pane up panes" list-panes -t selpup -F "#{pane_index}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s swapp -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t swapp -h /bin/sh
"${tmux_cmd[@]}" split-window -t swapp -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s swapp -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t swapp -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t swapp -h /bin/sh >/dev/null
compare "swap-pane up command" swap-pane -U -t swapp
compare "swap-pane up panes" list-panes -t swapp -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "swap-pane detached command" swap-pane -d -s swapp:.0 -t swapp:.1
compare "swap-pane detached panes" list-panes -t swapp -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s rotatew -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t rotatew -h /bin/sh
"${tmux_cmd[@]}" split-window -t rotatew -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s rotatew -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t rotatew -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t rotatew -h /bin/sh >/dev/null
compare "rotate-window command" rotate-window -t rotatew
compare "rotate-window panes" list-panes -t rotatew -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "rotate-window reverse command" rotate-window -D -t rotatew
compare "rotate-window reverse panes" list-panes -t rotatew -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s joinp -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t joinp -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s joinp -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t joinp -n second /bin/sh >/dev/null
compare "join-pane command" join-pane -s joinp:1.0 -t joinp:0.0 -h
compare "join-pane windows" list-windows -t joinp -F "#{window_index}:#{window_name}:#{window_active}:#{window_panes}"
compare "join-pane panes" list-panes -t joinp:0 -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s killp -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t killp -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s killp -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t killp -h /bin/sh >/dev/null
compare "kill-pane target command" kill-pane -t killp:.0
compare "kill-pane target panes" list-panes -t killp -F "#{pane_index}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s killpa -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" split-window -t killpa -h /bin/sh
"${tmux_cmd[@]}" split-window -t killpa -h /bin/sh
"${gotmux_cmd[@]}" new-session -d -s killpa -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t killpa -h /bin/sh >/dev/null
"${gotmux_cmd[@]}" split-window -t killpa -h /bin/sh >/dev/null
compare "kill-pane all command" kill-pane -a -t killpa:.1
compare "kill-pane all panes" list-panes -t killpa -F "#{pane_index}:#{pane_id}:#{pane_left}:#{pane_width}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s killw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t killw -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s killw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t killw -n second /bin/sh >/dev/null
compare "kill-window target command" kill-window -t killw:1
compare "kill-window target windows" list-windows -t killw -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s killwa -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t killwa -n second /bin/sh
"${tmux_cmd[@]}" new-window -t killwa -n third /bin/sh
"${gotmux_cmd[@]}" new-session -d -s killwa -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t killwa -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t killwa -n third /bin/sh >/dev/null
compare "kill-window all command" kill-window -a -t killwa:1
compare "kill-window all windows" list-windows -t killwa -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s unlinkw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t unlinkw -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s unlinkw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t unlinkw -n second /bin/sh >/dev/null
compare_status "unlink-window single link rejection" unlink-window -t unlinkw:1
compare "unlink-window rejected windows" list-windows -t unlinkw -F "#{window_index}:#{window_name}:#{window_active}"
compare "unlink-window kill command" unlinkw -k -t unlinkw:1
compare "unlink-window kill windows" list-windows -t unlinkw -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s linka -x 80 -y 24 -n one /bin/sh
"${tmux_cmd[@]}" new-session -d -s linkb -x 80 -y 24 -n two /bin/sh
"${gotmux_cmd[@]}" new-session -d -s linka -x 80 -y 24 -n one /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-session -d -s linkb -x 80 -y 24 -n two /bin/sh >/dev/null
compare "link-window command" link-window -s linka:0 -t linkb:2
compare "link-window source windows" list-windows -t linka -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "link-window target windows" list-windows -t linkb -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "unlink linked window command" unlink-window -t linkb:2
compare "unlink linked source windows" list-windows -t linka -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "unlink linked target windows" list-windows -t linkb -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "link-window detached command" linkw -d -s linka:0 -t linkb:2
compare "link-window detached windows" list-windows -t linkb -F "#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "link-window replace command" link-window -k -s linka:0 -t linkb:0
compare "link-window replace source windows" list-windows -t linka -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "link-window replace target windows" list-windows -t linkb -F "#{session_name}:#{window_index}:#{window_name}:#{window_id}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s swapw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t swapw -n second /bin/sh
"${tmux_cmd[@]}" new-window -t swapw -n third /bin/sh
"${gotmux_cmd[@]}" new-session -d -s swapw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t swapw -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t swapw -n third /bin/sh >/dev/null
compare "swap-window command" swap-window -s swapw:0 -t swapw:2
compare "swap-window windows" list-windows -t swapw -F "#{window_index}:#{window_name}:#{window_id}:#{window_active}"
compare "swap-window detached command" swapw -d -s swapw:0 -t swapw:2
compare "swap-window detached windows" list-windows -t swapw -F "#{window_index}:#{window_name}:#{window_id}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s movew -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t movew -n second /bin/sh
"${tmux_cmd[@]}" new-window -t movew -n third /bin/sh
"${gotmux_cmd[@]}" new-session -d -s movew -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t movew -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t movew -n third /bin/sh >/dev/null
compare "move-window command" move-window -s movew:0 -t movew:5
compare "move-window windows" list-windows -t movew -F "#{window_index}:#{window_name}:#{window_active}"
compare "move-window renumber command" movew -r -t movew
compare "move-window renumber windows" list-windows -t movew -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s renamew -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t renamew -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s renamew -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t renamew -n second /bin/sh >/dev/null
compare "rename-window target command" rename-window -t renamew:0 primary
compare "rename-window target windows" list-windows -t renamew -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s aliascmd -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s aliascmd -x 80 -y 24 -n first /bin/sh >/dev/null
compare "rename-session alias command" rename -t aliascmd aliasrenamed
compare "rename-window alias command" renamew -t aliasrenamed:0 primary
compare "rename aliases windows" list-windows -t aliasrenamed -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s selectw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t selectw -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selectw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t selectw -n second /bin/sh >/dev/null
compare "select-window target command" select-window -t selectw:0
compare "select-window target windows" list-windows -t selectw -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s selectwflags -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t selectwflags -n second /bin/sh
"${tmux_cmd[@]}" new-window -t selectwflags -n third /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selectwflags -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t selectwflags -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t selectwflags -n third /bin/sh >/dev/null
compare "select-window previous flag command" select-window -p -t selectwflags
compare "select-window previous flag windows" list-windows -t selectwflags -F "#{window_index}:#{window_name}:#{window_active}"
compare "select-window next flag command" select-window -n -t selectwflags
compare "select-window next flag windows" list-windows -t selectwflags -F "#{window_index}:#{window_name}:#{window_active}"
compare "previous-window target command" previous-window -t selectwflags
compare "previous-window target windows" list-windows -t selectwflags -F "#{window_index}:#{window_name}:#{window_active}"
compare "next-window target command" next-window -t selectwflags
compare "next-window target windows" list-windows -t selectwflags -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s newwd -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s newwd -x 80 -y 24 -n first /bin/sh >/dev/null
compare "new-window detached command" new-window -d -t newwd -n second /bin/sh
compare "new-window detached windows" list-windows -t newwd -F "#{window_index}:#{window_name}:#{window_active}"
compare "new-window print command" new-window -P -F "#{window_index}:#{window_name}" -t newwd -n third /bin/sh
compare "new-window print windows" list-windows -t newwd -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s newp -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s newp -x 80 -y 24 -n first /bin/sh >/dev/null
compare "new-pane print command" new-pane -P -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}" -t newp /bin/sh
compare "new-pane panes" list-panes -t newp -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"
compare "new-pane detached command" newp -d -x 20 -y 5 -X 3 -Y 4 -t newp /bin/sh
compare "new-pane detached panes" list-panes -t newp -F "#{pane_index}:#{pane_left}:#{pane_top}:#{pane_width}:#{pane_height}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s respawn -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s respawn -x 80 -y 24 -n first /bin/sh >/dev/null
compare "respawn-pane command" respawn-pane -k -t respawn:0.0 /bin/sh
"${tmux_cmd[@]}" split-window -t respawn -h /bin/sh
"${gotmux_cmd[@]}" split-window -t respawn -h /bin/sh >/dev/null
compare "respawn-window command" respawn-window -k -t respawn:0 /bin/sh
compare "respawn-window panes" list-panes -t respawn -F "#{pane_index}:#{pane_active}"

"${tmux_cmd[@]}" new-session -d -s splitd -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s splitd -x 80 -y 24 -n first /bin/sh >/dev/null
compare "split-window detached command" split-window -d -h -t splitd /bin/sh
compare "split-window detached panes" list-panes -t splitd -F "#{pane_index}:#{pane_left}:#{pane_width}:#{pane_active}"
compare "split-window print command" split-window -P -F "#{pane_index}:#{pane_active}" -t splitd /bin/sh

"${tmux_cmd[@]}" new-session -d -s lastw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t lastw -n second /bin/sh
"${tmux_cmd[@]}" new-window -t lastw -n third /bin/sh
"${gotmux_cmd[@]}" new-session -d -s lastw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t lastw -n second /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t lastw -n third /bin/sh >/dev/null
compare "last-window command" last-window -t lastw
compare "last-window windows" list-windows -t lastw -F "#{window_index}:#{window_name}:#{window_active}"
compare "select-window last flag command" select-window -l -t lastw
compare "select-window last flag windows" list-windows -t lastw -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" set -g status off
"${gotmux_cmd[@]}" set -g status off >/dev/null
compare "show global option" show -g status
compare "show global option value" show -gqv status
compare_status "set-once global option already set" set -go status on
compare "show server option value" show -sqv escape-time
"${tmux_cmd[@]}" set -s escape-time 123
"${gotmux_cmd[@]}" set -s escape-time 123 >/dev/null
compare "set server option value" show -sqv escape-time
compare "server option visible globally" show -gqv escape-time
"${tmux_cmd[@]}" set -su escape-time
"${gotmux_cmd[@]}" set -su escape-time >/dev/null
compare "unset server option value" show -sqv escape-time
"${tmux_cmd[@]}" set -s prefix C-a
"${gotmux_cmd[@]}" set -s prefix C-a >/dev/null
compare "server-specific option value" show -sqv prefix
compare_status "set-once server option already set" set -so prefix C-c
compare "server set leaves global option" show -gqv prefix
"${tmux_cmd[@]}" new-session -d -s optonce -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s optonce -x 80 -y 24 -n first /bin/sh >/dev/null
compare "set-once local option" set -o -t optonce status off
compare_status "set-once local option already set" set -o -t optonce status on
compare "set-once local option value" show -t optonce -v status
"${tmux_cmd[@]}" set -gu status
"${gotmux_cmd[@]}" set -gu status >/dev/null
compare "unset global option value" show -gqv status
"${tmux_cmd[@]}" set -g default-command foo
"${gotmux_cmd[@]}" set -g default-command foo >/dev/null
"${tmux_cmd[@]}" set -ga default-command bar
"${gotmux_cmd[@]}" set -ga default-command bar >/dev/null
compare "append global option value" show -gqv default-command
"${tmux_cmd[@]}" set -gu default-command
"${gotmux_cmd[@]}" set -gu default-command >/dev/null
compare "unset string global option value" show -gqv default-command

"${tmux_cmd[@]}" setw -g mode-keys vi
"${gotmux_cmd[@]}" setw -g mode-keys vi >/dev/null
compare "show global window option" show -gw mode-keys
compare "show-window-options command" show-window-options -gv mode-keys
compare "show-window-options alias" showw -gv mode-keys

"${tmux_cmd[@]}" new-session -d -s opttarget -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t opttarget -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s opttarget -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t opttarget -n second /bin/sh >/dev/null
"${tmux_cmd[@]}" set -t opttarget status off
"${gotmux_cmd[@]}" set -t opttarget status off >/dev/null
compare "target session option" show -t opttarget -v status
"${tmux_cmd[@]}" setw -t opttarget:1 mode-keys vi
"${gotmux_cmd[@]}" setw -t opttarget:1 mode-keys vi >/dev/null
compare "target window option" showw -t opttarget:1 -v mode-keys
compare "untouched target window option" showw -t opttarget:0 -v mode-keys

"${tmux_cmd[@]}" bind-key C-a send-prefix
"${gotmux_cmd[@]}" bind-key C-a send-prefix >/dev/null
"${tmux_cmd[@]}" bind-key -N "reload config" C-r source-file ~/.tmux.conf
"${gotmux_cmd[@]}" bind-key -N "reload config" C-r source-file ~/.tmux.conf >/dev/null
compare_key_line "default refresh binding" r
compare_key_line "list custom key" C-a
compare_note_line "list custom key note" C-r

"${tmux_cmd[@]}" setenv FOO bar
"${gotmux_cmd[@]}" setenv FOO bar >/dev/null
compare "show environment" showenv FOO
compare "show environment shell" showenv -s FOO
"${tmux_cmd[@]}" setenv -g GLOBAL yes
"${gotmux_cmd[@]}" setenv -g GLOBAL yes >/dev/null
compare "show global environment" showenv -g GLOBAL
"${tmux_cmd[@]}" setenv -u FOO
"${gotmux_cmd[@]}" setenv -u FOO >/dev/null
tmux_showenv_missing="$("${tmux_cmd[@]}" showenv FOO 2>&1 || true)"
gotmux_showenv_missing="$("${gotmux_cmd[@]}" showenv FOO 2>&1 || true)"
if [[ "${tmux_showenv_missing}" != "${gotmux_showenv_missing}" ]]; then
  echo "compat probe failed: unset environment" >&2
  echo "--- tmux" >&2
  printf '%s\n' "${tmux_showenv_missing}" >&2
  echo "--- gotmux" >&2
  printf '%s\n' "${gotmux_showenv_missing}" >&2
  exit 1
fi
printf 'ok unset environment\n'

source_file="$(mktemp)"
printf 'set -g status on\nnew-window -t compat -n sourced /bin/sh\n' > "${source_file}"
"${tmux_cmd[@]}" source-file "${source_file}"
"${gotmux_cmd[@]}" source-file "${source_file}" >/dev/null
rm -f "${source_file}"
compare "source-file option" show -gqv status
compare "source-file window" list-windows -t compat -F "#{window_index}:#{window_name}:#{window_panes}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s seq -n first /bin/sh \; new-window -t seq -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s seq -n first /bin/sh \; new-window -t seq -n second /bin/sh >/dev/null
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

default_home="$(mktemp -d)"
printf 'set -g status off\n' > "${default_home}/.tmux.conf"
HOME="${default_home}" "${TMUX_BIN}" -L "${tmux_default_config_sock}" new-session -d -s defaultconf -n first /bin/sh
HOME="${default_home}" "${BIN}" -S "${gotmux_default_config_sock}" new-session -d -s defaultconf -n first /bin/sh >/dev/null
tmux_default_status="$(HOME="${default_home}" "${TMUX_BIN}" -L "${tmux_default_config_sock}" show -gqv status)"
gotmux_default_status="$(HOME="${default_home}" "${BIN}" -S "${gotmux_default_config_sock}" show -gqv status)"
rm -rf "${default_home}"
if [[ "${tmux_default_status}" != "${gotmux_default_status}" ]]; then
  echo "compat probe failed: default config discovery" >&2
  echo "--- tmux" >&2
  printf '%s\n' "${tmux_default_status}" >&2
  echo "--- gotmux" >&2
  printf '%s\n' "${gotmux_default_status}" >&2
  exit 1
fi
printf 'ok default config discovery\n'
