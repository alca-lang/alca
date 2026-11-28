# Rebuild the shared compiler library used by the installer payload.
# The installer itself (ULangSetup.exe) is pre-built and is not modified.
$ErrorActionPreference = "Stop"

$repo = $PSScriptRoot
Set-Location $repo

$cxx = if ($env:CXX) { $env:CXX } else { "g++" }
$flags = @("-std=c++17", "-O2", "-shared")
$includes = @("-Icompiler/src")
$sources = @(
  "compiler/src/libulang.cpp",
  "compiler/src/cli/main.cpp",
  "compiler/src/ast/ast.cpp",
  "compiler/src/codegen/x64/codegen.cpp",
  "compiler/src/diag/diagnostic.cpp",
  "compiler/src/dunder/dunder.cpp",
  "compiler/src/ir/ir.cpp",
  "compiler/src/ir/lower.cpp",
  "compiler/src/lexer/lexer.cpp",
  "compiler/src/link/elf/elf_writer.cpp",
  "compiler/src/link/pe/pe_writer.cpp",
  "compiler/src/link/pe/efi_writer.cpp",
  "compiler/src/mod/mod.cpp",
  "compiler/src/parser/parser.cpp",
  "compiler/src/runtime/runtime.cpp",
  "compiler/src/token/token.cpp",
  "compiler/src/typecheck/builtins_table.cpp",
  "compiler/src/typecheck/typecheck.cpp",
  "compiler/src/typecheck/types.cpp"
)

Write-Host "[+] Rebuilding result/libulang.dll ..." -ForegroundColor Cyan
& $cxx @flags @includes @sources "-Wl,--out-implib,result/libulang.a" -o result/libulang.dll
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[+] Rebuilt:" -ForegroundColor Green
Get-ChildItem result/libulang.dll, result/libulang.a | Select-Object Name, Length, LastWriteTime

if (Test-Path pack_payload.exe) {
  Write-Host "[+] Repacking installer payload ..." -ForegroundColor Cyan
  & ./pack_payload.exe
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
  Write-Host "[!] pack_payload.exe not found - payload not repacked." -ForegroundColor Yellow
}
