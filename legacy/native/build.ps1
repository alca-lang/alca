$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force build | Out-Null
$src = @("src/front.cpp", "src/codegen.cpp", "src/link.cpp", "src/main.cpp")
& g++ -std=c++17 -O2 -Wall -Wextra -static -o build/pawlc.exe @src
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "[OK] native/build/pawlc.exe" -ForegroundColor Green
