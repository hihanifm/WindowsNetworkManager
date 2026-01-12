#!/bin/bash

# Release Script for Windows Network Manager
# This script builds the release package and uploads it to GitHub Releases
# Usage: ./release.sh [version]
# Example: ./release.sh 2.6.0

set -e

# Get version from argument or from version.go
if [ -z "$1" ]; then
    VERSION=$(grep 'const Version =' version/version.go | sed 's/.*"\(.*\)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "ERROR: Could not determine version. Please specify version as argument."
        echo "Usage: ./release.sh [version]"
        exit 1
    fi
    echo "Using version from version.go: $VERSION"
else
    VERSION="$1"
fi

DIST_DIR="dist"
ZIP_NAME="WindowsNetworkManager-v${VERSION}.zip"
TAG_NAME="v${VERSION}"

echo "========================================"
echo "Building Windows Network Manager Release"
echo "Version: $VERSION"
echo "========================================"
echo ""

# Clean up previous dist
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# Build Windows executable
echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -v -o "$DIST_DIR/WindowsNetworkManager.exe" .
if [ $? -ne 0 ]; then
    echo "ERROR: Build failed"
    exit 1
fi
echo "✓ Built WindowsNetworkManager.exe"
echo ""

# Copy WinDivert DLL from vendor directory
echo "Copying WinDivert DLL from vendor directory..."
if [ -f "vendor/windivert/WinDivert.dll" ]; then
    cp vendor/windivert/WinDivert.dll "$DIST_DIR/WinDivert.dll"
    echo "✓ Copied WinDivert.dll from vendor directory"
elif [ -f "WinDivert.dll" ]; then
    cp WinDivert.dll "$DIST_DIR/WinDivert.dll"
    echo "✓ Copied WinDivert.dll from current directory"
else
    echo "ERROR: WinDivert.dll not found in vendor/windivert/ or current directory"
    echo "Please ensure WinDivert.dll is in vendor/windivert/ directory"
    exit 1
fi
echo ""

# Copy web directory
echo "Copying web interface files..."
if [ -d "web" ]; then
    cp -r web "$DIST_DIR/web"
    echo "✓ Copied web directory (index.html, static/app.js)"
else
    echo "ERROR: web directory not found"
    echo "Please ensure web/ directory exists with index.html and static/app.js"
    exit 1
fi
echo ""

# Copy batch files
echo "Copying batch files..."
if [ -f "install_service.bat" ]; then
    cp install_service.bat "$DIST_DIR/"
    echo "✓ Copied install_service.bat"
fi
if [ -f "uninstall_service.bat" ]; then
    cp uninstall_service.bat "$DIST_DIR/"
    echo "✓ Copied uninstall_service.bat"
fi
if [ -f "configure_firewall.bat" ]; then
    cp configure_firewall.bat "$DIST_DIR/"
    echo "✓ Copied configure_firewall.bat"
fi
echo ""

# Create ZIP bundle
echo "Creating release bundle..."
cd "$DIST_DIR"
zip -r "../$ZIP_NAME" WindowsNetworkManager.exe WinDivert.dll web/ install_service.bat uninstall_service.bat configure_firewall.bat > /dev/null
cd ..
echo "✓ Created $ZIP_NAME"
echo ""

# Show file sizes
echo "Release files in dist/:"
ls -lh "$DIST_DIR"/
echo ""
echo "Release bundle:"
ls -lh "$ZIP_NAME"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "========================================"
    echo "GitHub CLI (gh) not found!"
    echo "========================================"
    echo ""
    echo "To upload to GitHub Releases, install GitHub CLI:"
    echo "  brew install gh  # macOS"
    echo "  Or download from: https://cli.github.com/"
    echo ""
    echo "Then run:"
    echo "  gh auth login"
    echo "  gh release create $TAG_NAME $ZIP_NAME --title \"Release $TAG_NAME\" --notes \"Windows Network Manager $VERSION\""
    echo ""
    echo "Or manually upload $ZIP_NAME to:"
    echo "  https://github.com/hihanifm/WindowsNetworkManager/releases/new"
    echo ""
    exit 0
fi

# Check if user is authenticated
if ! gh auth status &> /dev/null; then
    echo "========================================"
    echo "GitHub CLI not authenticated!"
    echo "========================================"
    echo ""
    echo "Please run: gh auth login"
    echo ""
    exit 1
fi

# Check if tag exists
if git rev-parse "$TAG_NAME" >/dev/null 2>&1; then
    echo "Tag $TAG_NAME already exists."
    read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "$TAG_NAME" 2>/dev/null || true
        git push origin ":refs/tags/$TAG_NAME" 2>/dev/null || true
        echo "Deleted existing tag"
    else
        echo "Skipping tag creation. Using existing tag."
    fi
fi

# Create tag if it doesn't exist
if ! git rev-parse "$TAG_NAME" >/dev/null 2>&1; then
    echo "Creating git tag $TAG_NAME..."
    git tag "$TAG_NAME"
    git push origin "$TAG_NAME"
    echo "✓ Tag created and pushed"
    echo ""
fi

# Create or update release
echo "Creating/updating GitHub Release..."
RELEASE_NOTES="## Windows Network Manager $VERSION

### Installation
1. Download \`WindowsNetworkManager-v${VERSION}.zip\`
2. Extract the ZIP file
3. Right-click \`install_service.bat\` → Run as Administrator
4. The service will install and start automatically
5. Access web interface at http://localhost:18080

### Files Included
- \`WindowsNetworkManager.exe\` - Main application
- \`WinDivert.dll\` - Required library (WinDivert 2.2.0)
- \`web/\` - Web interface files (index.html, static/app.js)
- \`install_service.bat\` - Install and start as Windows service (run as Administrator)
- \`uninstall_service.bat\` - Uninstall Windows service (run as Administrator)
- \`configure_firewall.bat\` - Configure Windows Firewall (run as Administrator)

### Quick Install
See [GitHub Pages](https://hihanifm.github.io/WindowsNetworkManager/) for quick installation instructions."

# Check if release exists
if gh release view "$TAG_NAME" &> /dev/null; then
    echo "Release $TAG_NAME already exists. Uploading assets..."
    gh release upload "$TAG_NAME" "$ZIP_NAME" "$DIST_DIR/WindowsNetworkManager.exe" "$DIST_DIR/WinDivert.dll" --clobber
    echo "✓ Assets uploaded to existing release"
else
    echo "Creating new release $TAG_NAME..."
    gh release create "$TAG_NAME" \
        "$ZIP_NAME" \
        "$DIST_DIR/WindowsNetworkManager.exe" \
        "$DIST_DIR/WinDivert.dll" \
        --title "Release $TAG_NAME" \
        --notes "$RELEASE_NOTES"
    echo "✓ Release created and assets uploaded"
fi

echo ""
echo "========================================"
echo "Release complete!"
echo "========================================"
echo ""
echo "Release URL:"
echo "  https://github.com/hihanifm/WindowsNetworkManager/releases/tag/$TAG_NAME"
echo ""
echo "Files created:"
echo "  - $DIST_DIR/WindowsNetworkManager.exe"
echo "  - $DIST_DIR/WinDivert.dll"
echo "  - $DIST_DIR/web/ (index.html, static/app.js)"
echo "  - $DIST_DIR/install_service.bat"
echo "  - $DIST_DIR/uninstall_service.bat"
echo "  - $DIST_DIR/configure_firewall.bat"
echo "  - $ZIP_NAME"
echo ""
