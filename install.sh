#!/usr/bin/env sh
set -eu

REPO="${REPO:-mishamyrt/repomop}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *)
      echo "error: unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "error: unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

validate_platform() {
  platform="$1/$2"
  case "$platform" in
    darwin/arm64|linux/amd64|linux/arm64)
      ;;
    *)
      echo "error: unsupported platform combination: $platform" >&2
      echo "supported: darwin/arm64, linux/amd64, linux/arm64" >&2
      exit 1
      ;;
  esac
}

checksum_bin() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    echo "shasum"
    return
  fi

  echo ""
}

sha256_file() {
  file="$1"
  checker="$2"

  if [ "$checker" = "sha256sum" ]; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi

  shasum -a 256 "$file" | awk '{print $1}'
}

check_path_hint() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      ;;
    *)
      echo "warning: $INSTALL_DIR is not in PATH" >&2
      echo "Add this to your shell profile:" >&2
      echo "  export PATH=\"$INSTALL_DIR:\$PATH\"" >&2
      ;;
  esac
}

require_cmd curl
require_cmd tar
require_cmd uname
require_cmd mktemp
require_cmd awk
require_cmd grep

CHECKSUM_TOOL="$(checksum_bin)"
if [ -z "$CHECKSUM_TOOL" ]; then
  echo "error: neither sha256sum nor shasum is available" >&2
  exit 1
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
validate_platform "$OS" "$ARCH"
ASSET="repomop_${OS}_${ARCH}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

TMP_DIR="$(mktemp -d)"
ARCHIVE_PATH="$TMP_DIR/$ASSET"
CHECKSUMS_PATH="$TMP_DIR/checksums.txt"
EXTRACT_DIR="$TMP_DIR/extract"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo "Downloading $ASSET from $BASE_URL"
curl -fsSL "$BASE_URL/$ASSET" -o "$ARCHIVE_PATH"
curl -fsSL "$BASE_URL/checksums.txt" -o "$CHECKSUMS_PATH"

EXPECTED_SUM="$(grep "  $ASSET$" "$CHECKSUMS_PATH" | awk '{print $1}')"
if [ -z "$EXPECTED_SUM" ]; then
  echo "error: checksum entry not found for $ASSET" >&2
  exit 1
fi

ACTUAL_SUM="$(sha256_file "$ARCHIVE_PATH" "$CHECKSUM_TOOL")"
if [ "$EXPECTED_SUM" != "$ACTUAL_SUM" ]; then
  echo "error: checksum mismatch for $ASSET" >&2
  echo "expected: $EXPECTED_SUM" >&2
  echo "actual:   $ACTUAL_SUM" >&2
  exit 1
fi

mkdir -p "$EXTRACT_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"

if [ ! -f "$EXTRACT_DIR/repomop" ]; then
  echo "error: archive does not contain repomop binary" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
cp "$EXTRACT_DIR/repomop" "$INSTALL_DIR/repomop"
chmod 0755 "$INSTALL_DIR/repomop"

echo "repomop installed to $INSTALL_DIR/repomop"
check_path_hint
