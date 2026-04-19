#!/bin/sh
# AgentClash CLI install script
# Usage: curl -fsSL https://raw.githubusercontent.com/agentclash/agentclash/main/scripts/install/install.sh | sh
set -eu

REPO="${REPO:-agentclash/agentclash}"
BINARY="${BINARY:-agentclash}"
DEFAULT_INSTALL_DIR="/usr/local/bin"
INSTALL_DIR_WAS_SET=0

if [ "${INSTALL_DIR+x}" = "x" ]; then
    INSTALL_DIR_WAS_SET=1
else
    INSTALL_DIR="$DEFAULT_INSTALL_DIR"
fi

err() {
    echo "Error: $*" >&2
    exit 1
}

has_command() {
    command -v "$1" >/dev/null 2>&1
}

need_command() {
    has_command "$1" || err "missing required command: $1"
}

download() {
    url="$1"
    output="$2"
    curl -fsSL --retry 3 --retry-delay 1 "$url" -o "$output"
}

check_url() {
    url="$1"
    curl -fsIL --retry 3 --retry-delay 1 "$url" >/dev/null
}

sha256_file() {
    file="$1"
    if has_command sha256sum; then
        sha256sum "$file" | awk '{print $1}'
    elif has_command shasum; then
        shasum -a 256 "$file" | awk '{print $1}'
    else
        err "missing sha256 checksum tool: install sha256sum or shasum"
    fi
}

need_command curl
need_command tar
need_command awk
need_command sed
need_command uname
need_command mktemp

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) err "unsupported architecture: $ARCH" ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) err "unsupported OS: $OS. Use scripts/install/install.ps1 on Windows." ;;
esac

if [ "${VERSION:-}" = "" ]; then
    VERSION=$(curl -fsSL --retry 3 --retry-delay 1 "https://api.github.com/repos/${REPO}/releases/latest" \
        | sed -n 's/.*"tag_name":[[:space:]]*"\(v[^"]*\)".*/\1/p' \
        | head -1)

    if [ "$VERSION" = "" ]; then
        err "could not determine latest version from GitHub releases"
    fi
fi

case "$VERSION" in
    v*) ;;
    *) err "VERSION must be a release tag like v0.1.2, got: $VERSION" ;;
esac

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${FILENAME}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})..."

check_url "$ARCHIVE_URL" || err "release asset not found: $ARCHIVE_URL"
check_url "$CHECKSUMS_URL" || err "release checksums not found: $CHECKSUMS_URL"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT HUP INT TERM

ARCHIVE_PATH="${TMPDIR}/${FILENAME}"
CHECKSUMS_PATH="${TMPDIR}/checksums.txt"

download "$ARCHIVE_URL" "$ARCHIVE_PATH"
download "$CHECKSUMS_URL" "$CHECKSUMS_PATH"

EXPECTED_SHA=$(awk -v file="$FILENAME" '$2 == file { print $1 }' "$CHECKSUMS_PATH" | head -1)
if [ "$EXPECTED_SHA" = "" ]; then
    err "checksums.txt does not contain an entry for $FILENAME"
fi

ACTUAL_SHA=$(sha256_file "$ARCHIVE_PATH")
if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
    err "checksum mismatch for $FILENAME"
fi

tar -xzf "$ARCHIVE_PATH" -C "$TMPDIR"
if [ ! -f "${TMPDIR}/${BINARY}" ]; then
    err "archive did not contain ${BINARY}"
fi

if [ "$INSTALL_DIR_WAS_SET" -eq 0 ] && [ ! -w "$INSTALL_DIR" ]; then
    if has_command sudo && { [ -t 0 ] || [ -r /dev/tty ]; }; then
        USE_SUDO=1
    else
        INSTALL_DIR="${HOME}/.local/bin"
        USE_SUDO=0
        echo "No write access to ${DEFAULT_INSTALL_DIR}; installing to ${INSTALL_DIR}."
    fi
else
    USE_SUDO=0
fi

if [ "$USE_SUDO" -eq 1 ]; then
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mkdir -p "$INSTALL_DIR"
    sudo install -m 755 "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
    mkdir -p "$INSTALL_DIR"
    install -m 755 "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"

case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        echo ""
        echo "Add ${INSTALL_DIR} to PATH if your shell cannot find ${BINARY}:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        ;;
esac

echo ""
echo "Get started:"
echo "  ${BINARY} auth login"
echo "  ${BINARY} --help"
echo ""
echo "Uninstall this script install:"
echo "  rm -f \"${INSTALL_DIR}/${BINARY}\""
