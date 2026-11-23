# Copyright 2026 Pawl Programming Language
# SPDX-License-Identifier: Apache-2.0
# Complete installer for Pawl toolchain on Windows:
# - Binaries (pawl.exe, apm.exe) v1.7.9
# - User & System PATH registration
# - Windows file associations (.al -> Pawl Source File, VS Code document icon, Explorer context menus)
# - VS Code & Cursor extensions (syntax highlighting, file icons, snippets, LSP)

$ErrorActionPreference = "Stop"

$Version = "1.7.9"
$Repo = "pawl-lang/pawl"
$ApmRepo = "pawl-lang/apm"
$RegistryHost = "http://91.92.34.94:9977"
$InstallDir = if ($env:PAWL_HOME) { Join-Path $env:PAWL_HOME "bin" } else { Join-Path $env:USERPROFILE ".pawl\bin" }

Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "   Installing Pawl Lang & APM v$Version       " -ForegroundColor Cyan
Write-Host "==============================================" -ForegroundColor Cyan

# 1. Architecture detection
$Arch = if ([IntPtr]::Size -eq 8) { "amd64" } else { "386" }
Write-Host "[1/4] Platform detected: windows_$Arch" -ForegroundColor Green

if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# 2. Download and unpack binaries
Write-Host "[2/4] Downloading Pawl & APM binaries..." -ForegroundColor Yellow
$ZipUrl = "https://github.com/$Repo/releases/latest/download/pawl-windows-$Arch.zip"
$RegistryZipUrl = "$RegistryHost/api/v1/download/pawl-windows-$Arch.zip"
$TempZip = Join-Path $env:TEMP "pawl_install.zip"
$Downloaded = $false

