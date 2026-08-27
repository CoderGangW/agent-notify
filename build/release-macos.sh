#!/bin/sh
# Builds the signed macOS app bundle + raw release binaries, locally only
# (macOS artifacts are not built in CI — signing happens on this machine).
#
#   build/release-macos.sh            # build into dist/
#   build/release-macos.sh v0.2.0     # build and `gh release upload` to that tag
#
# CODESIGN_IDENTITY overrides the signing identity (default: first valid
# codesigning identity in the keychain).
set -e
cd "$(dirname "$0")/.."

BUNDLE_ID="com.codergangw.claude-notify" # fixed — never change, TCC/launchd identity hangs off it
VERSION=$(sed -n 's/^const version = "\(.*\)"/\1/p' main.go)
IDENTITY=${CODESIGN_IDENTITY:-$(security find-identity -v -p codesigning | sed -n '1s/.*"\(.*\)"/\1/p')}
[ -n "$IDENTITY" ] || { echo "no codesigning identity found" >&2; exit 1; }

rm -rf dist && mkdir -p dist
echo "version $VERSION · signing as: $IDENTITY"

export MACOSX_DEPLOYMENT_TARGET=13.0
CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/agent-notify-darwin-arm64 .
CGO_ENABLED=1 GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/agent-notify-darwin-amd64 .

APP="dist/agent-notify.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
lipo -create dist/agent-notify-darwin-arm64 dist/agent-notify-darwin-amd64 \
  -output "$APP/Contents/MacOS/agent-notify"
cp assets/appicon.icns "$APP/Contents/Resources/appicon.icns"
cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
	<key>CFBundleName</key><string>agent-notify</string>
	<key>CFBundleDisplayName</key><string>agent-notify</string>
	<key>CFBundleExecutable</key><string>agent-notify</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$VERSION</string>
	<key>CFBundleVersion</key><string>$VERSION</string>
	<key>CFBundleIconFile</key><string>appicon</string>
	<key>LSMinimumSystemVersion</key><string>13.0</string>
	<key>LSUIElement</key><true/>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF
plutil -lint "$APP/Contents/Info.plist"

codesign --force --deep --sign "$IDENTITY" --identifier "$BUNDLE_ID" "$APP"
codesign --verify --strict "$APP" && echo "codesign OK"

(cd dist && zip -qry "agent-notify-macos-universal.app.zip" agent-notify.app)
ls -la dist/

if [ -n "$1" ]; then
  gh release upload "$1" \
    dist/agent-notify-darwin-arm64 \
    dist/agent-notify-darwin-amd64 \
    dist/agent-notify-macos-universal.app.zip --clobber
  echo "uploaded to release $1"
fi
