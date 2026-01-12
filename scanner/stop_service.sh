#!/bin/bash

echo "========================================"
echo "Stopping Windows Network Manager Scanner Service"
echo "========================================"
echo ""

SERVICE_NAME="com.windowsnetworkmanager.scanner"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"
STOPPED=false

# Check if service is running via launchctl
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "Stopping launchctl service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
    STOPPED=true
fi

# Check for any running wnm-scanner processes
PIDS=$(pgrep -f "wnm-scanner" 2>/dev/null)
if [ -n "$PIDS" ]; then
    echo "Stopping wnm-scanner processes..."
    echo "$PIDS" | xargs kill 2>/dev/null || true
    STOPPED=true
    # Wait a moment and force kill if still running
    sleep 1
    REMAINING=$(pgrep -f "wnm-scanner" 2>/dev/null)
    if [ -n "$REMAINING" ]; then
        echo "$REMAINING" | xargs kill -9 2>/dev/null || true
    fi
fi

if [ "$STOPPED" = true ]; then
    echo ""
    echo "========================================"
    echo "Service stopped successfully!"
    echo "========================================"
    echo ""
else
    echo "No running service or processes found."
    echo ""
fi
