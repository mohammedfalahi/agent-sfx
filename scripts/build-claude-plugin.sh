#!/bin/sh
# Build the Agent SFX binaries into the Claude Code plugin directory.
#
# Usage: scripts/build-claude-plugin.sh [output-plugin-dir]
#   Default output: npm/claude (in this repository). The binaries are build
#   artifacts and are ignored by git; run this before loading the plugin with
#   `claude --plugin-dir` or validating it.
#
# Targets macOS (darwin-arm64 and darwin-x64). Requires Go; no network access
# is needed when module dependencies are already in the local module cache.
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
plugin_dir=${1:-"$repo_root/npm/claude"}

if [ ! -f "$plugin_dir/.claude-plugin/plugin.json" ]; then
  echo "error: $plugin_dir does not look like the Agent SFX Claude plugin" >&2
  exit 1
fi

for arch in arm64 amd64; do
  case "$arch" in
    arm64) dir=darwin-arm64 ;;
    amd64) dir=darwin-x64 ;;
  esac
  out="$plugin_dir/runtime/$dir/agent-sfx"
  mkdir -p "$(dirname -- "$out")"
  (cd "$repo_root" && CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -o "$out" ./cmd/agent-sfx)
  echo "built $out"
done