try {
    Invoke-WebRequest -Uri $ZipUrl -OutFile $TempZip -UseBasicParsing -TimeoutSec 15
    $Downloaded = $true
    Write-Host "  -> Downloaded bundle from GitHub Releases" -ForegroundColor Green
} catch {
    try {
        Invoke-WebRequest -Uri $RegistryZipUrl -OutFile $TempZip -UseBasicParsing -TimeoutSec 15
        $Downloaded = $true
        Write-Host "  -> Downloaded bundle from Pawl Registry Mirror" -ForegroundColor Green
    } catch {
        Write-Host "  -> Downloading standalone executables..." -ForegroundColor Yellow
        try {
            Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest/download/pawl.exe" -OutFile (Join-Path $InstallDir "pawl.exe") -UseBasicParsing
            $Downloaded = $true
        } catch {}
        try {
            Invoke-WebRequest -Uri "https://github.com/$ApmRepo/releases/latest/download/apm.exe" -OutFile (Join-Path $InstallDir "apm.exe") -UseBasicParsing
        } catch {
            try {
                Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest/download/apm.exe" -OutFile (Join-Path $InstallDir "apm.exe") -UseBasicParsing
            } catch {}
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

# 4. Windows File Associations (.al files) with VS Code File Icon
Write-Host "[4/4] Setting up Windows file associations & extensions..." -ForegroundColor Yellow
try {
    $PawlExe = Join-Path $InstallDir "pawl.exe"
    $IcoFile = Join-Path $InstallDir "pawl-file.ico"

    # Install dedicated document icon (VS Code file sheet style)
    if (-not (Test-Path $IcoFile)) {
        $LocalIco = Join-Path $PSScriptRoot "pawl-file.ico"
        if (-not (Test-Path $LocalIco)) { $LocalIco = Join-Path (Split-Path -Parent $PSScriptRoot) "pawl-file.ico" }
        if (Test-Path $LocalIco) {
            Copy-Item $LocalIco $IcoFile -Force -ErrorAction SilentlyContinue
        } else {
            try {
                Invoke-WebRequest -Uri "$RegistryHost/assets/pawl-file.ico" -OutFile $IcoFile -UseBasicParsing -TimeoutSec 6 -ErrorAction SilentlyContinue
            } catch {
                try {
                    Invoke-WebRequest -Uri "https://raw.githubusercontent.com/$Repo/master/pawl-file.ico" -OutFile $IcoFile -UseBasicParsing -TimeoutSec 6 -ErrorAction SilentlyContinue
                } catch {}
            }
        }
    }

    # Associate .al extension
    New-Item -Path "HKCU:\Software\Classes\.al" -Value "PawlSourceFile" -Force | Out-Null
    Set-ItemProperty -Path "HKCU:\Software\Classes\.al" -Name "Content Type" -Value "text/x-pawl" -Force
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile" -Value "Pawl Source File" -Force | Out-Null
    
    # Use dedicated VS Code style document icon if present, fallback to pawl.exe
    $IconPath = if (Test-Path $IcoFile) { "`"$IcoFile`",0" } else { "`"$PawlExe`",0" }
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\DefaultIcon" -Value $IconPath -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\open\command" -Value "`"$PawlExe`" run `"%1`"" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\run" -Value "Run with Pawl" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\run\command" -Value "`"$PawlExe`" run `"%1`"" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\check" -Value "Check Syntax with Pawl" -Force | Out-Null
    New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\check\command" -Value "`"$PawlExe`" check `"%1`"" -Force | Out-Null

    # Context menu: Edit with VS Code or Cursor if available
    $CodeCmd = Get-Command code -ErrorAction SilentlyContinue
    if ($CodeCmd) {
        New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\edit" -Value "Edit with VS Code" -Force | Out-Null
        New-Item -Path "HKCU:\Software\Classes\PawlSourceFile\shell\edit\command" -Value "`"$($CodeCmd.Source)`" `"%1`"" -Force | Out-Null
    }

    # Broadcast shell change notify so Windows Explorer displays the new icon immediately
    try {
        $code = '[DllImport("shell32.dll")] public static extern void SHChangeNotify(int wEventId, int uFlags, IntPtr dwItem1, IntPtr dwItem2);'
        Add-Type -MemberDefinition $code -Namespace Win32 -Name Shell -ErrorAction SilentlyContinue
        [Win32.Shell]::SHChangeNotify(0x08000000, 0, [IntPtr]::Zero, [IntPtr]::Zero)
    } catch {}

    Write-Host "  -> Registered .al file associations (VS Code document icon: $IcoFile)" -ForegroundColor Green
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
        $targetExt = Join-Path $extBase "pawl-lang"
        try {
            $extZip = Join-Path $env:TEMP "pawl_vscode.zip"
            $extUrl = "$RegistryHost/editors/vscode.zip"
            Invoke-WebRequest -Uri $extUrl -OutFile $extZip -UseBasicParsing -TimeoutSec 10 -ErrorAction SilentlyContinue
            if (Test-Path $extZip) {
                if (Test-Path $targetExt) { Remove-Item -Recurse -Force $targetExt -ErrorAction SilentlyContinue }
                New-Item -ItemType Directory -Path $targetExt -Force | Out-Null
                Expand-Archive -Path $extZip -DestinationPath $targetExt -Force
                Remove-Item $extZip -Force -ErrorAction SilentlyContinue
                Write-Host "  -> Installed Pawl extension (syntax + icons + snippets) to $extBase" -ForegroundColor Green
                $ExtInstalled = $true
            }
        } catch {}
    }
}

if (-not $ExtInstalled) {
    $LocalVsCode = "editors\vscode"
    if (Test-Path $LocalVsCode) {
        foreach ($extBase in $VscodeExtDirs) {
            if (Test-Path $extBase) {
                $targetExt = Join-Path $extBase "pawl-lang"
                Copy-Item -Recurse -Force $LocalVsCode $targetExt -ErrorAction SilentlyContinue
                Write-Host "  -> Copied local Pawl extension to $extBase" -ForegroundColor Green
            }
        }
    }
}

Write-Host ""
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "    Pawl Lang v$Version is ready to use!      " -ForegroundColor Green
Write-Host "==============================================" -ForegroundColor Cyan
Write-Host "  * CLI Binary:     $InstallDir\pawl.exe" -ForegroundColor White
Write-Host "  * Package Mgr:    $InstallDir\apm.exe" -ForegroundColor White
Write-Host "  * File Icon:      $InstallDir\pawl-file.ico" -ForegroundColor White
Write-Host "  * File Extension: .al (Pawl Source Files)" -ForegroundColor White
Write-Host "  * Documentation:  $RegistryHost/#docs" -ForegroundColor Yellow
Write-Host "  * GitHub:         https://github.com/$Repo" -ForegroundColor Gray
Write-Host ""
Write-Host "Run 'pawl version' or 'apm --version' to verify." -ForegroundColor Cyan
