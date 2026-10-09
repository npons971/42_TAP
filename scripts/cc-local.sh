#!/usr/bin/env bash
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
[[ -x "$repo/.cc-sdk/zig" ]] || { echo 'Compilateur local absent. Lancez make install.' >&2; exit 1; }
export ZIG_GLOBAL_CACHE_DIR="$repo/.cc-cache"
export ZIG_LOCAL_CACHE_DIR="$repo/.cc-cache/local"
exec "$repo/.cc-sdk/zig" cc "$@"
