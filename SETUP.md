# Setup Guide

## Quick Start (Windows Users)

### Fastest Installation Method

1. **[Download from GitHub Pages](https://hihanifm.github.io/WindowsNetworkManager/)** (recommended)
   - Complete package with EXE and DLL included
   - No additional downloads needed
   - Extract and run as Administrator

2. **Or download from [GitHub Releases](https://github.com/hihanifm/WindowsNetworkManager/releases/latest)**
   - Download `WindowsNetworkManager-vX.X.X.zip`
   - Extract the ZIP file
   - Run `WindowsNetworkManager.exe` as Administrator

3. Access the web interface at **http://localhost:18080**

> **Note:** All releases include WinDivert.dll - no separate download needed!

## Development Setup (For Building from Source)

### 1. Install Go
Download and install Go 1.21 or later from https://golang.org/dl/

### 2. Download WinDivert (if building from source)
1. Go to https://www.reqrypt.org/windivert.html
2. Download the latest version (WinDivert 2.2 or later)
3. Extract the archive
4. Copy these files to your project directory:
   - `WinDivert.dll` (from the `x64` folder for 64-bit Windows)
   - `WinDivert64.sys` (from the `x64` folder)

### 3. Build the Application
```bash
# Install Go dependencies
go mod tidy

# Build the executable
go build -o WindowsNetworkManager.exe

# Or use the build script
build.bat
```

### 4. Run as Administrator
1. Right-click on `WindowsNetworkManager.exe`
2. Select "Run as Administrator"
3. Or open Command Prompt/PowerShell as Administrator and run:
   ```bash
   .\WindowsNetworkManager.exe
   ```

### 5. Access Web Interface

**Local Access:**
Open your browser and navigate to: **http://localhost:18080**

**Network Access (from other devices):**
1. Find your PC's IP address (check console output when app starts, or use `ipconfig`)
2. From another device on the same WiFi/LAN, open: `http://<PC_IP>:18080`
3. **Configure Firewall:** Run `configure_firewall.bat` as Administrator to allow network access

### 6. Configure Firewall for Network Access 🌐

To allow other devices on your network to access the web interface:

**Quick Setup:**
1. Right-click `configure_firewall.bat`
2. Select "Run as Administrator"
3. Firewall rule will be added automatically

**Manual Setup:**
- Open Windows Defender Firewall → Advanced Settings
- Inbound Rules → New Rule → Port → TCP → 18080 → Allow

### 7. Install as Windows Service (Auto-start on Boot) ⭐ NEW

To make the application start automatically when Windows boots:

**Quick Install:**
1. Right-click `install_service.bat`
2. Select "Run as Administrator"
3. The service is now installed and will start on boot!

**Manual Install:**
```bash
# Run as Administrator
WindowsNetworkManager.exe -service install
net start WindowsNetworkManager
```

**Service Commands:**
```bash
# Start service
net start WindowsNetworkManager
# Or: WindowsNetworkManager.exe -service start

# Stop service
net stop WindowsNetworkManager
# Or: WindowsNetworkManager.exe -service stop

# Restart service
WindowsNetworkManager.exe -service restart

# Uninstall service
WindowsNetworkManager.exe -service uninstall
# Or: Right-click uninstall_service.bat → Run as Administrator
```

**Verify Service:**
- Open Services (`Win+R` → `services.msc`)
- Look for "Windows Network Manager"
- Check that it's set to "Automatic" startup type

## Troubleshooting

### "Failed to open WinDivert handle"
- **Solution**: Ensure you're running as Administrator
- **Solution**: Verify `WinDivert.dll` is in the same directory as the executable
- **Solution**: Check that WinDivert driver is compatible with your Windows version

### "Access Denied" errors
- **Solution**: The application MUST run with Administrator privileges
- **Solution**: Disable User Account Control (UAC) temporarily, or always run as Administrator

### Application won't start
- **Solution**: Check Windows Firewall isn't blocking the application
- **Solution**: Verify port 18080 is not in use by another application
- **Solution**: Check console output for specific error messages

### Packets not being delayed
- **Solution**: Verify delay is set to a value > 0
- **Solution**: Ensure "Start" button was clicked and status shows "Running"
- **Solution**: Check that you have outbound network traffic
- **Solution**: Review console logs for error messages

### Service won't start or install
- **Solution**: Ensure you're running installation commands as Administrator
- **Solution**: Check Windows Event Viewer for service errors
- **Solution**: Verify `WinDivert.dll` is in the same directory as the executable
- **Solution**: Try: `sc query WindowsNetworkManager` to check service status
- **Solution**: Check service logs in Event Viewer → Windows Logs → Application

### Cannot access from other devices on network
- **Solution**: Run `configure_firewall.bat` as Administrator to open port 18080
- **Solution**: Verify devices are on the same network (same WiFi/LAN)
- **Solution**: Check the console output for your PC's IP address
- **Solution**: Try accessing `http://localhost:18080` on the host PC first to verify it works
- **Solution**: Check Windows Firewall settings manually if the script doesn't work

## API Compatibility Notes

If you encounter issues with the godivert library API, the following methods might need adjustment:

### Packet Sending
The code uses `handle.Send(packet)`. If this doesn't work, try:
```go
packet.Send(handle)
```

### Packet Length
The code tries to access `packet.Raw`. If this field doesn't exist, the library might use:
- `packet.Bytes`
- `packet.Data`
- Or calculate length differently

Check the godivert library documentation or source code for the exact API.

## Testing

1. Start the application as Administrator
2. Open the web interface
3. Set a delay (e.g., 100ms)
4. Click "Start"
5. Open a browser and visit a website
6. You should notice the website loads slower
7. Check the statistics to see packets being processed

## Next Steps

- Monitor the statistics in real-time
- Adjust delay values to test different scenarios
- Use network monitoring tools to verify packet delays
- Test with different applications to see the effect

