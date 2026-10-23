#!/bin/sh
# agent-notify installer (macOS / Linux)
#   curl -fsSL https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.sh | sh
# Downloads the latest release (macOS: agent-notify.app, Linux: binary),
# then registers the Claude Code hooks and login autostart via
# `agent-notify install`.
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

if [ "$OS" = "darwin" ]; then
  # Notifications only work from the .app bundle, so macOS gets the
  # universal app in /Applications (the path autostart and the updater
  # expect); `install` still copies the CLI to ~/.local/bin for hooks.
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT
  URL="https://github.com/$REPO/releases/latest/download/agent-notify-macos-universal.app.zip"
  echo "downloading $URL"
  curl -fsSL "$URL" -o "$TMP/app.zip"
  ditto -x -k "$TMP/app.zip" "$TMP"
  rm -rf /Applications/agent-notify.app
  mv "$TMP/agent-notify.app" /Applications/
  /Applications/agent-notify.app/Contents/MacOS/agent-notify install
  echo "installed: /Applications/agent-notify.app"
else
  mkdir -p "$BIN_DIR"
  URL="https://github.com/$REPO/releases/latest/download/agent-notify-$OS-$ARCH"
  echo "downloading $URL"
  curl -fsSL "$URL" -o "$BIN_DIR/agent-notify"
  chmod +x "$BIN_DIR/agent-notify"
  "$BIN_DIR/agent-notify" install
  echo "installed: $BIN_DIR/agent-notify"
fi

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "note: add $BIN_DIR to your PATH" ;;
esac
