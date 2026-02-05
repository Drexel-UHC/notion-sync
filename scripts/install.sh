#!/bin/bash
#
# Install script for notion-sync
# Usage: curl -fsSL https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.sh | bash
#

set -e

REPO="ran-codes/notion-sync"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Map OS names
case "$OS" in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        ;;
    mingw*|msys*|cygwin*)
        OS="windows"
        EXT=".exe"
        ;;
    *)
        echo "Unsupported operating system: $OS"
        exit 1
        ;;
esac

BINARY_NAME="notion-sync-${OS}-${ARCH}${EXT}"

echo "Detected: ${OS}/${ARCH}"
echo "Downloading ${BINARY_NAME}..."

# Get latest release URL
LATEST_URL=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep "browser_download_url.*${BINARY_NAME}" | cut -d '"' -f 4)

if [ -z "$LATEST_URL" ]; then
    echo "Could not find release for ${BINARY_NAME}"
    echo "Please check https://github.com/${REPO}/releases for available downloads"
    exit 1
fi

# Download to temp file
TMP_FILE=$(mktemp)
curl -fsSL "$LATEST_URL" -o "$TMP_FILE"
chmod +x "$TMP_FILE"

# Install
INSTALL_PATH="${INSTALL_DIR}/notion-sync"

if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$INSTALL_PATH"
else
    echo "Installing to ${INSTALL_PATH} (requires sudo)..."
    sudo mv "$TMP_FILE" "$INSTALL_PATH"
fi

echo "Installed notion-sync to ${INSTALL_PATH}"
echo ""
echo "Get started:"
echo "  notion-sync config set apiKey <your-notion-api-key>"
echo "  notion-sync import <database-id> --output ./my-notes"
echo ""
echo "Run 'notion-sync --help' for more information."
