#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
[[ -x "$repo/.cc-sdk/zig" ]] || { echo 'Compilateur local absent. Lancez make install.' >&2; exit 1; }
export ZIG_GLOBAL_CACHE_DIR="$repo/.cc-cache"
export ZIG_LOCAL_CACHE_DIR="$repo/.cc-cache/local"
if [[ -r "$repo/.webkit-sdk/.zig-libc" ]]; then
  export ZIG_LIBC="$repo/.webkit-sdk/.zig-libc"
  headers=(-nostdinc -isystem "$repo/.cc-sdk/lib/include" -isystem "$repo/.webkit-sdk/usr/include")
  if [[ -d "$repo/.webkit-sdk/usr/include/x86_64-linux-gnu" ]]; then
    headers+=(-isystem "$repo/.webkit-sdk/usr/include/x86_64-linux-gnu")
  fi
  exec "$repo/.cc-sdk/zig" cc "${headers[@]}" "$@"
fi
exec "$repo/.cc-sdk/zig" cc "$@"
