#!/bin/bash

# Build Release Script for Windows Network Manager
# This script builds the Windows executable and packages it with WinDivert.dll
# Usage: ./build_release.sh [version]
# Example: ./build_release.sh 2.0.0

set -e

VERSION="${1:-2.0.0}"
RELEASE_DIR="release"
ZIP_NAME="WindowsNetworkManager-v${VERSION}.zip"

echo "========================================"
echo "Building Windows Network Manager Release"
echo "Version: $VERSION"
echo "========================================"
echo ""

# Clean up previous release
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# Build Windows executable
echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -o "$RELEASE_DIR/WindowsNetworkManager.exe"
if [ $? -ne 0 ]; then
    echo "ERROR: Build failed"
    exit 1
fi
echo "✓ Built WindowsNetworkManager.exe"
echo ""

# Copy WinDivert DLL from vendor directory
echo "Copying WinDivert DLL from vendor directory..."
if [ -f "vendor/windivert/WinDivert.dll" ]; then
    cp vendor/windivert/WinDivert.dll "$RELEASE_DIR/WinDivert.dll"
    echo "✓ Copied WinDivert.dll from vendor directory"
elif [ -f "WinDivert.dll" ]; then
    cp WinDivert.dll "$RELEASE_DIR/WinDivert.dll"
    echo "✓ Copied WinDivert.dll from current directory"
else
    echo "WARNING: WinDivert.dll not found in vendor/windivert/ or current directory"
    echo "Please ensure WinDivert.dll is in vendor/windivert/ directory"
    echo "Or download from: https://www.reqrypt.org/windivert.html"
    exit 1
fi
echo ""

# Create ZIP bundle
echo "Creating release bundle..."
cd "$RELEASE_DIR"
zip -r "../$ZIP_NAME" WindowsNetworkManager.exe WinDivert.dll > /dev/null
cd ..
echo "✓ Created $ZIP_NAME"
echo ""

# Show file sizes
echo "Release files:"
ls -lh "$RELEASE_DIR"/
echo ""
echo "Release bundle:"
ls -lh "$ZIP_NAME"
echo ""

echo "========================================"
echo "Release build complete!"
echo "========================================"
echo ""
echo "Files created:"
echo "  - $RELEASE_DIR/WindowsNetworkManager.exe"
echo "  - $RELEASE_DIR/WinDivert.dll"
echo "  - $ZIP_NAME"
echo ""
echo "To create a GitHub release:"
echo "  1. Review the files in $RELEASE_DIR/"
echo "  2. If satisfied, upload $ZIP_NAME to GitHub Releases"
echo "  3. Or commit $ZIP_NAME and push (CI will upload it)"
echo ""
