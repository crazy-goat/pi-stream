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

detect_checksum_cmd() {
    if command -v sha256sum >/dev/null 2>&1; then
        echo "sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        echo "shasum -a 256"
    else
        echo "no-sha256sum"
    fi
}

PLATFORM="$(detect_platform)"
CHECKSUM_CMD="$(detect_checksum_cmd)"

if [ "${CHECKSUM_CMD}" = "no-sha256sum" ]; then
    echo "Error: no sha256sum or shasum found — cannot verify binary checksum" >&2
    exit 1
fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT

BINARY_URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${PLATFORM}"
CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"

echo "Downloading ${BINARY} for ${PLATFORM}..."
curl -sSfL --connect-timeout 10 --max-time 60 "${BINARY_URL}" -o "${TMPDIR}/${BINARY}"
curl -sSfL --connect-timeout 10 --max-time 60 "${CHECKSUM_URL}" -o "${TMPDIR}/checksums.txt"

echo "Verifying checksum..."
grep -E "^[a-f0-9]{64}[[:space:]]{2}${BINARY}-${PLATFORM}$" "${TMPDIR}/checksums.txt" | (cd "${TMPDIR}" && ${CHECKSUM_CMD} -c -) || {
    echo "Checksum verification failed! Downloaded binary may be corrupted or tampered with." >&2
    exit 1
}

chmod +x "${TMPDIR}/${BINARY}"
mkdir -p "${INSTALL_DIR}"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" --version
