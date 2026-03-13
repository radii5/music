#!/usr/bin/env sh
# radii5 quick-install
# Usage:  curl -fsSL https://raw.githubusercontent.com/radii5/radii5/main/scripts/install.sh | sh
set -e

REPO="radii5/music"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "✗ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux)  PLATFORM="linux-$ARCH" ;;
  darwin) PLATFORM="macos-$ARCH" ;;
  *)
    echo "✗ Unsupported OS: $OS — use install.go on Windows"
    exit 1
    ;;
esac

echo ""
echo "  radii5 installer"
echo "  platform: $PLATFORM"
echo ""

# ── resolve latest tag ────────────────────────────────────────────────────────
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "✗ Could not determine latest release. Is the repo published?"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$LATEST/radii5-$PLATFORM"
DEST="$INSTALL_DIR/radii5"

echo "  → radii5 $LATEST"

# check write permission — offer sudo if needed
if [ -w "$INSTALL_DIR" ]; then
  curl -fL --progress-bar "$URL" -o "$DEST"
  chmod +x "$DEST"
else
  echo "  (needs sudo to write to $INSTALL_DIR)"
  sudo curl -fL --progress-bar "$URL" -o "$DEST"
  sudo chmod +x "$DEST"
fi

echo ""
echo "  ✓ radii5 installed → $DEST"
echo ""

# ── yt-dlp ────────────────────────────────────────────────────────────────────
if command -v yt-dlp >/dev/null 2>&1; then
  echo "  ✓ yt-dlp already installed"
else
  echo "  → yt-dlp (latest)"
  YT_URL=""
  case "$OS" in
    linux)
      case "$ARCH" in
        arm64) YT_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64" ;;
        *)     YT_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux" ;;
      esac
      ;;
    darwin)
      case "$ARCH" in
        arm64) YT_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos" ;;
        *)     YT_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos_legacy" ;;
      esac
      ;;
  esac

  YT_DEST="$INSTALL_DIR/yt-dlp"
  if [ -w "$INSTALL_DIR" ]; then
    curl -fL --progress-bar "$YT_URL" -o "$YT_DEST"
    chmod +x "$YT_DEST"
  else
    sudo curl -fL --progress-bar "$YT_URL" -o "$YT_DEST"
    sudo chmod +x "$YT_DEST"
  fi
  echo "  ✓ yt-dlp installed → $YT_DEST"
fi

echo ""
echo "  All done! Try:  radii5 --version"
echo ""
