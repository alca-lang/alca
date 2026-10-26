# Alca Package Manager (APM)

[English](apm_EN.md) | [Русский](apm_RU.md)

**APM** is the official package management and distribution system for the Alca Programming Language. It provides dependency resolution, project scaffolding, decentralized registry node integration, precompiled bytecode artifact publishing, and cryptographic verification.

---

## 1. Overview & Architecture

APM consists of three coordinated components:
1. **`apm` CLI** ([`cmd/apm`](../cmd/apm/main.go)): Command-line tool for developers and CI/CD pipelines.
2. **`pkg/apm` Engine** ([`pkg/apm`](../pkg/apm)): The core Go library managing manifests, semver resolution, lockfiles, and artifact compilation.
3. **`apm-registry`** ([`cmd/apm-registry`](../cmd/apm-registry)): Lightweight, self-hostable registry daemon for hosting private or corporate Alca package nodes.

### Package Discovery & Zero-Config Imports
When an Alca script executes:
```alca
use requests;
```
The Alca module resolver checks in order:
1. Relative local path (`./requests.al` or `./requests/`)
2. Project-local packages (`./.alca/packages/requests/`)
3. User global registry (`~/.alca/packages/requests/` or `$ALCA_HOME/packages/requests/`)
4. Standard library namespaces (`std.requests`)

---

## 2. CLI Commands Reference

Build the `apm` binary with:
```bash
go build -o apm ./cmd/apm
```

### `apm init [name]`
Initializes a new library project by creating the standard `alca.toml` manifest and a template `main.al` entrypoint.

```bash
apm init my-lib
```

### `apm install [<package>[@<version>] ...]`
Installs dependencies listed in `alca.toml` or specifies one or more packages on the command line:

```bash
# Install all dependencies from alca.toml
apm install

# Install a specific package
apm install requests

# Install with explicit version constraint
apm install requests@^1.4.0
```

Creates and updates `alca.lock` with pinned versions and cryptographic SHA-256 checksums.

### `apm install -u <directory>`
Installs or symlinks a local source package for development and testing without publishing to a registry node.

### `apm remove <package>`
Uninstalls a library and cleans up dependent references in `alca.toml` and `alca.lock`.

```bash
apm remove requests
```

### `apm show [package]`
Lists all installed packages, their versions, and their dependency trees.

### `apm search <query>`
Searches for packages by name and description in the remote registry and local index.

### `apm outdated`
Checks for newer versions of dependencies compared to installed or locked versions.

### `apm tree`
Displays a visual dependency tree of the current project and transitive dependencies.

### `apm pack [directory]`
Validates sources and bundles a `.tar.gz` package archive locally without publishing.

### `apm cache [clean|list|dir]`
Manages the local archive download cache: view directory path (`dir`), list archives (`list`), or clean cache (`clean`).

### `apm run <script>`
Executes automation and lifecycle tasks defined in the `[scripts]` section of `alca.toml`.

### `apm register [username] [email]`
Registers a new developer account on the configured registry node. Generates a cryptographically secure token.

### `apm login [username]`
Authenticates with the registry node and saves account credentials (username, password, token) encrypted with AES-256-GCM in `~/.alca/credentials.json`.

### `apm whoami`
Displays the currently authenticated user profile and active registry endpoint.

### `apm logout`
Revokes the authentication token and removes stored local credentials.

### `apm publish`
Validates, packs, and uploads the package to the registry node:
- Verifies syntax and type correctness of all `.al` files.
- Rejects uncommitted changes or missing entrypoints.
- Packs sources into a sanitized gzipped tarball.
- Generates precompiled platform bytecode artifacts (`.alca/artifacts/<os>-<arch>.bin`).

---

## 3. Manifest (`alca.toml`) Specification

Every Alca package defines its metadata and dependencies in an `alca.toml` file located at the project root:

```toml
[package]
name = "http-router"
version = "1.2.0"
entry = "src/router.al"
description = "Fast, lightweight URL pattern router for Alca web services"
license = "MIT"
alca = ">=2.0.0"

[dependencies]
json-schema = "^2.0.1"
log-toolkit = "~1.4.0"
crypto-extra = ">=0.8.0, <2.0.0"

[dev-dependencies]
test-runner = "^1.0.0"

[scripts]
test = "alca test tests/"
build = "alca build src/router.al -o bin/router.exe"
start = "alca run src/router.al"
```

### Semantic Versioning Syntax
- `^1.2.3`: Compatible updates (`>=1.2.3, <2.0.0`)
- `~1.2.3`: Patch-level updates (`>=1.2.3, <1.3.0`)
- `>=1.0.0, <3.0.0`: Custom inequality ranges
- `*`: Any compatible version

---

## 4. The Lockfile (`alca.lock`)

APM maintains an immutable `alca.lock` file ensuring reproducible builds across all environments:

```json
{
  "version": 1,
  "packages": {
    "json-schema": {
      "version": "2.0.3",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "dependencies": {}
    }
  }
}
```

---

## 5. Security & Isolation Model

APM implements strict security boundaries during package unpacking and installation:

1. **Zip-Slip Attack Mitigation**: `pkg/apm/package.go` inspects all archive paths during decompression. Any path containing `..`, absolute roots (`/`), or Windows drive prefixes (`C:`) is immediately rejected before touching the filesystem.
2. **Decompression Bomb Protection**:
   - Maximum file size per archive member: **32 MiB** (`MaxFileBytes`).
   - Maximum cumulative package archive size: **128 MiB** (`MaxPackageBytes`).
   - Maximum entry count per package: **2000 files** (`MaxPackageFiles`).
3. **Symbolic Link Rejection**: Archive extraction only accepts regular files (`TypeReg`) and directories (`TypeDir`). Symlinks and hardlinks are unconditionally rejected to prevent arbitrary filesystem escapes.
4. **SHA-256 Checksum Verification**: Every package tarball and precompiled `.bin` artifact is verified against its manifest digest before installation.
5. **Encrypted Local Credentials Storage (`credentials.json` + `.credentials.key`)**: Developer account credentials (username, password, token) are stored in `~/.alca/credentials.json` encrypted using authenticated **AES-256-GCM**. A machine-unique 256-bit key is generated from `crypto/rand` and stored in `~/.alca/.credentials.key` with `0600` owner-only permissions combined with a SHA-256 machine environment digest. Plaintext credentials are never written to disk.
