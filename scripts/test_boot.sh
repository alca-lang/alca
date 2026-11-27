#!/usr/bin/env bash
# Full freestanding smoke test: build the native compiler, compile a minimal
# kernel, boot it in QEMU and assert the serial markers.  Requires g++ and
# qemu-system-x86 (CI installs both).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${TMPDIR:-/tmp}/ulang-boot-test"
rm -rf "$OUT"
mkdir -p "$OUT"

CXX="${CXX:-g++}"
SOURCES=()
while IFS= read -r f; do
    case "$f" in
        */runtime/*|*/main_runner.cpp) continue ;;
    esac
    SOURCES+=("$f")
done < <(find "$ROOT/compiler/src" -name '*.cpp' -type f)

echo "Building native compiler..."
"$CXX" -std=c++17 -O1 -I"$ROOT/compiler/src" \
    "${SOURCES[@]}" -o "$OUT/ulang++"

echo "Building kernel..."
"$OUT/ulang++" build --target freestanding-elf \
    -o "$OUT/kernel.elf" "$ROOT/tests/boot/kernel_smoke.upp"

# QEMU's -kernel needs the std/ tree next to the compiler for `use std.*`.
ln -sfn "$ROOT/std" "$OUT/std"

echo "Booting kernel in QEMU..."
SERIAL="$OUT/serial.log"
QEMU="${QEMU:-qemu-system-x86_64}"
# Run from OUT and use a relative serial path: native QEMU on Windows/MSYS
# cannot resolve POSIX-style /tmp paths.
(
    cd "$OUT"
    timeout 15s "$QEMU" -kernel kernel.elf -display none \
        -serial file:serial.log -monitor none -no-reboot || true
)

if ! grep -q 'ULANG_BOOT_OK' "$SERIAL"; then
    echo "FAIL: kernel did not print ULANG_BOOT_OK"
    cat "$SERIAL" 2>/dev/null || true
    exit 1
fi
if ! grep -q 'IDT_PIC_PIT_CPUID_MSR_OK' "$SERIAL"; then
    echo "FAIL: kernel did not reach the IDT/PIC/PIT/CPUID/MSR marker"
    cat "$SERIAL" 2>/dev/null || true
    exit 1
fi

echo "PASS: freestanding boot, modules, intrinsics and println work"

