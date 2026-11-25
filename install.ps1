<#
.SYNOPSIS
    ULang++ Programming Language Installer for Windows.
.DESCRIPTION
    Installs the ULang++ compiler (ulang++.exe), sets up standard directories including
    ProgramData\ULangLibs\libs (and legacy ProgramData\ULangLibs\libs) for custom packages and libraries,
    and adds ULang++ to the user's system PATH.
#>

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\ULang++",
    [string]$LibsDir = "$env:ProgramData\ULangLibs\libs",
    [switch]$AddToPath = $true,
    [switch]$Uninstall = $false
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "   ULang++ Language Toolchain Installer   " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if ($Uninstall) {
    Write-Host "[*] Uninstalling ULang++..." -ForegroundColor Yellow
    if (Test-Path $InstallDir) {
        Remove-Item -Recurse -Force $InstallDir
        Write-Host "[+] Removed $InstallDir" -ForegroundColor Green
    }
    Write-Host "[+] Uninstallation complete." -ForegroundColor Green
    exit 0
}

# 1. Create ProgramData\ULangLibs\libs directory for external & custom modules
Write-Host "[*] Creating custom libraries directory: $LibsDir" -ForegroundColor Cyan
if (!(Test-Path $LibsDir)) {
    New-Item -ItemType Directory -Force -Path $LibsDir | Out-Null
    Write-Host "[+] Created $LibsDir" -ForegroundColor Green
} else {
    Write-Host "[=] $LibsDir already exists." -ForegroundColor Gray
}

# 2. Setup install directory for compiler binary and stdlib
Write-Host "[*] Preparing install directory: $InstallDir" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path "$InstallDir\bin" | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallDir\std" | Out-Null

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Copy compiler binary (ulang++.exe and ulang++.exe compatibility symlink/copy)
$compilerExe = "$scriptDir\compiler\build_bin\ulang++.exe"
if (!(Test-Path $compilerExe)) {
    $compilerExe = "$scriptDir\compiler\build_bin\ulang++.exe"
}

if (Test-Path $compilerExe) {
    Copy-Item -Force $compilerExe "$InstallDir\bin\ulang++.exe"
    Copy-Item -Force $compilerExe "$InstallDir\bin\ulang++.exe"
    Write-Host "[+] Installed compiler -> $InstallDir\bin\ulang++.exe" -ForegroundColor Green
} else {
    Write-Host "[!] Note: compiler executable not found in build_bin. Build it first." -ForegroundColor Yellow
}

# Copy standard library files
$stdDir = "$scriptDir\std"
if (Test-Path $stdDir) {
    Copy-Item -Recurse -Force "$stdDir\*" "$InstallDir\std\"
    Write-Host "[+] Installed stdlib to $InstallDir\std\" -ForegroundColor Green
}

# 3. Add to User PATH if requested
if ($AddToPath) {
    $binPath = "$InstallDir\bin"
    $userPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
    if ($userPath -notlike "*$binPath*") {
        $newPath = "$userPath;$binPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::User)
        Write-Host "[+] Added $binPath to User PATH environment variable." -ForegroundColor Green
    } else {
        Write-Host "[=] PATH already contains $binPath" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "    Installation Finished Successfully!   " -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host "Compiler location: $InstallDir\bin\ulang++.exe"
Write-Host "Custom libs path:  $LibsDir"
Write-Host ""
Write-Host "You can now run:"
Write-Host "  ulang++ build main.al"
Write-Host "  ulang++ run main.al"
Write-Host ""
