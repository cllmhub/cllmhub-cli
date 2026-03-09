#!/bin/sh
set -e

# Updates the Homebrew formula with the correct version and SHA256 checksums.
# Usage: ./scripts/update-formula.sh <version> <binaries-dir>
# Example: ./scripts/update-formula.sh 0.4.9 bin/

VERSION="$1"
BIN_DIR="$2"
FORMULA="Formula/cllmhub.rb"

if [ -z "$VERSION" ] || [ -z "$BIN_DIR" ]; then
  echo "Usage: $0 <version> <binaries-dir>"
  exit 1
fi

SHA_DARWIN_AMD64=$(sha256sum "$BIN_DIR/cllmhub-darwin-amd64" | awk '{print $1}')
SHA_DARWIN_ARM64=$(sha256sum "$BIN_DIR/cllmhub-darwin-arm64" | awk '{print $1}')
SHA_LINUX_AMD64=$(sha256sum "$BIN_DIR/cllmhub-linux-amd64" | awk '{print $1}')
SHA_LINUX_ARM64=$(sha256sum "$BIN_DIR/cllmhub-linux-arm64" | awk '{print $1}')

sed -i "s/version \".*\"/version \"$VERSION\"/" "$FORMULA"
sed -i "s/PLACEHOLDER_DARWIN_ARM64/$SHA_DARWIN_ARM64/" "$FORMULA"
sed -i "s/PLACEHOLDER_DARWIN_AMD64/$SHA_DARWIN_AMD64/" "$FORMULA"
sed -i "s/PLACEHOLDER_LINUX_ARM64/$SHA_LINUX_ARM64/" "$FORMULA"
sed -i "s/PLACEHOLDER_LINUX_AMD64/$SHA_LINUX_AMD64/" "$FORMULA"

echo "Updated $FORMULA for version $VERSION"
