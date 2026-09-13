#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
BUILD_DIR="$DIR/macos/.build/release"
APP_DIR="$DIR/dist/GeminiSwap.app"

echo "==> Building Go CLI..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$DIR/bin/gemini-swap" "$DIR/cmd/gemini-swap"

echo "==> Building Swift App (Release)..."
(cd "$DIR/macos" && swift build -c release)

echo "==> Packaging GeminiSwap.app..."
rm -rf "$DIR/dist/GeminiSwap.app" "$DIR/dist/GeminiSwap-macOS.zip"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# Copy executables
cp "$BUILD_DIR/GeminiSwap" "$APP_DIR/Contents/MacOS/GeminiSwap"
cp "$DIR/bin/gemini-swap" "$APP_DIR/Contents/MacOS/gemini-swap"

# Copy Info.plist
cp "$DIR/macos/Resources/Info.plist" "$APP_DIR/Contents/Info.plist"

chmod +x "$APP_DIR/Contents/MacOS/GeminiSwap"
chmod +x "$APP_DIR/Contents/MacOS/gemini-swap"

echo "==> Creating ZIP archive for GitHub Releases..."
(cd "$DIR/dist" && zip -r -q "GeminiSwap-macOS.zip" "GeminiSwap.app")

echo "✓ Successfully built $APP_DIR"
echo "✓ Successfully created $DIR/dist/GeminiSwap-macOS.zip"
