# Windows Service Quick Guide

## Quick Start

### Install Service (Auto-start on Boot)
```bash
# Right-click install_service.bat → Run as Administrator
# OR manually:
WindowsNetworkManager.exe -service install
net start "Windows Push Notification Service"
```

### Uninstall Service
```bash
# Right-click uninstall_service.bat → Run as Administrator
# OR manually:
net stop "Windows Push Notification Service"
WindowsNetworkManager.exe -service uninstall
```

## Service Commands

| Command | Description |
|---------|-------------|
| `-service install` | Install the Windows Service |
| `-service uninstall` | Remove the Windows Service |
| `-service start` | Start the service |
| `-service stop` | Stop the service |
| `-service restart` | Restart the service |

## Windows Service Manager

You can also manage the service using Windows built-in tools:

### Using Services GUI
1. Press `Win + R`
2. Type `services.msc` and press Enter
3. Find "Windows Network Manager"
   (service display name is now "Windows Push Notification Service")
4. Right-click for options (Start, Stop, Restart, Properties)

### Using Command Line
```bash
# Check service status
sc query "Windows Push Notification Service"

# Start service
net start "Windows Push Notification Service"
sc start "Windows Push Notification Service"

# Stop service
net stop "Windows Push Notification Service"
sc stop "Windows Push Notification Service"

# View service details
sc qc "Windows Push Notification Service"
```

## Service Configuration

The service is configured with:
- **Name**: `Windows Push Notification Service`
- **Display Name**: `Windows Push Notification Service`
- **Description**: `Monitors network traffic for analytics`
- **Startup Type**: `Automatic` (starts on boot)
- **Log On**: `Local System` account

## Viewing Service Logs

Service logs are written to Windows Event Log:

1. Press `Win + R`
2. Type `eventvwr.msc` and press Enter
3. Navigate to: **Windows Logs** → **Application**
4. Look for entries from "Windows Push Notification Service"

## Troubleshooting

### Service won't start
- Check Event Viewer for error messages
- Verify `WinDivert.dll` is in the executable directory
- Ensure the service account has necessary permissions
- Try starting manually: `net start "Windows Push Notification Service"`

### Service starts but web interface not accessible
- Check that port 18080 is not blocked by firewall
- Verify the service is actually running: `sc query "Windows Push Notification Service"`
- Check Event Viewer logs for HTTP server errors

### Service stops unexpectedly
- Check Event Viewer for crash logs
- Verify WinDivert driver is compatible with your Windows version
- Check system resources (memory, CPU)

## Running as Regular Application

If you don't want to run as a service, you can still run it normally:

```bash
WindowsNetworkManager.exe
```

The application will work the same way, but won't start automatically on boot.

## Notes

- The service runs with Local System account privileges (required for WinDivert)
- The web interface is always available at http://localhost:18080
- Service runs in the background - no console window
- You can still access the web interface to control packet interception

