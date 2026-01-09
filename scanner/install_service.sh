#!/bin/bash

echo "========================================"
echo "Installing Windows Network Manager Scanner Service"
echo "========================================"
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PLIST_FILE="$SCRIPT_DIR/com.windowsnetworkmanager.scanner.plist"
SERVICE_NAME="com.windowsnetworkmanager.scanner"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="$LAUNCH_AGENTS_DIR/$SERVICE_NAME.plist"

# Use current user's home directory
USER_HOME="$HOME"

# Check if scanner binary exists
if [ ! -f "$SCRIPT_DIR/wnm-scanner" ]; then
    echo "ERROR: wnm-scanner binary not found!"
    echo "Please build the scanner first: ./build.sh"
    exit 1
fi

# Create logs directory
mkdir -p "$HOME/Library/Logs/wnm-scanner"

# Update plist with correct paths
echo "Updating service configuration..."

# Get absolute path to scanner binary
SCANNER_PATH="$(cd "$SCRIPT_DIR" && pwd)/wnm-scanner"

# Create plist with correct paths
cat > "$TARGET_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>$SERVICE_NAME</string>
	<key>ProgramArguments</key>
	<array>
		<string>$SCANNER_PATH</string>
		<string>-web</string>
		<string>-port</string>
		<string>18081</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>$USER_HOME/Library/Logs/wnm-scanner/out.log</string>
	<key>StandardErrorPath</key>
	<string>$USER_HOME/Library/Logs/wnm-scanner/error.log</string>
	<key>WorkingDirectory</key>
	<string>$SCRIPT_DIR</string>
</dict>
</plist>
EOF

# Unload existing service if running
if launchctl list | grep -q "$SERVICE_NAME"; then
    echo "Stopping existing service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
fi

# Load and start the service
echo "Loading service..."
launchctl load "$TARGET_PLIST"

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "Service installed successfully!"
    echo "========================================"
    echo ""
    echo "The scanner web interface is now running at:"
    echo "  http://localhost:18081"
    echo ""
    echo "The service will start automatically on boot."
    echo ""
    echo "To check service status:"
    echo "  launchctl list | grep $SERVICE_NAME"
    echo ""
    echo "To view logs:"
    echo "  tail -f $USER_HOME/Library/Logs/wnm-scanner/out.log"
    echo ""
else
    echo ""
    echo "========================================"
    echo "Service installation failed!"
    echo "========================================"
    exit 1
fi
