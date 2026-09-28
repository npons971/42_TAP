#!/usr/bin/env bash
set -euo pipefail

repo="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo"
make_command="$1"
server_addr="$2"
world_file="$3"
cli_addr="$4"
gui_host="$5"
gui_port="$6"

if [[ -z "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]]; then
    echo 'make run-all nécessite une session graphique.' >&2
    exit 1
fi

terminal=''
for candidate in gnome-terminal konsole xfce4-terminal mate-terminal kitty alacritty xterm; do
    if command -v "$candidate" >/dev/null 2>&1; then
        terminal="$candidate"
        break
    fi
done
if [[ -z "$terminal" ]]; then
    echo 'Aucun terminal compatible trouvé. Lancez make run-server, make run-cli et make gui dans trois terminaux.' >&2
    exit 1
fi

probe_host="${cli_addr%:*}"
probe_host="${probe_host#[}"
probe_host="${probe_host%]}"
probe_port="${cli_addr##*:}"
if [[ "$probe_host" != 127.0.0.1 && "$probe_host" != localhost && "$probe_host" != ::1 ]]; then
    echo 'make run-all nécessite un CLI_ADDR local pour attendre le serveur.' >&2
    exit 1
fi

server_ready() (
    exec 3<>"/dev/tcp/$probe_host/$probe_port" || exit 1
    IFS= read -r -t 1 greeting <&3 || exit 1
    [[ "$greeting" == 'OK hello proto=1' ]]
) 2>/dev/null

if server_ready; then
    echo "Un serveur TAP tourne déjà sur $cli_addr. Arrêtez-le avant make run-all." >&2
    exit 1
fi

server_pid=''
cleanup() {
    if [[ -n "$server_pid" ]]; then
        # The isolated process group also contains the server started by go run.
        kill -TERM -- "-$server_pid" 2>/dev/null || true
        wait "$server_pid" 2>/dev/null || true
    fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Job control gives the server and its build/run children their own process group.
set -m
"$make_command" run-server "SERVER_ADDR=$server_addr" "WORLD_FILE=$world_file" &
server_pid=$!
set +m

echo "Attente du serveur TAP sur $cli_addr…"
while true; do
    if ! kill -0 "$server_pid" 2>/dev/null; then
        wait "$server_pid"
        exit 1
    fi
    if server_ready; then
        break
    fi
    sleep 0.1
done

open_terminal() {
    local title="$1"
    shift
    local -a command=(bash -c 'cd -- "$1" || exit; shift; "$@"; result=$?; printf "\nCommande terminée (code %s). Entrée pour fermer.\n" "$result"; read -r; exit "$result"' tap-terminal "$repo" "$make_command" "$@")
    case "$terminal" in
        gnome-terminal|mate-terminal) "$terminal" --title="$title" -- "${command[@]}" ;;
        konsole) "$terminal" --separate --title "$title" -e "${command[@]}" ;;
        xfce4-terminal) "$terminal" --disable-server --title="$title" -x "${command[@]}" ;;
        kitty) "$terminal" --title "$title" "${command[@]}" ;;
        alacritty) "$terminal" --title "$title" -e "${command[@]}" ;;
        xterm) "$terminal" -T "$title" -e "${command[@]}" ;;
    esac
}

open_terminal 'TAP — CLI' run-cli "CLI_ADDR=$cli_addr" &
open_terminal 'TAP — GUI' gui "GUI_HOST=$gui_host" "GUI_PORT=$gui_port" GUI_SERVER=off "WORLD_FILE=$world_file" &
echo 'Clients ouverts dans deux terminaux. Ctrl+C ici pour arrêter le serveur.'
wait "$server_pid"
