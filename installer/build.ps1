# Build the ULang++ 2.4 Windows installer.
$ErrorActionPreference = "Stop"

$installerDir = $PSScriptRoot
$repo = Split-Path -Parent $installerDir
Set-Location $repo

# 1. Turn the PNG file icon into a real .ico (PNG-compressed icon entry).
$png = Join-Path $repo "ULang++_FILE-ICO.png"
$ico = Join-Path $repo "ULang++_FILE-ICO.ico"
$data = [IO.File]::ReadAllBytes($png)
$ms = New-Object IO.MemoryStream
$w = New-Object IO.BinaryWriter($ms)
$w.Write([uint16]0)      # reserved
$w.Write([uint16]1)      # type: icon
$w.Write([uint16]1)      # one image
$w.Write([byte]0)        # width 0 = 256
$w.Write([byte]0)        # height 0 = 256
$w.Write([byte]0)        # palette
$w.Write([byte]0)        # reserved
$w.Write([uint16]1)      # planes
$w.Write([uint16]32)     # bits per pixel
$w.Write([uint32]$data.Length)
$w.Write([uint32]22)     # image offset
$w.Write($data)
$w.Flush()
[IO.File]::WriteAllBytes($ico, $ms.ToArray())
$w.Dispose(); $ms.Dispose()
Write-Host "[+] $ico" -ForegroundColor Green

# 2. Compile the WinForms installer with the logo embedded.
$csc = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe"
if (-not (Test-Path $csc)) { $csc = "C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe" }
if (-not (Test-Path $csc)) { throw "csc.exe not found (.NET Framework required)" }

$manifest = Join-Path $installerDir "app.manifest"
$logo = Join-Path $repo "ULang++_Logo.png"
$source = Join-Path $installerDir "ULangSetup.cs"
$output = Join-Path $repo "ULangSetup.exe"
$resource = "$logo,ULangLogo"

& $csc /nologo /target:winexe /optimize+ `
    "/win32manifest:$manifest" `
    "/resource:$resource" `
    "/out:$output" `
    $source
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[+] $output" -ForegroundColor Green
Write-Host "Run ULangSetup.exe as administrator." -ForegroundColor Cyan
