# Development Guide (macOS)

This project is designed to be developed on macOS and cross-compiled for Windows.

## Quick Start

### Build for Windows

```bash
# Simple build script
./build.sh

# Or using Make
make build

# Or manually
GOOS=windows GOARCH=amd64 go build -o WindowsNetworkManager.exe
```

## Development Workflow

### 1. Edit Code on Mac
- Use your favorite editor (VS Code, Cursor, etc.)
- All Go code can be written and edited on macOS
- The code is cross-platform compatible

### 2. Build for Windows
```bash
./build.sh
```

This creates `WindowsNetworkManager.exe` that runs on Windows.

### 3. Test on Windows
- Copy `WindowsNetworkManager.exe` to a Windows PC
- Copy `WinDivert.dll` to the same directory
- Run as Administrator on Windows
- Test the web interface and packet interception

## Project Structure

```
WindowsNetworkManager/
├── main.go              # HTTP server and API endpoints
├── packet_delay.go      # WinDivert packet interception engine
├── service_wrapper.go   # Windows Service wrapper
├── web/                 # Web interface files
├── build.sh             # macOS build script (cross-compile)
├── Makefile             # Make targets for building
└── *.bat                # Windows scripts (for reference, not used on Mac)
```

## Build Scripts

### `build.sh` (macOS)
- Cross-compiles Go code to Windows
- Creates `WindowsNetworkManager.exe`
- Checks for Go installation
- Installs dependencies automatically

### `Makefile`
- `make build` - Build Windows executable
- `make clean` - Remove build artifacts
- `make install-deps` - Install Go dependencies
- `make test` - Run tests (if any)
- `make help` - Show available targets

## Testing

### Local Testing (macOS)
- You can test the HTTP server code locally (without WinDivert)
- The web interface HTML/JS can be tested in a browser
- Full packet interception requires Windows

### Windows Testing
- Copy built `.exe` to Windows
- Requires WinDivert DLL
- Must run as Administrator
- Test packet interception and service functionality

## Dependencies

All dependencies are managed via `go.mod`:
- `github.com/deblasis/godivert` - WinDivert Go bindings
- `github.com/kardianos/service` - Windows Service support

Install with:
```bash
go mod tidy
```

## Notes

- **WinDivert is Windows-only**: Cannot test packet interception on macOS
- **Windows Service**: Service functionality only works on Windows
- **Cross-compilation**: Go makes it easy to build Windows binaries from Mac
- **No Windows VM needed**: Just copy the `.exe` to a Windows PC for testing

## Troubleshooting

### Build fails with "cannot find package"
```bash
go mod tidy
go mod download
```

### Cross-compilation issues
Ensure you have the correct Go version:
```bash
go version  # Should be 1.21+
```

### Executable doesn't run on Windows
- Ensure `WinDivert.dll` is in the same directory
- Run as Administrator
- Check Windows Event Viewer for errors
