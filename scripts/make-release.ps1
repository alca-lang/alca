# Copyright 2026 Alca Programming Language
# SPDX-License-Identifier: Apache-2.0

# Builds the releases/ bundle: static binaries for Windows and Linux hosts,
# cross-compilation runtimes, documentation and examples.
#
# Usage: powershell -ExecutionPolicy Bypass -File scripts/make-release.ps1 [-Registry <url>]
#
# -Registry stamps a default registry node URL into the apm binaries (also
# taken from $ALCA_REGISTRY when the parameter is omitted), so released
# clients contact that node without --registry or environment setup.
param(
    [string]$Registry = "http://91.92.34.94:9977"
)
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$ldflags = "-s -w"

$rel = Join-Path $root "releases"
$bin = Join-Path $rel "bin"
$allPlatforms = @(
    @{ os = "linux";   arch = "amd64"; suffix = "" },
    @{ os = "linux";   arch = "arm64"; suffix = "" },
    @{ os = "darwin";  arch = "amd64"; suffix = "" },
    @{ os = "darwin";  arch = "arm64"; suffix = "" },
    @{ os = "windows"; arch = "amd64"; suffix = ".exe" },
    @{ os = "windows"; arch = "arm64"; suffix = ".exe" }
)
$commands = @("alca")

Write-Host "Building releases/ binaries for all 6 platforms..."
$env:CGO_ENABLED = "0"
foreach ($h in $allPlatforms) {
    $env:GOOS = $h.os
    $env:GOARCH = $h.arch
    foreach ($c in $commands) {
        $flatOut = Join-Path $rel "$c-$($h.os)-$($h.arch)$($h.suffix)"
        go build -trimpath -ldflags $ldflags -o $flatOut "./cmd/$c"
        if ($LASTEXITCODE -ne 0) { throw "build failed: $c $($h.os)/$($h.arch)" }
        Write-Host "  $flatOut"
    }
}

Write-Host "Building bundle bin/ directories..."
$bundleHosts = @(
    @{ os = "windows"; arch = "amd64"; suffix = ".exe" },
    @{ os = "linux";   arch = "amd64"; suffix = "" },
    @{ os = "linux";   arch = "arm64"; suffix = "" }
)
foreach ($h in $bundleHosts) {
    $dir = Join-Path $bin "$($h.os)-$($h.arch)"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    foreach ($c in $commands) {
        $flatOut = Join-Path $rel "$c-$($h.os)-$($h.arch)$($h.suffix)"
        $out = Join-Path $dir "$c$($h.suffix)"
        Copy-Item $flatOut $out -Force
    }
}
Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue

# Also update workspace root binaries
Copy-Item (Join-Path $rel "alca-windows-amd64.exe") (Join-Path $root "alca.exe") -Force

Write-Host "Copying cross-compilation runtimes..."
foreach ($h in $bundleHosts) {
    $dir = Join-Path $bin "$($h.os)-$($h.arch)\runtimes"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    foreach ($r in $bundleHosts) {
        if ($r.os -eq $h.os -and $r.arch -eq $h.arch) { continue }
        $name = "alca-$($r.os)-$($r.arch)"
        if ($r.os -eq "windows") { $name += ".exe" }
        $src = Join-Path $root "runtimes\$name"
        if (Test-Path $src) {
            Copy-Item $src (Join-Path $dir $name) -Force
        } else {
            Write-Warning "runtime missing: $src (run: go run ./cmd/alca build-runtimes --out runtimes)"
        }
    }
}

Write-Host "Copying documentation and examples..."
$docs = Join-Path $rel "docs"
New-Item -ItemType Directory -Force -Path $docs | Out-Null
Get-ChildItem (Join-Path $root "docs\*.md") | Copy-Item -Destination $docs -Force
$examples = Join-Path $rel "examples"
New-Item -ItemType Directory -Force -Path $examples | Out-Null
Get-ChildItem (Join-Path $root "examples\*.al") | Copy-Item -Destination $examples -Force

Write-Host "Release bundle is in $rel"
