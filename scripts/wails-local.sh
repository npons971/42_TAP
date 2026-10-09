#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
for tool in go node npm cc .go-work/bin/wails; do
  [[ -x "$repo/$tool" ]] || { echo "Outil local manquant : $tool. Lancez make install." >&2; exit 1; }
done
export PATH="$repo:$repo/.webkit-sdk/usr/bin:$PATH"
export CC="$repo/cc" CGO_ENABLED=1
export GOPATH="$repo/.go-work" GOCACHE="$repo/.go-cache"
export npm_config_cache="$repo/.npm-cache"
export PKG_CONFIG_PATH="$repo/.webkit-sdk/usr/lib64/pkgconfig:$repo/.webkit-sdk/usr/lib/x86_64-linux-gnu/pkgconfig:$repo/.webkit-sdk/usr/lib/pkgconfig:$repo/.webkit-sdk/usr/share/pkgconfig:${PKG_CONFIG_PATH:-}"
export LD_LIBRARY_PATH="$repo/.webkit-sdk/usr/lib64:$repo/.webkit-sdk/usr/lib/x86_64-linux-gnu:$repo/.webkit-sdk/usr/lib:${LD_LIBRARY_PATH:-}"
# Root-level ./wails commands always target this repository's GUI.
if [[ "$PWD" == "$repo" ]]; then cd "$repo/Project/client-gui"; fi
exec "$repo/.go-work/bin/wails" "$@"
