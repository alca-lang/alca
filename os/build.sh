#!/usr/bin/env bash
# Build the ULang++ UEFI terminal OS.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS_DIR="$ROOT/os"
ESP="$OS_DIR/esp/EFI/BOOT"
CXX="${CXX:-g++}"

echo "[1/3] Building native compiler..."
SRCS=()
while IFS= read -r f; do
    case "$f" in
        */runtime/*|*/main_runner.cpp|*/libulang.cpp) continue ;;
    esac
    SRCS+=("$f")
done < <(find "$ROOT/compiler/src" -name '*.cpp' -type f)
"$CXX" -std=c++17 -O1 -I"$ROOT/compiler/src" "${SRCS[@]}" \
    -o "$OS_DIR/ulang++"

echo "[2/3] Compiling EFI application..."
mkdir -p "$ESP"
# Source files import std.* next to the compiler executable.
rm -rf "$OS_DIR/std"
ln -s "$ROOT/std" "$OS_DIR/std"
"$OS_DIR/ulang++" build --target efi-app \
    -o "$ESP/BOOTX64.EFI" "$OS_DIR/main.upp"

echo "[3/3] Image ready: $ESP/BOOTX64.EFI"
echo "Run with: OVMF_FD=/path/to/OVMF.fd bash $OS_DIR/run.sh"
