#!/usr/bin/env bash
# Renders every icon asset from the SVG masters (macOS; needs Google
# Chrome for rendering — it antialiases, blurs and grades like a design
# tool, which the old pure-Go rasterizer could not).
#
#   assets/appicon.svg  → assets/appicon.png (1024), assets/appicon.icns
#   frontend/logo.svg   → assets/logo.png (256), assets/icon.png (32),
#                         assets/appicon.ico, build/winres/appicon-256.png
#   assets/tray.svg     → assets/icon_mac.png (32, menu-bar template)
#
# Edit an SVG, run tools/icon.sh, rebuild.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

render() { # svg out.png size — square, transparent background
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars \
    --default-background-color=00000000 --window-size="$3,$3" \
    --screenshot="$2" "file://$1" >/dev/null 2>&1
}
shrink() { sips -z "$3" "$3" "$1" --out "$2" >/dev/null; } # src out size

cd "$ROOT"
render "$ROOT/assets/appicon.svg" "$TMP/app1024.png" 1024
render "$ROOT/frontend/logo.svg" "$TMP/flat1024.png" 1024
render "$ROOT/assets/tray.svg" "$TMP/tray512.png" 512

cp "$TMP/app1024.png" assets/appicon.png
shrink "$TMP/flat1024.png" assets/logo.png 256
shrink "$TMP/flat1024.png" assets/icon.png 32
shrink "$TMP/flat1024.png" build/winres/appicon-256.png 256
shrink "$TMP/tray512.png" assets/icon_mac.png 32

# macOS .icns
SET="$TMP/appicon.iconset"
mkdir -p "$SET"
for s in 16 32 128 256 512; do
  shrink "$TMP/app1024.png" "$SET/icon_${s}x${s}.png" "$s"
  shrink "$TMP/app1024.png" "$SET/icon_${s}x${s}@2x.png" "$((s * 2))"
done
iconutil -c icns "$SET" -o assets/appicon.icns

# Windows .ico: PNG-compressed entries (Vista+), written by hand
for s in 16 24 32 48 64 128 256; do shrink "$TMP/flat1024.png" "$TMP/ico$s.png" "$s"; done
python3 - "$TMP" <<'EOF'
import struct, sys
tmp = sys.argv[1]
sizes = [16, 24, 32, 48, 64, 128, 256]
blobs = [open(f"{tmp}/ico{s}.png", "rb").read() for s in sizes]
out = struct.pack("<HHH", 0, 1, len(sizes))
off = 6 + 16 * len(sizes)
for s, b in zip(sizes, blobs):
    out += struct.pack("<BBBBHHII", s % 256, s % 256, 0, 0, 1, 32, len(b), off)
    off += len(b)
open("assets/appicon.ico", "wb").write(out + b"".join(blobs))
EOF
echo "icons rendered"
