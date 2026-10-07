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

if [[ ! -x "$repo/node" ]]; then
    echo 'Node local absent : lancez make install.' >&2
    exit 1
fi

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
    echo 'Aucun terminal compatible trouvé. Lancez make run-server, make run-client et make run-client-gui dans trois terminaux.' >&2
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

probe_server() {
    "$repo/node" --input-type=module - "$probe_host" "$probe_port" <<'JS'
import net from 'node:net';
const socket = net.createConnection({ host: process.argv[2], port: Number(process.argv[3]) });
let data = '', settled = false;
const finish = status => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
    socket.destroy();
    console.log(status);
};
const timer = setTimeout(() => finish('occupied'), 1000);
socket.on('error', error => finish(error.code === 'ECONNREFUSED' ? 'absent' : 'occupied'));
socket.on('end', () => finish('occupied'));
socket.on('data', chunk => {
    data += chunk.toString('utf8');
    if (data.includes('\n')) finish(data.split('\n')[0] === 'OK hello proto=1' ? 'tap' : 'occupied');
    else if (data.length > 4096) finish('occupied');
});
JS
}

status="$(probe_server)"
if [[ "$status" == occupied ]]; then
    echo "Le port $cli_addr est occupé par un service qui ne répond pas comme un serveur TAP. Libérez le port ou choisissez SERVER_ADDR, CLI_ADDR et GUI_PORT correspondants." >&2
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

if [[ "$status" == tap ]]; then
    echo "Serveur TAP existant sur $cli_addr : réutilisation pour les deux clients."
else
    # Job control gives the server and its build/run children their own process group.
    set -m
    "$make_command" run-server "SERVER_ADDR=$server_addr" "WORLD_FILE=$world_file" &
    server_pid=$!
    set +m

    echo "Attente du serveur TAP sur $cli_addr…"
    deadline=$((SECONDS + 120))
    while true; do
        if ! kill -0 "$server_pid" 2>/dev/null; then
            wait "$server_pid"
            exit 1
        fi
        if [[ "$(probe_server)" == tap ]]; then
            break
        fi
        if (( SECONDS >= deadline )); then
            echo "Le serveur TAP ne répond pas sur $cli_addr après 120 secondes. Vérifiez que SERVER_ADDR et CLI_ADDR correspondent." >&2
            exit 1
        fi
        sleep 0.1
    done
fi

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

open_terminal 'TAP — CLI' run-client "CLI_ADDR=$cli_addr" &
open_terminal 'TAP — GUI' run-client-gui "GUI_HOST=$gui_host" "GUI_PORT=$gui_port" GUI_SERVER=off "WORLD_FILE=$world_file" &
if [[ -n "$server_pid" ]]; then
    echo 'Clients ouverts dans deux terminaux. Ctrl+C ici pour arrêter le serveur.'
    wait "$server_pid"
else
    echo 'Clients ouverts dans deux terminaux. Le serveur existant reste actif.'
    wait
fi
