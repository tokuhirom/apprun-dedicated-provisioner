#!/bin/bash
set -euo pipefail

# This script is called by tagpr via tagpr.command
# Environment variables available:
#   TAGPR_CURRENT_VERSION - current version (e.g., v0.0.34)
#   TAGPR_NEXT_VERSION - next version (e.g., v0.0.35)

echo "Updating flake.nix for version ${TAGPR_NEXT_VERSION}"

# Run goreleaser in snapshot mode to get checksums
goreleaser release --snapshot --clean

# Extract version without 'v' prefix
VERSION="${TAGPR_NEXT_VERSION#v}"

# Extract hashes from snapshot checksums
LINUX_AMD64=$(grep linux_amd64 dist/checksums.txt | awk '{print $1}')
LINUX_ARM64=$(grep linux_arm64 dist/checksums.txt | awk '{print $1}')
DARWIN_AMD64=$(grep darwin_amd64 dist/checksums.txt | awk '{print $1}')
DARWIN_ARM64=$(grep darwin_arm64 dist/checksums.txt | awk '{print $1}')

echo "Extracted hashes:"
echo "  x86_64-linux:  $LINUX_AMD64"
echo "  aarch64-linux: $LINUX_ARM64"
echo "  x86_64-darwin: $DARWIN_AMD64"
echo "  aarch64-darwin: $DARWIN_ARM64"

# Update version
sed -i "s/version = \"[^\"]*\"/version = \"$VERSION\"/" flake.nix

# Update hashes
sed -i "s/\"x86_64-linux\" = \"[^\"]*\"/\"x86_64-linux\" = \"$LINUX_AMD64\"/" flake.nix
sed -i "s/\"aarch64-linux\" = \"[^\"]*\"/\"aarch64-linux\" = \"$LINUX_ARM64\"/" flake.nix
sed -i "s/\"x86_64-darwin\" = \"[^\"]*\"/\"x86_64-darwin\" = \"$DARWIN_AMD64\"/" flake.nix
sed -i "s/\"aarch64-darwin\" = \"[^\"]*\"/\"aarch64-darwin\" = \"$DARWIN_ARM64\"/" flake.nix

# Clean up dist directory
rm -rf dist

echo "Updated flake.nix to version $VERSION"
