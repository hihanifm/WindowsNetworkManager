#!/bin/bash

echo "========================================"
echo "Starting Windows Network Manager Scanner Service"
echo "========================================"
echo ""

SERVICE_NAME="com.windowsnetworkmanager.scanner"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"

# Check if service is installed
if [ ! -f "$TARGET_PLIST" ]; then
    echo "ERROR: Service is not installed!"
    echo "Please install the service first: ./install_service.sh"
    exit 1
fi

# Check if service is already running
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "Service is already running."
    echo ""
    echo "To check service status:"
    echo "  launchctl list | grep $SERVICE_NAME"
    echo ""
    echo "To view logs:"
    echo "  tail -f ~/Library/Logs/wnm-scanner/out.log"
    echo ""
    exit 0
fi

# Start the service
echo "Starting service..."
launchctl load "$TARGET_PLIST"

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "Service started successfully!"
    echo "========================================"
    echo ""
    echo "The scanner web interface is now running at:"
    echo "  http://localhost:18081"
    echo ""
    echo "To check service status:"
    echo "  launchctl list | grep $SERVICE_NAME"
    echo ""
    echo "To view logs:"
    echo "  tail -f ~/Library/Logs/wnm-scanner/out.log"
    echo ""
else
    echo ""
    echo "========================================"
    echo "Failed to start service!"
    echo "========================================"
    exit 1
fi
