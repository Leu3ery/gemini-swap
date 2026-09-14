#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
BUILD_DIR="$DIR/macos/.build/release"
APP_DIR="$DIR/dist/GeminiSwap.app"
ZIP_PATH="$DIR/dist/GeminiSwap-macOS.zip"
DMG_PATH="$DIR/dist/GeminiSwap-macOS.dmg"
VERIFY_DIR="$(mktemp -d /tmp/gemini-swap-verify.XXXXXX)"
DMG_STAGING="$(mktemp -d /tmp/gemini-swap-dmg.XXXXXX)"

cleanup() {
    rm -rf "$VERIFY_DIR" "$DMG_STAGING"
}
trap cleanup EXIT

echo "==> Building Go CLI..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$DIR/bin/gemini-swap" "$DIR/cmd/gemini-swap"

echo "==> Building Swift App (Release)..."
(cd "$DIR/macos" && swift build -c release)

echo "==> Packaging GeminiSwap.app..."
rm -rf "$APP_DIR" "$ZIP_PATH" "$DMG_PATH"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# Copy executables
cp "$BUILD_DIR/GeminiSwap" "$APP_DIR/Contents/MacOS/GeminiSwap"
cp "$DIR/bin/gemini-swap" "$APP_DIR/Contents/MacOS/gemini-swap"

# Copy Info.plist
cp "$DIR/macos/Resources/Info.plist" "$APP_DIR/Contents/Info.plist"
cp "$DIR/macos/Resources/DistributionNotice.txt" "$APP_DIR/Contents/Resources/DistributionNotice.txt"

chmod +x "$APP_DIR/Contents/MacOS/GeminiSwap"
chmod +x "$APP_DIR/Contents/MacOS/gemini-swap"

echo "==> Applying ad-hoc signature..."
codesign --force --deep --options runtime --sign - "$APP_DIR"
codesign --verify --deep --strict "$APP_DIR"

echo "==> Creating and verifying ZIP archive..."
ditto -c -k --sequesterRsrc --keepParent "$APP_DIR" "$ZIP_PATH"
ditto -x -k "$ZIP_PATH" "$VERIFY_DIR"
codesign --verify --deep --strict "$VERIFY_DIR/GeminiSwap.app"

echo "==> Creating DMG installer..."
cp -R "$APP_DIR" "$DMG_STAGING/GeminiSwap.app"
ln -s /Applications "$DMG_STAGING/Applications"
hdiutil create -volname "Gemini Swap" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG_PATH" >/dev/null
hdiutil verify "$DMG_PATH" >/dev/null

echo "✓ Successfully built $APP_DIR"
echo "✓ Successfully created $ZIP_PATH"
echo "✓ Successfully created $DMG_PATH"
