#!/bin/bash

echo "========================================"
echo "Uninstalling Windows Network Manager Scanner Service"
echo "========================================"
echo ""

SERVICE_NAME="com.windowsnetworkmanager.scanner"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"

# Unload service if running
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "Stopping service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
fi

# Remove plist file
if [ -f "$TARGET_PLIST" ]; then
    echo "Removing service configuration..."
    rm "$TARGET_PLIST"
fi

echo ""
echo "========================================"
echo "Service uninstalled successfully!"
echo "========================================"
echo ""
