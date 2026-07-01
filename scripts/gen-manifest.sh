#!/usr/bin/env bash
#
# Generate the self-update manifest.json the agent fetches. It maps each
# "<goos>/<goarch>" to the raw binary's download URL on this repo's GitHub
# Release and its SHA-256. Prints the manifest to stdout.
#
#   REPO=tlmanz/klutch-agent scripts/gen-manifest.sh v1.2.3 dist > dist/manifest.json
#
# <bindir> must contain the raw per-platform binaries named exactly:
#   klutch-agent-linux-amd64   klutch-agent-linux-arm64
#   klutch-agent-windows-amd64.exe
#   klutch-agent-darwin-amd64  klutch-agent-darwin-arm64
# Missing binaries are skipped (with a note on stderr), so the manifest only
# advertises platforms that were actually built.
set -euo pipefail

VERSION="${1:?usage: gen-manifest.sh <version> <bindir>}"
BINDIR="${2:?usage: gen-manifest.sh <version> <bindir>}"
REPO="${REPO:-tlmanz/klutch-agent}"
BASE="https://github.com/$REPO/releases/download/$VERSION"

# key:asset pairs (goos/goarch → release asset filename).
PAIRS="
linux/amd64:klutch-agent-linux-amd64
linux/arm64:klutch-agent-linux-arm64
windows/amd64:klutch-agent-windows-amd64.exe
darwin/amd64:klutch-agent-darwin-amd64
darwin/arm64:klutch-agent-darwin-arm64
"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

entries=""
sep=""
for pair in $PAIRS; do
  key="${pair%%:*}"
  asset="${pair#*:}"
  path="$BINDIR/$asset"
  if [ ! -f "$path" ]; then
    echo "gen-manifest: skip $asset (not found)" >&2
    continue
  fi
  sum="$(sha256 "$path")"
  entries="${entries}${sep}
    \"$key\": {\"url\": \"$BASE/$asset\", \"sha256\": \"$sum\"}"
  sep=","
done

printf '{\n  "version": "%s",\n  "artifacts": {%s\n  }\n}\n' "$VERSION" "$entries"
