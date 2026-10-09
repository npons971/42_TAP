#!/usr/bin/env bash
set -uo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
sdk="$repo/.webkit-sdk/usr"
export PATH="$repo:$sdk/bin:$PATH"
export GOPATH="$repo/.go-work" GOCACHE="$repo/.go-cache"
export npm_config_cache="$repo/.npm-cache"
export LD_LIBRARY_PATH="$sdk/lib64:$sdk/lib/x86_64-linux-gnu:$sdk/lib:${LD_LIBRARY_PATH:-}"
export PKG_CONFIG_PATH="$sdk/lib64/pkgconfig:$sdk/lib/x86_64-linux-gnu/pkgconfig:$sdk/lib/pkgconfig:$sdk/share/pkgconfig:${PKG_CONFIG_PATH:-}"
status=0
check() {
  local label=$1
  shift
  if "$@" >/dev/null 2>&1; then printf '  OK  %s\n' "$label"
  else printf '  MANQUANT  %s\n' "$label"; status=1; fi
}
printf '%s\n' '42 TAP — environnement local du GUI'
check 'Go local' "$repo/go" version
check 'Node.js local' "$repo/node" --version
check 'npm local' "$repo/npm" --version
check 'Wails local' test -x "$repo/.go-work/bin/wails"
check 'Compilateur C local' "$repo/cc" --version
check 'pkg-config local' "$sdk/bin/pkg-config" --version
check 'GTK3 et WebKitGTK 4.1 locaux' "$sdk/bin/pkg-config" --exists gtk+-3.0 webkit2gtk-4.1
if [[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]]; then
  printf '%s\n' '  OK  Session graphique annoncée (connexion à vérifier au lancement)'
else
  printf '%s\n' '  INFO  Pas de session graphique ; make build-gui reste disponible.'
fi
if [[ "$status" == 0 ]]; then printf '%s\n' 'Prêt : make gui'
else printf '%s\n' 'Dépendances manquantes : make install'; fi
exit "$status"
