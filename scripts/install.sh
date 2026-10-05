#!/usr/bin/env bash
set -euo pipefail

# Build planfix and install the binary into BIN_DIR (default ~/.local/bin).

cd "$(dirname "${BASH_SOURCE[0]}")/.."

bin_dir="${BIN_DIR:-$HOME/.local/bin}"
module="github.com/6insanes/planfix-cli"
ldflags="-X $module/internal/buildinfo.Version=$(git describe --tags --always --dirty)"
ldflags="$ldflags -X $module/internal/buildinfo.Commit=$(git rev-parse --short HEAD)"
ldflags="$ldflags -X $module/internal/buildinfo.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mkdir -p "$bin_dir"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go build -trimpath -ldflags "$ldflags" -o "$tmp/planfix" .
mv "$tmp/planfix" "$bin_dir/planfix"

"$bin_dir/planfix" --version
