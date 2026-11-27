# ULang++ UEFI Terminal OS

A minimal UEFI application (PE32+, `Subsystem=10`) that works as an echo
terminal: it prints a banner through the firmware console and forwards
keystrokes from `EFI_SIMPLE_TEXT_INPUT_PROTOCOL` back to
`EFI_SIMPLE_TEXT_OUTPUT_PROTOCOL`.

No libc, no CRT, no UEFI headers. The source (`os/main.upp`) talks to raw
protocol pointers with `mem.read_u64` / `mem.write_u16` and calls firmware
functions via function pointers.

## Build

```bash
bash os/build.sh
```

Output: `os/esp/EFI/BOOT/BOOTX64.EFI` (UEFI fallback boot path).

## Boot (QEMU)

`OVMF` is not bundled. Two options:

```bash
# single combined firmware image
OVMF_FD=/path/to/OVMF.fd bash os/run.sh

# split Code+Vars firmware (as shipped by QEMU on MSYS2)
OVMF_CODE=/ucrt64/share/qemu/edk2-x86_64-code.fd \
OVMF_VARS=/path/to/vars.fd bash os/run.sh
```

The run script defaults to the split QEMU firmware when `OVMF_FD` is unset.

## What you should see

After firmware boot messages: `ULANG UEFI TYPE`, then each typed character is
echoed. Enter emits CRLF, Backspace erases the previous character.

## Notes

- This is a UEFI application, not a kernel that owns the machine: no own
  scheduler, interrupts, paging or filesystem.
- Only the firmware text console is used.
- Verified in QEMU with QEMU's prebuilt `edk2-x86_64-code.fd`; a combined
  OVMF.fd was not bundled with the toolchain.
