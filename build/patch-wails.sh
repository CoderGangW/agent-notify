#!/bin/sh
# Vendors dependencies and applies the agent-notify Wails patch:
# on macOS 26/27 a status-item left click can reach the click callback
# with currentEvent.type != LeftMouseDown (observed 5 / MouseMoved), and
# stock Wails silently drops it — treat any non-right event as left.
# Idempotent; macOS builds must then use `go build -mod=vendor`.
set -e
cd "$(dirname "$0")/.."
# vendor exits nonzero over a windows-only embed (WebView2Loader.dll)
# that's absent on macOS; the tree is still written completely
go mod vendor || true
F=vendor/github.com/wailsapp/wails/v3/pkg/application/systemtray_darwin.go
[ -f "$F" ] || { echo "vendor failed: $F missing" >&2; exit 1; }
grep -q "agent-notify patch" "$F" && { echo "wails already patched"; exit 0; }
python3 - "$F" <<'PY'
import sys, pathlib
p = pathlib.Path(sys.argv[1]); t = p.read_text()
old = """//export systrayClickCallback
func systrayClickCallback(id C.long, buttonID C.int) {"""
new = """//export systrayClickCallback
func systrayClickCallback(id C.long, buttonID C.int) {
	// agent-notify patch: on macOS 26/27 the status-item action can fire
	// with currentEvent.type != LeftMouseDown for plain left clicks
	// (observed type 5 / MouseMoved); anything that isn't a right-button
	// event is treated as a left click instead of being dropped.
	if buttonID != 3 && buttonID != 4 {
		buttonID = 1
	}"""
assert old in t
p.write_text(t.replace(old, new))
PY
echo "wails patched"
