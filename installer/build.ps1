# Build the self-contained ULang++ 2.4 Windows installer.
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
$w.Write([uint16]0); $w.Write([uint16]1); $w.Write([uint16]1)
$w.Write([byte]0); $w.Write([byte]0); $w.Write([byte]0); $w.Write([byte]0)
$w.Write([uint16]1); $w.Write([uint16]32)
$w.Write([uint32]$data.Length); $w.Write([uint32]22)
$w.Write($data); $w.Flush()
[IO.File]::WriteAllBytes($ico, $ms.ToArray())
$w.Dispose(); $ms.Dispose()

# 2. Assemble the payload: compiler, runtime, stdlib, docs and file icon.
$payloadDir = Join-Path $installerDir "payload"
if (Test-Path $payloadDir) { Remove-Item -Recurse -Force $payloadDir }
New-Item -ItemType Directory -Force $payloadDir | Out-Null

$resultDir = Join-Path $repo "result"
if (Test-Path (Join-Path $resultDir "ulang++.exe")) {
    Copy-Item (Join-Path $resultDir "ulang++.exe") (Join-Path $payloadDir "ulang++.exe")
    Copy-Item (Join-Path $resultDir "ulang++.exe") (Join-Path $payloadDir "u++.exe")
    Copy-Item (Join-Path $resultDir "ulang++.exe") (Join-Path $payloadDir "upp.exe")
}
Get-ChildItem $resultDir -Filter "*.dll" -File -ErrorAction SilentlyContinue | ForEach-Object {
    Copy-Item $_.FullName (Join-Path $payloadDir $_.Name)
}
$stdSrc = Join-Path $repo "std"
if (Test-Path $stdSrc) { Copy-Item -Recurse -Force $stdSrc (Join-Path $payloadDir "std") }
foreach ($name in @("LICENSE", "NOTICE", "README.md", "CHANGELOG.md")) {
    $f = Join-Path $repo $name
    if (Test-Path $f) { Copy-Item $f (Join-Path $payloadDir $name) }
}
Copy-Item $ico (Join-Path $payloadDir "ULang++_FILE-ICO.ico")

$zip = Join-Path $installerDir "payload.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $payloadDir "*") -DestinationPath $zip -CompressionLevel Optimal
Write-Host "[+] $zip" -ForegroundColor Green

# 3. Compile the WinForms installer with the logo and payload embedded.
$csc = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe"
if (-not (Test-Path $csc)) { $csc = "C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe" }
if (-not (Test-Path $csc)) { throw "csc.exe not found (.NET Framework required)" }

$manifest = Join-Path $installerDir "app.manifest"
$logo = Join-Path $repo "ULang++_Logo.png"
$fileIcon = Join-Path $repo "ULang++_FILE-ICO.png"
$source = Join-Path $installerDir "ULangSetup.cs"
$output = Join-Path $repo "ULangSetup.exe"

& $csc /nologo /target:winexe /optimize+ `
    "/r:System.IO.Compression.dll" "/r:System.IO.Compression.FileSystem.dll" `
    "/win32manifest:$manifest" `
    "/resource:$logo,ULangLogo" `
    "/resource:$fileIcon,FileIcon" `
    "/resource:$zip,Payload" `
    "/out:$output" `
    $source
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[+] $output" -ForegroundColor Green
Write-Host "Run ULangSetup.exe as administrator." -ForegroundColor Cyan
