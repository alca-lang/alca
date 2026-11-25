$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
if (!(Test-Path "build/pawlc.exe")) { & ../build.ps1 }
New-Item -ItemType Directory -Force out | Out-Null

& build/pawlc.exe examples/hello.al -o out/hello.exe --target windows
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$got = & out/hello.exe
$expected = @("hello from native Pawl", "55", "55", "-42")
if (Compare-Object $got $expected) {
    Write-Host "FAIL: unexpected output" -ForegroundColor Red
    $got | ForEach-Object { Write-Host "  $_" }
    exit 1
}
Write-Host "[OK] windows PE smoke test: $($got -join ' | ')" -ForegroundColor Green

& build/pawlc.exe examples/hello.al -o out/hello.elf --target linux
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$magic = [System.IO.File]::ReadAllBytes("out/hello.elf")[0..3]
if ($magic[0] -ne 0x7F -or $magic[1] -ne 0x45 -or $magic[2] -ne 0x4C -or $magic[3] -ne 0x46) {
    Write-Host "FAIL: not an ELF file" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] linux ELF emitted (header verified)" -ForegroundColor Green
