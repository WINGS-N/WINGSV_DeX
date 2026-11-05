#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod || ! -f Taskfile.yml ]]; then
  echo "Run this command from the WINGSV_DeX checkout: nix run ." >&2
  exit 1
fi

for path in external/vk-turn-proxy/client external/libXray/go.mod external/Xray-core/go.mod external/byedpi/Makefile; do
  if [[ ! -e "$path" ]]; then
    echo "Missing $path. Run: git submodule update --init --recursive" >&2
    exit 1
  fi
done

if git submodule status | grep -q '^[+-]'; then
  echo "Submodule revisions differ from this checkout. Run: git submodule update --init --recursive" >&2
  exit 1
fi

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/wingsv-dex/nix-run"
mkdir -p "$cache_dir"

if [[ ! -x "$cache_dir/wails3" ]] || [[ go.mod -nt "$cache_dir/wails3" ]]; then
  go build -o "$cache_dir/wails3" github.com/wailsapp/wails/v3/cmd/wails3
fi

export PATH="$cache_dir:$PATH"
task build
exec ./bin/wingsv-dex "$@"
