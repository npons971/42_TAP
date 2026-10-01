#!/usr/bin/env bash
# Présentation commune aux tâches Make ; les outils restent locaux au dépôt.
set -euo pipefail

repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
export PATH="$repo:$PATH"
export GOPATH="$repo/.go-work"
export GOCACHE="$repo/.go-cache"
export npm_config_cache="$repo/.npm-cache"

blue='' green='' red='' dim='' reset=''
if [[ -t 1 && ${TERM:-dumb} != dumb && ! -v NO_COLOR ]]; then
  blue=$'\033[1;36m' green=$'\033[1;32m' red=$'\033[1;31m'
  dim=$'\033[2m' reset=$'\033[0m'
fi

mode=$1
label=$2
shift 2
download_archive() {
  local destination=$1 checksum=$2 url=$3
  curl -fsSL --retry 3 -o "$destination.tmp" "$url" || return $?
  printf '%s  %s\n' "$checksum" "$destination.tmp" | sha256sum -c - || return $?
  mv "$destination.tmp" "$destination"
}
if [[ $mode == download ]]; then
  mode=run
  set -- download_archive "$@"
fi
if [[ $mode == title ]]; then
  printf '\n%s  42 TAP%s · %s\n\n' "$blue" "$reset" "$label"
  exit 0
fi
if [[ $mode == say ]]; then
  printf '%s  ›%s %s\n' "$blue" "$reset" "$label"
  exit 0
fi
[[ $mode == run ]] || exit 2

printf '%s  ›%s %s\n' "$blue" "$reset" "$label"
if [[ ${V:-0} == 1 ]]; then
  "$@"
  exit $?
fi

mkdir -p "$repo/.build/logs"
log=$(mktemp "$repo/.build/logs/make.XXXXXX.log")
started=$SECONDS
spinner_pid=''
cleanup() {
  if [[ -n $spinner_pid ]]; then
    kill "$spinner_pid" 2>/dev/null || true
    wait "$spinner_pid" 2>/dev/null || true
    printf '\r\033[2K'
  fi
}
trap cleanup EXIT
if [[ -t 1 && ${TERM:-dumb} != dumb ]]; then
  (
    while :; do
      for frame in '⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏'; do
        printf '\r%s  %s%s %s %s(%ss)%s' "$blue" "$frame" "$reset" "$label" "$dim" "$((SECONDS - started))" "$reset"
        sleep 0.15
      done
    done
  ) &
  spinner_pid=$!
fi
if "$@" >"$log" 2>&1; then
  cleanup
  spinner_pid=''
  printf '%s  ✓%s %s %s(%ss)%s\n' "$green" "$reset" "$label" "$dim" "$((SECONDS - started))" "$reset"
else
  status=$?
  cleanup
  spinner_pid=''
  printf '%s  ✗%s %s\n' "$red" "$reset" "$label" >&2
  tail -n 40 "$log" >&2
  printf '\n  Log complet : %s\n' "${log#"$repo/"}" >&2
  exit "$status"
fi
