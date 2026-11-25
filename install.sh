#!/usr/bin/env bash
# ULang++ Programming Language Installer for Linux / macOS

set -e

INSTALL_DIR="${HOME}/.local/share/ulang++"
BIN_DIR="${HOME}/.local/bin"
LIBS_DIR="/var/lib/ULangLibs/libs"

if [ "$EUID" -ne 0 ]; then
    LIBS_DIR="${HOME}/.local/share/ULangLibs/libs"
fi

echo "=========================================="
echo "   ULang++ Language Toolchain Installer   "
echo "=========================================="

mkdir -p "${LIBS_DIR}"
mkdir -p "${INSTALL_DIR}/bin"
mkdir -p "${INSTALL_DIR}/std"
mkdir -p "${BIN_DIR}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

EXE="${SCRIPT_DIR}/compiler/build_bin/ulang++"
if [ ! -f "$EXE" ]; then
    EXE="${SCRIPT_DIR}/compiler/build_bin/ulang++"
fi

if [ -f "$EXE" ]; then
    cp -f "$EXE" "${INSTALL_DIR}/bin/ulang++"
    ln -sf "${INSTALL_DIR}/bin/ulang++" "${BIN_DIR}/ulang++"
    ln -sf "${INSTALL_DIR}/bin/ulang++" "${BIN_DIR}/ulang++"
    echo "[+] Installed compiler to ${BIN_DIR}/ulang++"
fi

if [ -d "${SCRIPT_DIR}/std" ]; then
    cp -rf "${SCRIPT_DIR}/std/"* "${INSTALL_DIR}/std/"
    echo "[+] Installed stdlib to ${INSTALL_DIR}/std"
fi

echo "[+] Created custom libraries directory: ${LIBS_DIR}"
echo "=========================================="
echo "    Installation Finished Successfully!   "
echo "=========================================="
