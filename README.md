# Windows Network Manager

A Windows application with a web interface that monitors network traffic and adds configurable latency to network packets using WinDivert (user-mode).

## Features

- 🌐 Web-based interface for easy configuration
- 🌍 Network access - accessible from other devices on WiFi/LAN
- ⏱️ Configurable packet delay (0-10000ms)
- 📊 Real-time network statistics
- 🚀 User-mode implementation (no kernel driver required)
- ⚡ Start/Stop packet interception on demand
- 🔄 Dynamic delay adjustment while running
- 🔧 Windows Service support - runs automatically on boot
- 🔍 Mac Network Scanner - discover Windows instances from macOS
- 🔄 Remote Upgrade - update the application remotely via web interface

## Prerequisites

### Development (macOS)
1. **macOS** (for development)
2. **Go 1.21+** - [Download](https://golang.org/dl/)
3. **Make** (usually pre-installed on macOS)

### Runtime (Windows)
1. **Windows 10/11** (64-bit) - Required to run the application
2. **WinDivert Library** - [Download](https://www.reqrypt.org/windivert.html)
3. **Administrator privileges** (required to run on Windows)

## Development Setup (macOS)

### 1. Install Go

```bash
# Using Homebrew (recommended)
brew install go

# Or download from: https://golang.org/dl/
```

### 2. Build for Windows

The project cross-compiles from macOS to Windows:

```bash
# Option 1: Using the build script
./build.sh

# Option 2: Using Make
make build

# Option 3: Manual cross-compilation
GOOS=windows GOARCH=amd64 go build -o WindowsNetworkManager.exe
```

This creates `WindowsNetworkManager.exe` which you can copy to your Windows PC.

### 3. Transfer to Windows

1. Copy `WindowsNetworkManager.exe` to your Windows PC
2. Download WinDivert and copy `WinDivert.dll` to the same directory
3. Run on Windows as Administrator

## Runtime Setup (Windows)

### 1. Download WinDivert

1. Download WinDivert from: https://www.reqrypt.org/windivert.html
2. Extract the archive
3. Copy the following files to your project directory:
   - `WinDivert.dll` (from `x64` folder for 64-bit Windows)
   - `WinDivert32.sys` or `WinDivert64.sys` (from `x64` folder)
   
   **Note:** The Go library (`github.com/deblasis/godivert`) will automatically load the DLL, but you need to ensure it's in the same directory as the executable or in your system PATH.

### 2. Build the Project (on macOS)

```bash
# Install dependencies
go mod tidy

# Build for Windows (cross-compile from Mac)
./build.sh
# Or: make build
```

This will create `WindowsNetworkManager.exe` ready for Windows.

### 3. Run on Windows

**IMPORTANT:** Run as Administrator on Windows!

1. Copy `WindowsNetworkManager.exe` and `WinDivert.dll` to your Windows PC
2. Right-click Command Prompt or PowerShell
3. Select "Run as Administrator"
4. Navigate to the directory with the executable
5. Run: `.\WindowsNetworkManager.exe`

The web interface will be available at: **http://localhost:18080**

### 5. Run as Windows Service (Auto-start on Boot) ⭐ NEW

To run the application automatically when Windows boots:

#### Option A: Using Installation Scripts (Recommended)

1. **Install the service:**
   - Right-click `install_service.bat`
   - Select "Run as Administrator"
   - The service will be installed and set to start automatically on boot

2. **Start the service:**
   ```bash
   net start WindowsNetworkManager
   ```
   Or use: `WindowsNetworkManager.exe -service start`

3. **Stop the service:**
   ```bash
   net stop WindowsNetworkManager
   ```
   Or use: `WindowsNetworkManager.exe -service stop`

4. **Uninstall the service:**
   - Right-click `uninstall_service.bat`
   - Select "Run as Administrator"

#### Option B: Manual Installation

```bash
# Install service (run as Administrator)
WindowsNetworkManager.exe -service install

# Start service
WindowsNetworkManager.exe -service start
# Or: net start WindowsNetworkManager

# Stop service
WindowsNetworkManager.exe -service stop
# Or: net stop WindowsNetworkManager

# Restart service
WindowsNetworkManager.exe -service restart

# Uninstall service
WindowsNetworkManager.exe -service uninstall
```

#### Service Management

- **View service status:** Open Services (`services.msc`) and look for "Windows Network Manager"
- **Configure startup type:** The service is set to start automatically by default
- **View logs:** Service logs are written to Windows Event Log (use Event Viewer)

**Note:** The service runs in the background and starts automatically on boot. The web interface remains available at http://localhost:18080 even when running as a service.

## Network Access (Access from Other Devices) 🌐

The web interface is accessible from other devices on the same network (WiFi/LAN):

1. **The server automatically binds to all network interfaces** (0.0.0.0)
2. **Find your PC's IP address:**
   - Check the console/event log when the application starts - it will display network-accessible URLs
   - Or use: `ipconfig` in Command Prompt and look for "IPv4 Address"
   - Or access the API: `http://localhost:18080/api/network` to get IP addresses
   - **Or use the Mac Scanner** (see below) to automatically discover instances

3. **Access from other devices:**
   - From another PC/phone/tablet on the same network, open: `http://<PC_IP_ADDRESS>:18080`
   - Example: If your PC's IP is `192.168.1.100`, use `http://192.168.1.100:18080`

4. **Windows Firewall Configuration:**
   - Windows may prompt you to allow the connection when first accessed
   - Or manually allow port 18080 in Windows Firewall:
     - Open Windows Defender Firewall → Advanced Settings
     - Inbound Rules → New Rule
     - Port → TCP → 18080 → Allow connection
     - Apply to all profiles

## Mac Network Scanner 🔍

A Mac companion tool to automatically discover Windows instances on your network.

### Building the Scanner

```bash
cd scanner
./build.sh
# Or: make build
```

### Using the Scanner

```bash
# Scan network for Windows instances
./wnm-scanner scan

# Scan with custom settings
./wnm-scanner scan -workers 50 -timeout 1s

# Output in JSON format
./wnm-scanner scan -json

# Open discovered instance in browser
./wnm-scanner open 192.168.1.100

# List instances (alias for scan)
./wnm-scanner list
```

### Scanner Features

- **Automatic Network Detection**: Detects your local subnet automatically
- **Parallel Scanning**: Scans multiple IPs simultaneously (default: 30 workers)
- **Progress Tracking**: Shows real-time scan progress
- **Instance Verification**: Verifies discovered endpoints are WindowsNetworkManager
- **Quick Access**: Open discovered instances directly in browser

### Installation (Optional)

```bash
cd scanner
make install
# Now you can use 'wnm-scanner' from anywhere
```

## Usage

1. **Open the web interface** in your browser: 
   - **Local access:** http://localhost:18080
   - **Network access:** http://<PC_IP_ADDRESS>:18080 (from other devices)
2. **Set the desired delay** in milliseconds (0-10000)
3. **Click "Start"** to begin intercepting and delaying packets
4. **Monitor statistics** in real-time
5. **Adjust delay** while running (click "Set Delay" to update)
6. **Click "Stop"** to stop packet interception

## How It Works

The application uses WinDivert to intercept network packets at the user-mode level:

1. **Packet Capture**: WinDivert captures all outbound network packets
2. **Delay Queue**: Packets are queued with a timestamp based on the configured delay
3. **Delayed Reinjection**: Packets are held in memory and reinjected after the delay period
4. **Statistics**: Real-time tracking of processed packets and bytes

## Project Structure

```
WindowsNetworkManager/
├── main.go                  # HTTP server and API endpoints
├── packet_delay.go          # WinDivert packet interception engine
├── service_wrapper.go       # Windows Service wrapper
├── upgrade.go               # Remote upgrade functionality
├── scanner/                 # Mac network scanner
│   ├── main.go              # Scanner CLI
│   ├── network_scanner.go   # Network scanning logic
│   ├── discovery.go         # Instance discovery
│   ├── browser.go           # Browser integration
│   ├── Makefile             # Build system
│   └── README.md            # Scanner documentation
├── web/
│   ├── index.html           # Web interface
│   └── static/
│       └── app.js           # Frontend JavaScript
├── go.mod                   # Go dependencies
├── build.sh                 # macOS build script (cross-compile)
├── build.bat                # Windows build script
├── install_service.bat      # Service installation script
├── uninstall_service.bat    # Service uninstallation script
└── README.md                # This file
```

## API Endpoints

- `GET /` - Web interface
- `GET /api/config` - Get current delay configuration
- `POST /api/config` - Set delay (JSON: `{"delay_ms": 100}`)
- `POST /api/start` - Start packet interception
- `POST /api/stop` - Stop packet interception
- `GET /api/stats` - Get network statistics
- `GET /api/network` - Get server's local IP addresses for network access
- `GET /api/discover` - Discovery endpoint for network scanners (returns instance info)
- `GET /api/upgrade/check` - Check for available updates
- `POST /api/upgrade` - Start upgrade process
- `GET /api/upgrade/status` - Get upgrade progress/status

## Technical Details

### WinDivert Integration

The application uses the `github.com/deblasis/godivert` library, which provides Go bindings for WinDivert. This library:
- Handles DLL loading automatically
- Provides a clean Go API for packet interception
- Supports filtering and packet manipulation

### Packet Processing

- Packets are captured using WinDivert's `Recv()` function
- A delay queue ensures packets are sent at the correct time
- Statistics are updated atomically for thread safety
- The engine gracefully handles errors and stop signals

### Performance Considerations

- Packet queue size is limited to 1000 packets to prevent memory issues
- If the queue is full, packets are sent immediately to avoid blocking
- Error handling includes retry logic and graceful degradation

## Remote Upgrade

The application supports remote upgrades via the web interface, allowing you to update without direct access to the Windows host.

### How to Upgrade

1. **Open the web interface** (http://localhost:18080 or from network)
2. **Click "Check for Updates"** in the Updates section
3. **If an update is available**, click "Upgrade Now"
4. **Monitor progress** - the upgrade will:
   - Download the new executable from GitHub releases
   - Stop the service automatically
   - Backup the current version
   - Install the new version
   - Restart the service

### Upgrade Process

The upgrade system:
- Checks GitHub releases for new versions
- Downloads the Windows executable automatically
- Creates a backup of the current version (`.exe.backup`)
- Stops the Windows service gracefully
- Replaces the executable
- Restarts the service automatically

### Configuration

By default, the upgrade system checks:
- **GitHub Releases API**: `https://api.github.com/repos/hihanifm/WindowsNetworkManager/releases/latest`
- Looks for assets ending in `.exe` containing "WindowsNetworkManager"

### Safety Features

- **Automatic backup** before upgrade
- **Service restart** after successful upgrade
- **Progress tracking** with real-time status
- **Error handling** with rollback capability
- **HTTPS only** for secure downloads

### Requirements

- Application must be running as a Windows Service for automatic restart
- Administrator privileges required for service management
- Internet connection for downloading updates

## Troubleshooting

### "Failed to open WinDivert handle"

- Ensure you're running as Administrator
- Verify WinDivert DLL is in the same directory as the executable
- Check that WinDivert driver is properly installed

### "Access Denied" errors

- The application MUST run with Administrator privileges
- Right-click the executable and select "Run as Administrator"
- When installing as a service, ensure you run the install script as Administrator

### Service won't start

- Verify the service was installed correctly: `sc query WindowsNetworkManager`
- Check Windows Event Viewer for error messages
- Ensure WinDivert DLL is in the same directory as the executable
- Try starting the service manually: `net start WindowsNetworkManager`

### Packets not being delayed

- Verify the delay is set to a value > 0
- Check that packet interception is started (status should show "Running")
- Review console logs for error messages

### Cannot access web interface from other devices

- **Check Windows Firewall:** Ensure port 18080 is allowed for incoming connections
- **Verify IP address:** Make sure you're using the correct IP address (check console output or `/api/network` endpoint)
- **Network connectivity:** Ensure devices are on the same network (same WiFi/LAN)
- **Firewall on other devices:** Check if other devices' firewalls are blocking the connection
- **Try localhost first:** Verify the interface works at `http://localhost:18080` on the host PC

## Security Notes

⚠️ **Important Security Considerations:**

- This application requires Administrator privileges to intercept network packets
- Only use in controlled environments for testing/development
- Intercepting network traffic may be subject to legal restrictions in some jurisdictions
- Do not use on production systems or networks without proper authorization
- The application modifies network traffic, which could affect system security

## Limitations

- Only intercepts **outbound** packets (packets leaving the system)
- Maximum delay is limited to 10 seconds (10000ms) for safety
- Packet queue is limited to 1000 packets
- Performance may degrade with very high packet rates

## License

MIT License - Use at your own risk

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## Acknowledgments

- [WinDivert](https://www.reqrypt.org/windivert.html) - Windows packet capture library
- [godivert](https://github.com/deblasis/godivert) - Go bindings for WinDivert

