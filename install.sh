#!/bin/sh
# cintelis installer for macOS and Linux: downloads the latest release for this
# machine, checks it against the release's checksums.txt, and installs it.
#   curl -fsSL https://raw.githubusercontent.com/cintelis/hackernews/main/install.sh | sh
set -eu

REPO="cintelis/hackernews"
INSTALL_DIR="${CINTELIS_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "cintelis: unsupported OS $(uname -s) — on Windows use install.ps1" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  arm64 | aarch64) arch=arm64 ;;
  x86_64 | amd64) arch=amd64 ;;
  *) echo "cintelis: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

# the releases/latest page redirects to .../tag/vX.Y.Z
version=$(curl -fsSI "https://github.com/$REPO/releases/latest" |
  tr -d '\r' | sed -n 's#^[Ll]ocation: .*/tag/v\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)$#\1#p')
if [ -z "$version" ]; then
  echo "cintelis: couldn't find the latest release" >&2
  exit 1
fi

asset="cintelis_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading cintelis $version ($os/$arch) ..."
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

want=$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
else
  got=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "cintelis: checksum mismatch for $asset — not installing" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp" cintelis
mkdir -p "$INSTALL_DIR"
mv "$tmp/cintelis" "$INSTALL_DIR/cintelis"
chmod +x "$INSTALL_DIR/cintelis"

echo "installed: $INSTALL_DIR/cintelis"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "note: $INSTALL_DIR is not on your PATH — add it to your shell profile" ;;
esac
