#!/bin/sh
# agent-notify installer (macOS / Linux)
#   curl -fsSL https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.sh | sh
# Downloads the latest release binary, then registers the Claude Code
# hooks and login autostart via `agent-notify install`.
set -e

REPO="CoderGangW/agent-notify"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
case "$OS" in
  darwin|linux) ;;
  *) echo "unsupported OS: $OS (Windows: use install.ps1)" >&2; exit 1 ;;
esac

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"

URL="https://github.com/$REPO/releases/latest/download/agent-notify-$OS-$ARCH"
echo "downloading $URL"
curl -fsSL "$URL" -o "$BIN_DIR/agent-notify"
chmod +x "$BIN_DIR/agent-notify"

"$BIN_DIR/agent-notify" install

echo "installed: $BIN_DIR/agent-notify"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: add $BIN_DIR to your PATH" ;;
esac
if [ "$OS" = "darwin" ] && ! command -v terminal-notifier >/dev/null 2>&1; then
  echo "tip: 'brew install terminal-notifier' enables click-to-focus notifications"
fi
