# Windows Network Manager - Mac Scanner

A Mac companion tool to automatically discover Windows Network Manager instances running on your local network.

## Web Interface & Background Service

The scanner includes a web interface that can run as a macOS background service, accessible from any device on your network.

### Quick Start (Web Interface)

```bash
# Run web server
./wnm-scanner -web

# Or install as background service (auto-starts on boot)
./install_service.sh
```

Access the web interface at: **http://localhost:18081** (or from network: **http://<mac-ip>:18081**)

## Quick Start

### Build

```bash
./build.sh
# Or: make build
```

### Usage

```bash
# Scan network for instances
./wnm-scanner scan

# Open instance in browser
./wnm-scanner open 192.168.1.100
```

## Commands

### `scan` - Scan Network

Scans the local network for Windows Network Manager instances.

```bash
./wnm-scanner scan
./wnm-scanner scan -workers 50 -timeout 1s
./wnm-scanner scan -json
```

**Options:**
- `-workers int` - Number of parallel workers (default: 30)
- `-timeout duration` - Timeout per IP check (default: 2s)
- `-json` - Output results in JSON format

### `list` - List Instances

Alias for `scan` - shows discovered instances in a table format.

```bash
./wnm-scanner list
```

### `open <ip>` - Open in Browser

Opens the web interface for a specific instance in your default browser.

```bash
./wnm-scanner open 192.168.1.100
```

## How It Works

1. **Network Detection**: Automatically detects your local subnet (e.g., 192.168.1.0/24)
2. **Parallel Scanning**: Scans IP addresses in parallel using goroutines
3. **Instance Verification**: Checks each IP on port 18080 and verifies it's WindowsNetworkManager via `/api/discover`
4. **Results Display**: Shows discovered instances with their status and configuration

## Performance

- **Scan Time**: Typically 5-15 seconds for a /24 subnet (254 IPs)
- **Parallel Workers**: Default 30, adjustable with `-workers` flag
- **Timeout**: Default 2 seconds per IP, adjustable with `-timeout` flag

## Installation

### Option 1: Build and Run Locally

```bash
cd scanner
./build.sh
./wnm-scanner scan
```

### Option 2: Install System-Wide

```bash
cd scanner
make install
wnm-scanner scan  # Now available from anywhere
```

## Example Output

```
Scanning network: 192.168.1.0/24
Using 30 workers with 2s timeout per IP...

Scanned: 254/254 | Found: 2

Found 2 instance(s):

IP Address         Status     Delay (ms) URL
─────────────────────────────────────────────────────────────
192.168.1.100      Running    100        http://192.168.1.100:18080
192.168.1.105      Stopped    0          http://192.168.1.105:18080
```

## JSON Output

```bash
./wnm-scanner scan -json
```

```json
[
  {
    "ip": "192.168.1.100",
    "port": 18080,
    "service": "WindowsNetworkManager",
    "version": "2.2.0",
    "is_running": true,
    "delay_ms": 100,
    "local_ips": ["192.168.1.100"]
  }
]
```

## Web Interface Mode

### Running as Web Server

```bash
# Start web server (default port 18081)
./wnm-scanner -web

# Custom port
./wnm-scanner -web -port 8080
```

### Installing as macOS Background Service

```bash
# Install service (auto-starts on boot)
./install_service.sh

# Uninstall service
./uninstall_service.sh
```

**Service Features:**
- Runs continuously in background
- Auto-starts when Mac boots
- Auto-restarts if it crashes (keep-alive)
- Accessible from any device on network
- Logs to `~/Library/Logs/wnm-scanner/`

**Access Web Interface:**
- Local: http://localhost:18081
- Network: http://<mac-ip>:18081

**Check Service Status:**
```bash
launchctl list | grep com.windowsnetworkmanager.scanner
```

**View Logs:**
```bash
tail -f ~/Library/Logs/wnm-scanner/out.log
```

## Requirements

- macOS (for running the scanner)
- Go 1.21+ (for building)
- Network access to the subnet where Windows instances are running

## Troubleshooting

### No instances found
- Ensure Windows instances are running and accessible
- Check Windows Firewall allows port 18080
- Verify you're on the same network
- Try increasing timeout: `-timeout 5s`

### Scanner is slow
- Increase workers: `-workers 50`
- Decrease timeout: `-timeout 1s` (if network is fast)
- Note: Scanning 254 IPs will always take some time

### Network detection fails
- Ensure you're connected to a network
- Check network interface is up and has an IP address
- Scanner only works with IPv4 subnets (/24 or smaller)
