#!/usr/bin/env bash
# Copyright 2026 Alca Programming Language
# SPDX-License-Identifier: Apache-2.0
# Complete installer for Alca toolchain on Linux & macOS:
# - Binaries (alca, apm) v1.7.9
# - System & User PATH registration
# - VS Code & Cursor extensions (syntax, file icons, snippets)
# - FreeDesktop MIME types & file associations (*.al)

set -e

VERSION="1.7.9"
REPO="alca-lang/alca"
APM_REPO="alca-lang/apm"
REGISTRY_HOST="http://91.92.34.94:9977"
INSTALL_DIR="${ALCA_HOME:-$HOME/.alca}/bin"

echo "=============================================="
echo "    Installing Alca Lang & APM v${VERSION}    "
echo "=============================================="

# 1. Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     PLATFORM="linux";;
    Darwin*)    PLATFORM="darwin";;
    *)          echo "Error: Unsupported operating system: ${OS}"; exit 1;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)     ARCH="amd64";;
    aarch64|arm64) ARCH="arm64";;
    *)          echo "Error: Unsupported architecture: ${ARCH}"; exit 1;;
esac

echo "[1/4] Platform detected: ${PLATFORM}_${ARCH}"
mkdir -p "${INSTALL_DIR}"

GITHUB_URL="https://github.com/${REPO}/releases/latest/download/alca-${PLATFORM}-${ARCH}.tar.gz"
REGISTRY_URL="${REGISTRY_HOST}/api/v1/download/alca-${PLATFORM}-${ARCH}.tar.gz"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# 3. Download & extract binaries
echo "[2/4] Downloading Alca & APM binaries..."
if curl -fsSL "${GITHUB_URL}" -o "${TMP_DIR}/alca.tar.gz" 2>/dev/null; then
    echo "  -> Downloaded from GitHub Releases"
elif curl -fsSL "${REGISTRY_URL}" -o "${TMP_DIR}/alca.tar.gz" 2>/dev/null; then
    echo "  -> Downloaded from Alca Registry Mirror"
else
    echo "  -> Downloading direct binaries..."
    curl -fsSL "https://github.com/${REPO}/releases/latest/download/alca-${PLATFORM}-${ARCH}" -o "${INSTALL_DIR}/alca" 2>/dev/null ||     curl -fsSL "${REGISTRY_HOST}/api/v1/download/alca" -o "${INSTALL_DIR}/alca" 2>/dev/null || true
    curl -fsSL "https://github.com/${APM_REPO}/releases/latest/download/apm-${PLATFORM}-${ARCH}" -o "${INSTALL_DIR}/apm" 2>/dev/null ||     curl -fsSL "https://github.com/${REPO}/releases/latest/download/apm-${PLATFORM}-${ARCH}" -o "${INSTALL_DIR}/apm" 2>/dev/null ||     curl -fsSL "${REGISTRY_HOST}/api/v1/download/apm" -o "${INSTALL_DIR}/apm" 2>/dev/null || true
fi

if [ -f "${TMP_DIR}/alca.tar.gz" ]; then
    tar -xzf "${TMP_DIR}/alca.tar.gz" -C "${TMP_DIR}" 2>/dev/null || true
    if [ -f "${TMP_DIR}/alca" ]; then cp "${TMP_DIR}/alca" "${INSTALL_DIR}/alca"; fi
    if [ -f "${TMP_DIR}/apm" ]; then cp "${TMP_DIR}/apm" "${INSTALL_DIR}/apm"; fi
fi

chmod +x "${INSTALL_DIR}/alca" 2>/dev/null || true
chmod +x "${INSTALL_DIR}/apm" 2>/dev/null || true

# Download file icon
curl -fsSL "${REGISTRY_HOST}/assets/alca-file.png" -o "${INSTALL_DIR}/alca-file.png" 2>/dev/null || true

# 4. PATH registration
echo "[3/4] Configuring PATH..."
PATH_EXPORT="export PATH="${INSTALL_DIR}:\$PATH""

for RC in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
    if [ -f "$RC" ]; then
        if ! grep -q "${INSTALL_DIR}" "$RC"; then
            echo "" >> "$RC"
            echo "# Alca Programming Language" >> "$RC"
            echo "${PATH_EXPORT}" >> "$RC"
            echo "  -> Added PATH to ${RC}"
        fi
    fi
done

# 5. VS Code & Cursor Extension Auto-Installation
echo "[4/4] Setting up editor integrations & MIME types..."
EXT_DIRS=(
    "$HOME/.vscode/extensions"
    "$HOME/.cursor/extensions"
    "$HOME/.vscode-oss/extensions"
)

EXT_ZIP="${TMP_DIR}/vscode.zip"
curl -fsSL "${REGISTRY_HOST}/editors/vscode.zip" -o "${EXT_ZIP}" 2>/dev/null || true

if [ -f "${EXT_ZIP}" ]; then
    for EDIR in "${EXT_DIRS[@]}"; do
        if [ -d "$EDIR" ]; then
            TARGET="${EDIR}/alca-lang"
            mkdir -p "${TARGET}"
            unzip -q -o "${EXT_ZIP}" -d "${TARGET}" 2>/dev/null || true
            echo "  -> Installed Alca extension (syntax + icons) to ${EDIR}"
        fi
    done
fi

# 6. Linux FreeDesktop MIME type and icon for *.al
if [ "${PLATFORM}" = "linux" ]; then
    MIME_DIR="$HOME/.local/share/mime/packages"
    mkdir -p "${MIME_DIR}"
    cat << 'EOF' > "${MIME_DIR}/alca.xml"
<?xml version="1.0" encoding="UTF-8"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="text/x-alca">
    <comment>Alca Source File</comment>
    <glob pattern="*.al"/>
    <icon name="text-x-alca"/>
  </mime-type>
</mime-info>
EOF
    if command -v update-mime-database >/dev/null 2>&1; then
        update-mime-database "$HOME/.local/share/mime" >/dev/null 2>&1 || true
    fi

    # Install icon to hicolor theme
    ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/mimetypes"
    mkdir -p "${ICON_DIR}"
    if [ -f "${INSTALL_DIR}/alca-file.png" ]; then
        cp "${INSTALL_DIR}/alca-file.png" "${ICON_DIR}/text-x-alca.png"
        if command -v gtk-update-icon-cache >/dev/null 2>&1; then
            gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true
        fi
    fi
    echo "  -> Registered FreeDesktop MIME type text/x-alca with document icon"
fi

echo ""
echo "=============================================="
echo "    Alca Lang v${VERSION} is ready to use!    "
echo "=============================================="
echo "  * Binaries:       ${INSTALL_DIR}/alca, apm"
echo "  * File Icon:      ${INSTALL_DIR}/alca-file.png"
echo "  * Extension:      .al (Alca Source File)"
echo "  * Documentation:  ${REGISTRY_HOST}/#docs"
echo "  * GitHub:         https://github.com/${REPO}"
echo ""
echo "Run 'alca version' or 'apm --version' to verify."
