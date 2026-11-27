# ULang++ UEFI Terminal OS

A minimal "OS" built as a **UEFI application** (PE32+, `Subsystem=10`) that uses
the firmware's console protocols instead of writing its own drivers. It is a
terminal/shell: text output via `EFI_SIMPLE_TEXT_OUTPUT_PROTOCOL`, keyboard input
via `EFI_SIMPLE_TEXT_INPUT_PROTOCOL`.

## What works

- Prints a banner through `ConOut -> OutputString(this, CHAR16*)`.
- Reads keystrokes through `ConIn -> ReadKeyStroke(this, EFI_INPUT_KEY*)`.
- Echoes printable characters, handles Backspace and Enter.
- On Enter, recognises `help` and `clear`; otherwise echoes the typed line back.
- `clear` calls `ConOut -> ClearScreen(this)`.

The source is `os/main.upp`. It talks to raw addresses with `mem.read_u64`,
`mem.write_u16`, `mem.read_u16`, `mem.write_u8`, and calls protocol function
pointers with `call_ptr` — no libc, no CRT, no UEFI headers.

## Build

```bash
bash os/build.sh
```

Output: `os/esp/EFI/BOOT/BOOTX64.EFI` (the standard UEFI fallback boot path).

## Boot (QEMU + OVMF)

OVMF firmware is **not bundled**. Set `OVMF_FD` to your firmware image:

```bash
OVMF_FD=/usr/share/ovmf/OVMF.fd bash os/run.sh
```

On Windows/MSYS2 the firmware usually lives at
`/ucrt64/share/qemu/ovmf-x64/OVMF.fd` or similar.

## Compiler support

`ulang++ build --target efi-app -o kernel.efi main.upp` emits a PE32+ image with:

- no imports / no KERNEL32 dependencies / no relocation table;
- `.text` / `.data` / `.bss` (1 MiB heap area);
- RIP-relative access to globals and the exception/heap slots;
- entry stub that forwards `ImageHandle` (RCX) and `SystemTable` (RDX) to
  `efi_main(image: int, systable: int) -> int`.

## Limitations

- This is a UEFI application, not a kernel that takes over the machine: there
  is no own scheduler, paging, interrupt handling or filesystem driver.
- Only the firmware's basic text console is used (no Graphics Output Protocol).
- Requires a UEFI-capable firmware; legacy BIOS / Multiboot boot is the
  separate `--target freestanding-elf` path.
- OVMF was not available in the development environment, so the image was
  verified structurally (PE headers/sections) but not boot-tested here.
