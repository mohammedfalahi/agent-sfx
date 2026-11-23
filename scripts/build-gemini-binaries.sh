#!/bin/sh
# Build the Agent SFX binaries used by the Gemini CLI extension and npm
# package (npm/bin/<platform-arch>/agent-sfx, launched by npm/bin/run.js).
#
# Usage: scripts/build-gemini-binaries.sh
#   The binaries are committed, like the Claude plugin's. Rebuild and commit
#   them whenever Go code changes. The build is reproducible (-trimpath,
#   -buildvcs=false, stripped), so unchanged source and the same Go version
#   give identical files.
#
# Targets macOS (darwin-arm64 and darwin-x64), the platforms shipped today.
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

for arch in arm64 amd64; do
  case "$arch" in
    arm64) dir=darwin-arm64 ;;
    amd64) dir=darwin-x64 ;;
  esac
  out="$repo_root/npm/bin/$dir/agent-sfx"
  mkdir -p "$(dirname -- "$out")"
  (cd "$repo_root" && CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$out" ./cmd/agent-sfx)
  echo "built $out"
done
