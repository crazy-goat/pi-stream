#!/usr/bin/env sh
set -eu

REPO="crazy-goat/pi-stream"
BINARY="pi-stream"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "${OS}" in
        Linux)  OS="linux"  ;;
        Darwin) OS="darwin" ;;
        *)
            echo "Unsupported OS: ${OS}" >&2
            exit 1
            ;;
    esac

    case "${ARCH}" in
        x86_64)          ARCH="amd64" ;;
        aarch64 | arm64) ARCH="arm64" ;;
        *)
            echo "Unsupported architecture: ${ARCH}" >&2
            exit 1
            ;;
    esac

    echo "${OS}-${ARCH}"
}

PLATFORM="$(detect_platform)"
URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${PLATFORM}"

echo "Downloading ${BINARY} for ${PLATFORM}..."
curl -sSfL "${URL}" -o "${BINARY}"
chmod +x "${BINARY}"

mkdir -p "${INSTALL_DIR}"
mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" --version
