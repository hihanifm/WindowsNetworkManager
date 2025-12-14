# Windows Network Manager

A Windows application with a web interface that monitors network traffic and adds configurable latency to network packets using WinDivert (user-mode).

## Features

- 🌐 Web-based interface for easy configuration
- ⏱️ Configurable packet delay (0-10000ms)
- 📊 Real-time network statistics
- 🚀 User-mode implementation (no kernel driver required)
- ⚡ Start/Stop packet interception on demand
- 🔄 Dynamic delay adjustment while running
- 🔧 Windows Service support - runs automatically on boot

## Prerequisites

1. **Windows 10/11** (64-bit)
2. **Go 1.21+** - [Download](https://golang.org/dl/)
3. **WinDivert Library** - [Download](https://www.reqrypt.org/windivert.html)
4. **Administrator privileges** (required to run)

## Setup Instructions

### 1. Install Go

Download and install Go from the official website: https://golang.org/dl/

### 2. Download WinDivert

1. Download WinDivert from: https://www.reqrypt.org/windivert.html
2. Extract the archive
3. Copy the following files to your project directory:
   - `WinDivert.dll` (from `x64` folder for 64-bit Windows)
   - `WinDivert32.sys` or `WinDivert64.sys` (from `x64` folder)
   
   **Note:** The Go library (`github.com/deblasis/godivert`) will automatically load the DLL, but you need to ensure it's in the same directory as the executable or in your system PATH.

### 3. Build the Project

```bash
# Install dependencies
go mod tidy

# Build (on Windows)
go build -o WindowsNetworkManager.exe
```

Or use the provided build script:

```bash
build.bat
```

### 4. Run the Application

**IMPORTANT:** Run as Administrator!

1. Right-click Command Prompt or PowerShell
2. Select "Run as Administrator"
3. Navigate to the project directory
4. Run: `.\WindowsNetworkManager.exe`

The web interface will be available at: **http://localhost:8080**

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

**Note:** The service runs in the background and starts automatically on boot. The web interface remains available at http://localhost:8080 even when running as a service.

## Usage

1. **Open the web interface** in your browser: http://localhost:8080
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
├── web/
│   ├── index.html           # Web interface
│   └── static/
│       └── app.js           # Frontend JavaScript
├── go.mod                   # Go dependencies
├── build.bat                # Build script
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

