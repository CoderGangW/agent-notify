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
# Identity priority (ulio-style): explicit override > Developer ID >
# Apple Development. A real identity keeps the designated requirement —
# and therefore TCC grants — stable across releases.
IDENTITY="${CODESIGN_IDENTITY:-}"
if [ -z "$IDENTITY" ]; then
  IDS=$(security find-identity -v -p codesigning 2>/dev/null)
  for KIND in "Developer ID Application" "Apple Development"; do
    HIT=$(printf '%s\n' "$IDS" | sed -n "s/.*\"\($KIND[^\"]*\)\".*/\1/p" | head -1)
    if [ -n "$HIT" ]; then IDENTITY="$HIT"; break; fi
  done
fi
[ -n "$IDENTITY" ] || { echo "no codesigning identity found" >&2; exit 1; }

rm -rf dist && mkdir -p dist
echo "version $VERSION · signing as: $IDENTITY"

export MACOSX_DEPLOYMENT_TARGET=13.0
./build/patch-wails.sh
CGO_ENABLED=1 GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="-s -w" -o dist/agent-notify-darwin-arm64 .
CGO_ENABLED=1 GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="-s -w" -o dist/agent-notify-darwin-amd64 .

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

# Developer ID + notarization makes the app run on anyone's Mac; an
# Apple Development identity only runs on this machine's dev profile.
case "$IDENTITY" in
  "Developer ID Application"*)
    codesign --force --deep --timestamp --options runtime --sign "$IDENTITY" --identifier "$BUNDLE_ID" "$APP"
    ;;
  *)
    codesign --force --deep --timestamp --sign "$IDENTITY" --identifier "$BUNDLE_ID" "$APP"
    echo "note: '$IDENTITY' is not a Developer ID cert — other Macs will need"
    echo "      right-click→Open or 'xattr -cr agent-notify.app' on first run."
    ;;
esac
codesign --verify --strict "$APP" && echo "codesign OK"

(cd dist && zip -qry "agent-notify-macos-universal.app.zip" agent-notify.app)

# Notarize when a keychain profile exists (create once with:
#   xcrun notarytool store-credentials agent-notify --apple-id <id> --team-id <team> --password <app-specific-pw>)
if [ -n "$NOTARY_PROFILE" ]; then
  xcrun notarytool submit dist/agent-notify-macos-universal.app.zip \
    --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$APP"
  rm dist/agent-notify-macos-universal.app.zip
  (cd dist && zip -qry "agent-notify-macos-universal.app.zip" agent-notify.app)
  echo "notarized + stapled"
fi
ls -la dist/

if [ -n "$1" ]; then
  gh release upload "$1" \
    dist/agent-notify-darwin-arm64 \
    dist/agent-notify-darwin-amd64 \
    dist/agent-notify-macos-universal.app.zip --clobber
  echo "uploaded to release $1"
fi
