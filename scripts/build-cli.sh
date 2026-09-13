#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
DIST="$DIR/dist"
mkdir -p "$DIST"

VERSION="1.1.4"

echo "==> Building cross-platform CLI binaries..."

TARGETS=(
  "darwin/arm64/gemini-swap-darwin-arm64"
  "darwin/amd64/gemini-swap-darwin-amd64"
  "linux/amd64/gemini-swap-linux-amd64"
  "linux/arm64/gemini-swap-linux-arm64"
  "windows/amd64/gemini-swap-windows-amd64.exe"
)

for TARGET in "${TARGETS[@]}"; do
  IFS="/" read -r GOOS GOARCH OUTNAME <<< "$TARGET"
  echo "Building $GOOS/$GOARCH -> $OUTNAME..."
  GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -ldflags="-s -w" -o "$DIST/$OUTNAME" "$DIR/cmd/gemini-swap"

  if [ "$GOOS" = "windows" ]; then
    (cd "$DIST" && zip -q "${OUTNAME%.exe}.zip" "$OUTNAME")
  else
    (cd "$DIST" && tar -czf "${OUTNAME}.tar.gz" "$OUTNAME")
  fi
done

echo "✓ All CLI targets compiled and packaged into $DIST"
ls -lh "$DIST"
