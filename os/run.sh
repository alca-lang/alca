#!/usr/bin/env bash
# Boot the UEFI terminal OS in QEMU with OVMF firmware.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ESP="$ROOT/os/esp"
QEMU="${QEMU:-qemu-system-x86_64}"

if [[ ! -f "$ESP/EFI/BOOT/BOOTX64.EFI" ]]; then
    echo "Run os/build.sh first." >&2
    exit 1
fi

common=(-machine q35 -drive "format=raw,file=fat:rw:$ESP" -net none -nographic)

if [[ -n "${OVMF_FD:-}" ]]; then
    exec "$QEMU" "${common[@]}" -bios "$OVMF_FD" "$@"
fi

CODE="${OVMF_CODE:-/ucrt64/share/qemu/edk2-x86_64-code.fd}"
VARS="${OVMF_VARS:-}"
if [[ -z "$VARS" ]]; then
    echo "Set OVMF_FD, or both OVMF_CODE and OVMF_VARS." >&2
    echo "OVMF firmware is not bundled with this repository." >&2
    exit 1
fi

exec "$QEMU" "${common[@]}" \
    -drive "if=pflash,format=raw,unit=0,file=$CODE,readonly=on" \
    -drive "if=pflash,format=raw,unit=1,file=$VARS" "$@"
