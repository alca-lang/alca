$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force build | Out-Null

$cxx = if ($env:CXX) { $env:CXX } else { "g++" }
$flags = @("-std=c++17", "-O2", "-Wall", "-Wextra", "-Werror")
$includes = @("-Isrc", "-Ithird_party")
$core = @(
  "src/token/token.cpp",
  "src/lexer/lexer.cpp",
  "src/diag/diagnostic.cpp",
  "src/ast/ast.cpp",
  "src/parser/parser.cpp",
  "src/dunder/dunder.cpp",
  "src/mod/mod.cpp",
  "src/typecheck/builtins_table.cpp",
  "src/typecheck/types.cpp",
  "src/typecheck/typecheck.cpp",
  "src/ir/ir.cpp",
  "src/ir/lower.cpp",
  "src/codegen/x64/codegen.cpp",
  "src/link/pe/pe_writer.cpp",
  "src/link/elf/elf_writer.cpp"
)

& $cxx @flags @includes -o build/pawl.exe @core src/cli/main.cpp
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& $cxx @flags @includes -o build/pawl_tests.exe @core tests/test_main.cpp tests/test_token.cpp tests/test_lexer.cpp tests/test_ast.cpp tests/test_parser.cpp tests/test_typecheck.cpp tests/test_ir.cpp tests/test_codegen.cpp tests/test_differential.cpp
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[OK] compiler/build/pawl.exe" -ForegroundColor Green
Write-Host "[OK] compiler/build/pawl_tests.exe" -ForegroundColor Green
