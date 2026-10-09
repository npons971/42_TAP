#!/usr/bin/env bash
# Checks use only repository-local SDKs and leave source files unchanged.
set -euo pipefail
repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
export PATH="$repo:$PATH"
export GOPATH="$repo/.go-work" GOCACHE="$repo/.go-cache"
export npm_config_cache="$repo/.npm-cache"
[[ -x "$repo/go" ]] || { echo 'Outil Go local manquant : lancez make install.' >&2; exit 1; }

case "${1:-}" in
  test-gui)
    [[ -x "$repo/cc" ]] || { echo 'Compilateur local manquant : lancez make install.' >&2; exit 1; }
    cd Project/client-gui
    CGO_ENABLED=1 CC="$repo/cc" "$repo/go" test -race -count=1 -timeout=120s app.go app_test.go
    ;;
  lint-go)
    mapfile -t sources < <(git ls-files --cached --others --exclude-standard -- '*.go')
    unformatted=$("$repo/.go-sdk/go/bin/gofmt" -l "${sources[@]}")
    if [[ -n "$unformatted" ]]; then
      printf 'Fichiers Go à formater avec le gofmt local :\n%s\n' "$unformatted" >&2
      exit 1
    fi
    CGO_ENABLED=0 "$repo/go" vet ./...
    cd Project/client-gui
    # The TCP backend can be checked without installing the GTK/WebKit SDK.
    CGO_ENABLED=0 "$repo/go" vet app.go app_test.go
    ;;
  *) echo 'Usage: bash scripts/check-code.sh {test-gui|lint-go}' >&2; exit 2 ;;
esac
