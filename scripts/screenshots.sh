#!/usr/bin/env bash
# Regenerates docs/images/*.png from the real UI with demo data
# (screenshots_test.go). Needs freeze: go install github.com/charmbracelet/freeze@latest
set -euo pipefail
cd "$(dirname "$0")/.."

freeze=$(command -v freeze || echo "$(go env GOPATH)/bin/freeze")
[ -x "$freeze" ] || { echo "install freeze: go install github.com/charmbracelet/freeze@latest" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
SWAT_SHOTS="$tmp" go test -tags screenshots -run TestScreenshots -count=1 . >/dev/null

for f in "$tmp"/*.ansi; do
  name=$(basename "$f" .ansi)
  "$freeze" -x "cat $f" --window --background "#0D1117" --padding 20 --margin 0 \
    --border.radius 10 --font.size 14 --line-height 1.25 -o "docs/images/$name.png" </dev/null >/dev/null
  # freeze renders at 4x; 2x keeps it sharp on retina screens at a fraction of the size.
  if command -v sips >/dev/null; then
    width=$(sips -g pixelWidth "docs/images/$name.png" | awk '/pixelWidth/ {print $2}')
    sips --resampleWidth $((width / 2)) "docs/images/$name.png" >/dev/null
  fi
  echo "docs/images/$name.png"
done
