#!/usr/bin/env bash
# Builds the releases/ bundle on Linux/macOS hosts.
#
# Usage: scripts/make-release.sh [registry-url]
#
# The optional registry URL is stamped into the apm binaries as their default
# registry node (falls back to $ALCA_REGISTRY when the argument is omitted),
# so released clients contact that node without --registry or environment
# setup.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
rel="$root/releases"
bin="$rel/bin"

registry="${1:-${ALCA_REGISTRY:-}}"
ldflags="-s -w"
if [ -n "$registry" ]; then
  ldflags="$ldflags -X github.com/alca-lang/alca/pkg/apm.DefaultRegistry=$registry"
  echo "Default registry node baked into apm: $registry"
  case "$registry" in
    https://*) ;;
    *) echo "warning: registry URL is not https; clients refuse non-loopback plain http" >&2 ;;
  esac
else
  echo "No default registry node baked in (pass a registry URL to stamp one)"
fi

build_host() { # os arch suffix
  local os="$1" arch="$2" suffix="$3"
  local dir="$bin/$os-$arch"
  mkdir -p "$dir"
  for c in alca apm apm-registry; do
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
for doc in language.md cli.md stdlib.md cross-compilation.md architecture.md apm.md apm-registry.md; do
  cp -f "docs/$doc" "$rel/docs/$doc"
done
cp -f examples/*.al "$rel/examples/"

echo "Release bundle is in $rel"
