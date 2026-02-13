# repomop

`repomop` is an interactive CLI utility for cleaning up large development artifacts in your projects directory.

## What It Does

1. Recursively scans a directory (`--path`, current directory by default).
2. Finds build/dependency artifacts using rules and project markers.
3. Calculates the size of each discovered directory.
4. Shows an interactive list (arrow keys, `Space`, `Enter`).
5. Requests confirmation before deletion (`y/N`).

Deletion is permanent (`os.RemoveAll`).

## Supported Artifacts

- Python: virtualenv with any directory name
- JavaScript/Node.js: `node_modules`
- Rust: `target`
- Swift SPM: `.build`
- Java/Kotlin (Gradle): `.gradle`, `build`, `out`
- Java (Maven): `target`
- C/C++ (CMake): `build`, `cmake-build-*`, `CMakeFiles`
- Dart/Flutter: `.dart_tool`, `build`
- Ruby: `.bundle`, `vendor/bundle`
- PHP: `vendor`

## Build and Run

```bash
make build
./repomop
```

Example: run against another directory

```bash
./repomop --path ~/Code
```

## Install via curl

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/mishamyrt/repomop/master/install.sh | sh
```

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/mishamyrt/repomop/master/install.sh | INSTALL_DIR=/usr/local/bin sh
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/mishamyrt/repomop/master/install.sh | VERSION=v0.1.0 sh
```

## CLI Flags

- `--path <dir>`: scan root
- `--max-depth <n>`: maximum depth (`-1` = unlimited)
- `--dry-run`: show only, do not delete
- `--yes`: delete all found artifacts without interactive confirmation

## TUI Controls

- `↑/↓` or `k/j`: move through the list
- `Space`: select/unselect item
- `Enter`: go to confirmation
- `y`: confirm deletion
- `n` or `Esc`: cancel confirmation and return to the list
- `q` or `Ctrl+C`: quit

## Release Artifacts

Each release tagged as `v*` publishes:

- `repomop_darwin_arm64.tar.gz`
- `repomop_linux_amd64.tar.gz`
- `repomop_linux_arm64.tar.gz`
- `checksums.txt` (SHA-256 for all archives)

The install script verifies archive checksums before installation.

## How to Publish a Release

Create and push a tag:

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

GitHub Actions will build artifacts and publish the GitHub Release automatically.

## Development Commands

```bash
make build   # build binary
make test    # run unit/integration tests
make lint    # run revive (uses revive.toml)
```
