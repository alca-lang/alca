#!/usr/bin/env bash
# Boot the UEFI terminal OS in QEMU with OVMF firmware.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ESP="$ROOT/os/esp"

if [[ -z "${OVMF_FD:-}" ]]; then
    echo "Set OVMF_FD to an OVMF firmware image (e.g. /usr/share/ovmf/OVMF.fd)." >&2
    echo "OVMF is not bundled with this repository." >&2
    exit 1
fi
if [[ ! -f "$ESP/EFI/BOOT/BOOTX64.EFI" ]]; then
    echo "Run os/build.sh first." >&2
    exit 1
fi

exec qemu-system-x86_64 \
    -machine q35 \
    -bios "$OVMF_FD" \
    -drive "format=raw,file=fat:rw:$ESP" \
    -net none \
    -nographic \
    "$@"
