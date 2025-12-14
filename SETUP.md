# Setup Guide

## Quick Start

### 1. Install Go
Download and install Go 1.21 or later from https://golang.org/dl/

### 2. Download WinDivert
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
Open your browser and navigate to: **http://localhost:8080**

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
- **Solution**: Verify port 8080 is not in use by another application
- **Solution**: Check console output for specific error messages

### Packets not being delayed
- **Solution**: Verify delay is set to a value > 0
- **Solution**: Ensure "Start" button was clicked and status shows "Running"
- **Solution**: Check that you have outbound network traffic
- **Solution**: Review console logs for error messages

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

