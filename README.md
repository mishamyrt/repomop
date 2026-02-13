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

## Usage

Go to your projects directory and run:

```bash
repomop
```

## TUI Controls

- `↑/↓` or `k/j`: move through the list
- `Space`: select/unselect item
- `Enter`: go to confirmation
- `y`: confirm deletion
- `n` or `Esc`: cancel confirmation and return to the list
- `q` or `Ctrl+C`: quit
