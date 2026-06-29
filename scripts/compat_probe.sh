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
  tmux_output="$("${tmux_cmd[@]}" list-keys -T prefix | grep -E "[[:space:]]${key}[[:space:]]" | sed -E 's/[[:space:]]+/ /g')"
  gotmux_output="$("${gotmux_cmd[@]}" list-keys -T prefix | grep -E "[[:space:]]${key}[[:space:]]" | sed -E 's/[[:space:]]+/ /g')"
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
compare "display-message target pane" display-message -p -t compat:.0 -F "#{pane_index}:#{pane_active}"

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
"${tmux_cmd[@]}" delete-buffer -b named
"${gotmux_cmd[@]}" delete-buffer -b named >/dev/null
compare "delete named buffer" list-buffers -F "#{buffer_name}:#{buffer_size}:#{buffer_sample}"

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

"${tmux_cmd[@]}" new-session -d -s killw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t killw -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s killw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t killw -n second /bin/sh >/dev/null
compare "kill-window target command" kill-window -t killw:1
compare "kill-window target windows" list-windows -t killw -F "#{window_index}:#{window_name}:#{window_active}"

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

"${tmux_cmd[@]}" new-session -d -s selectw -x 80 -y 24 -n first /bin/sh
"${tmux_cmd[@]}" new-window -t selectw -n second /bin/sh
"${gotmux_cmd[@]}" new-session -d -s selectw -x 80 -y 24 -n first /bin/sh >/dev/null
"${gotmux_cmd[@]}" new-window -t selectw -n second /bin/sh >/dev/null
compare "select-window target command" select-window -t selectw:0
compare "select-window target windows" list-windows -t selectw -F "#{window_index}:#{window_name}:#{window_active}"

"${tmux_cmd[@]}" new-session -d -s newwd -x 80 -y 24 -n first /bin/sh
"${gotmux_cmd[@]}" new-session -d -s newwd -x 80 -y 24 -n first /bin/sh >/dev/null
compare "new-window detached command" new-window -d -t newwd -n second /bin/sh
compare "new-window detached windows" list-windows -t newwd -F "#{window_index}:#{window_name}:#{window_active}"
compare "new-window print command" new-window -P -F "#{window_index}:#{window_name}" -t newwd -n third /bin/sh
compare "new-window print windows" list-windows -t newwd -F "#{window_index}:#{window_name}:#{window_active}"

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

"${tmux_cmd[@]}" setw -g mode-keys vi
"${gotmux_cmd[@]}" setw -g mode-keys vi >/dev/null
compare "show global window option" show -gw mode-keys

"${tmux_cmd[@]}" bind-key C-a send-prefix
"${gotmux_cmd[@]}" bind-key C-a send-prefix >/dev/null
compare_key_line "list custom key" C-a

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
