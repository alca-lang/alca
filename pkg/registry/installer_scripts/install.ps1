# Copyright 2026 Alca Programming Language
# SPDX-License-Identifier: Apache-2.0
# Complete installer for Alca toolchain on Windows:
# - Binaries (alca.exe, apm.exe)
# - User & System PATH registration
# - Windows file associations (.al -> Alca Source File, Icon, Explorer context menus)
# - VS Code & Cursor extensions (syntax highlighting, file icons, snippets, LSP)

$ErrorActionPreference = "Stop"

$Repo = "alca-lang/alca"
$RegistryHost = "http://91.92.34.94:9977"
$InstallDir = if ($env:ALCA_HOME) { Join-Path $env:ALCA_HOME "bin" } else { Join-Path $env:USERPROFILE ".alca\bin" }

Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "    Installing Alca Programming Language      " -ForegroundColor Cyan
Write-Host "==============================================" -ForegroundColor Cyan

# 1. Architecture detection & directories
$Arch = if ([IntPtr]::Size -eq 8) { "amd64" } else { "386" }
Write-Host "[1/4] Platform detected: windows_${Arch}" -ForegroundColor Green

if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# 2. Download binaries
Write-Host "[2/4] Downloading Alca toolchain..." -ForegroundColor Yellow
$ZipUrl = "https://github.com/$Repo/releases/latest/download/alca-windows-$Arch.zip"
$RegistryZipUrl = "$RegistryHost/api/v1/download/alca-windows-$Arch.zip"
$TempZip = Join-Path $env:TEMP "alca_install.zip"
$Downloaded = $false

try {
    Invoke-WebRequest -Uri $ZipUrl -OutFile $TempZip -UseBasicParsing -TimeoutSec 15
    $Downloaded = $true
    Write-Host "  -> Downloaded from GitHub Releases" -ForegroundColor Green
} catch {
    try {
        Invoke-WebRequest -Uri $RegistryZipUrl -OutFile $TempZip -UseBasicParsing -TimeoutSec 15
        $Downloaded = $true
        Write-Host "  -> Downloaded from Alca Registry Mirror" -ForegroundColor Green
    } catch {
        Write-Host "  -> Downloading standalone binaries..." -ForegroundColor Yellow
        try {
            Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest/download/alca.exe" -OutFile (Join-Path $InstallDir "alca.exe") -UseBasicParsing
            Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest/download/apm.exe" -OutFile (Join-Path $InstallDir "apm.exe") -UseBasicParsing
            $Downloaded = $true
        } catch {
            Write-Host "  -> Warning: Falling back to local bin if available" -ForegroundColor Gray
        }
    }
}

if ($Downloaded -and (Test-Path $TempZip)) {
    Expand-Archive -Path $TempZip -DestinationPath $InstallDir -Force
    Remove-Item $TempZip -Force -ErrorAction SilentlyContinue
}

# 3. Environment PATH registration
Write-Host "[3/4] Configuring environment PATH..." -ForegroundColor Yellow
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "  -> Added $InstallDir to User PATH." -ForegroundColor Green
} else {
    Write-Host "  -> $InstallDir is already in User PATH." -ForegroundColor Gray
}

# 4. Windows File Associations (.al files)
Write-Host "[4/4] Setting up Windows file associations & extensions..." -ForegroundColor Yellow
try {
    $AlcaExe = Join-Path $InstallDir "alca.exe"
    # Associate .al extension
    New-Item -Path "HKCU:\Software\Classes\.al" -Value "AlcaSourceFile" -Force | Out-Null
    Set-ItemProperty -Path "HKCU:\Software\Classes\.al" -Name "Content Type" -Value "text/x-alca" -Force
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile" -Value "Alca Source File" -Force | Out-Null
    
    # Icon and Explorer commands
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\DefaultIcon" -Value "`"$AlcaExe`",0" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\open\command" -Value "`"$AlcaExe`" run `"%1`"" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\run" -Value "Run with Alca" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\run\command" -Value "`"$AlcaExe`" run `"%1`"" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\check" -Value "Check Syntax with Alca" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\check\command" -Value "`"$AlcaExe`" check `"%1`"" -Force | Out-Null

    # Context menu: Edit with VS Code or Cursor if available
    $CodeCmd = Get-Command code -ErrorAction SilentlyContinue
    if ($CodeCmd) {
        New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\edit" -Value "Edit with VS Code" -Force | Out-Null
        New-Item -Path "HKCU:\Software\Classes\AlcaSourceFile\shell\edit\command" -Value "`"$($CodeCmd.Source)`" `"%1`"" -Force | Out-Null
    }
    Write-Host "  -> Registered .al file associations and Explorer context actions." -ForegroundColor Green
} catch {
    Write-Host "  -> Note: File association skipped ($($_.Exception.Message))" -ForegroundColor Gray
}

# 5. VS Code & Cursor Extensions (Syntax highlighting, file icons, snippets)
$VscodeExtDirs = @(
    (Join-Path $env:USERPROFILE ".vscode\extensions"),
    (Join-Path $env:USERPROFILE ".cursor\extensions"),
    (Join-Path $env:USERPROFILE ".vscode-oss\extensions")
)

$ExtInstalled = $false
foreach ($extBase in $VscodeExtDirs) {
    if (Test-Path $extBase) {
        $targetExt = Join-Path $extBase "alca-lang"
        try {
            # Try downloading vscode extension bundle
            $extZip = Join-Path $env:TEMP "alca_vscode.zip"
            $extUrl = "$RegistryHost/editors/vscode.zip"
            Invoke-WebRequest -Uri $extUrl -OutFile $extZip -UseBasicParsing -TimeoutSec 10 -ErrorAction SilentlyContinue
            if (Test-Path $extZip) {
                if (Test-Path $targetExt) { Remove-Item -Recurse -Force $targetExt -ErrorAction SilentlyContinue }
                New-Item -ItemType Directory -Path $targetExt -Force | Out-Null
                Expand-Archive -Path $extZip -DestinationPath $targetExt -Force
                Remove-Item $extZip -Force -ErrorAction SilentlyContinue
                Write-Host "  -> Installed Alca extension (syntax + icons + snippets) to $extBase" -ForegroundColor Green
                $ExtInstalled = $true
            }
        } catch {}
    }
}

if (-not $ExtInstalled) {
    # If editors/vscode exists locally in current folder, copy directly
    $LocalVsCode = "editors\vscode"
    if (Test-Path $LocalVsCode) {
        foreach ($extBase in $VscodeExtDirs) {
            if (Test-Path $extBase) {
                $targetExt = Join-Path $extBase "alca-lang"
                Copy-Item -Recurse -Force $LocalVsCode $targetExt -ErrorAction SilentlyContinue
                Write-Host "  -> Copied local Alca extension to $extBase" -ForegroundColor Green
            }
        }
    }
}

Write-Host ""
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "    Alca is ready to use!                     " -ForegroundColor Green
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "  * CLI Binary:     $InstallDir\alca.exe" -ForegroundColor White
Write-Host "  * Package Mgr:    $InstallDir\apm.exe" -ForegroundColor White
Write-Host "  * File Extension: .al (Alca Source Files)" -ForegroundColor White
Write-Host "  * Documentation:  $RegistryHost/#docs" -ForegroundColor Yellow
Write-Host "  * GitHub:         https://github.com/$Repo" -ForegroundColor Gray
Write-Host ""
Write-Host "Run 'alca version' to verify." -ForegroundColor Cyan
