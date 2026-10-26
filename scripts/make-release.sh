#!/usr/bin/env bash

# Copyright 2026 Alca Programming Language
# SPDX-License-Identifier: Apache-2.0

# Builds the releases/ bundle on Linux/macOS hosts.
#
# Usage: scripts/make-release.sh
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
rel="$root/releases"
bin="$rel/bin"

ldflags="-s -w"

build_host() { # os arch suffix
  local os="$1" arch="$2" suffix="$3"
  local dir="$bin/$os-$arch"
  mkdir -p "$dir"
  for c in alca; do
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "$ldflags" -o "$dir/$c$suffix" "./cmd/$c"
    echo "  $dir/$c$suffix"
  done
}

echo "Building binaries..."
build_host windows amd64 .exe
build_host linux amd64 ""
build_host linux arm64 ""

echo "Copying cross-compilation runtimes..."
for h in windows-amd64 linux-amd64 linux-arm64; do
  dir="$bin/$h/runtimes"
  mkdir -p "$dir"
  for r in windows-amd64 linux-amd64 linux-arm64; do
    [ "$r" = "$h" ] && continue
    name="alca-$r"
    [ "${r%%-*}" = windows ] && name="$name.exe"
    if [ -f "runtimes/$name" ]; then
      cp -f "runtimes/$name" "$dir/$name"
    else
      echo "warning: runtime missing: runtimes/$name (run: go run ./cmd/alca build-runtimes --out runtimes)" >&2
    fi
  done
done

echo "Copying documentation and examples..."
mkdir -p "$rel/docs" "$rel/examples"
cp -f docs/*.md "$rel/docs/" 2>/dev/null || true
cp -f examples/*.al "$rel/examples/" 2>/dev/null || true

echo "Release bundle is in $rel"
