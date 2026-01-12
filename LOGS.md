# Log Locations and Viewing Guide

This guide explains where Windows Network Manager stores its logs and how to view them.

## Log Storage Locations

### When Running as Windows Service

**Location:** Windows Event Log (not a file)

- **Log Name:** Application
- **Source:** WindowsNetworkManager
- **Path:** Windows Event Log database (system-managed)
- **Access:** Windows Event Viewer

The service uses Windows Event Log API to write logs. Logs are stored in the Windows Event Log database, not as individual files.

### When Running Manually (Not as Service)

**Location:** Console Output

- **Log Name:** Standard output (console window)
- **Path:** No file - logs appear in the console window
- **Access:** The console window where you started the application

When running manually, all logs are displayed in the console window. There is no log file created.

## Viewing Logs

### Method 1: Event Viewer (GUI) - Recommended

1. **Open Event Viewer:**
   - Press `Win + R`
   - Type `eventvwr.msc` and press Enter
   - Or search for "Event Viewer" in Start menu

2. **Navigate to Application Log:**
   - Expand "Windows Logs" in the left panel
   - Click on "Application"

3. **Filter by Source:**
   - In the right panel, click "Filter Current Log..."
   - Under "Event sources", check "Windows Network Manager"
   - Click OK

4. **View Logs:**
   - All logs from Windows Network Manager will be displayed
   - Double-click any entry to see full details

### Method 2: Using view_logs.bat Script

Run the provided script:
```cmd
view_logs.bat
```

This script will:
- Open Event Viewer automatically
- Show recent log entries in PowerShell
- Display service status
- Provide quick commands for log viewing

### Method 3: PowerShell Commands

**View Recent Logs:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager" -Newest 20
```

**View All Logs:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager"
```

**Export Logs to CSV:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager" | Export-Csv logs.csv
```

**Export Logs to Text File:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager" | Format-Table -AutoSize | Out-File logs.txt
```

### Method 4: Command Line (wevtutil)

**View Recent Logs:**
```cmd
wevtutil qe Application /c:20 /rd:true /f:text /q:"*[System[Provider[@Name='WindowsNetworkManager']]]"
```

**Export Logs:**
```cmd
wevtutil epl Application logs.evtx /q:"*[System[Provider[@Name='WindowsNetworkManager']]]"
```

## Log Entry Types

The service logs different types of entries:

- **Information:** Normal operation messages
  - Service starting/stopping
  - Web server starting
  - Network interface detection
  - Packet interception started/stopped

- **Warning:** Non-critical issues
  - Failed to initialize upgrade manager
  - Network interface detection issues

- **Error:** Critical errors
  - Service start failures
  - HTTP server errors
  - WinDivert initialization failures
  - Upgrade failures

## What Gets Logged

### Service Lifecycle
- Service start/stop events
- Service installation/uninstallation

### Application Events
- Web server startup
- Network interface detection
- Local IP addresses
- Port binding

### Packet Interception
- Packet interception started
- Packet interception stopped
- Delay configuration changes
- Statistics updates

### Errors and Warnings
- WinDivert initialization failures
- HTTP server errors
- Upgrade manager errors
- Network detection issues

## Log Retention

Windows Event Log has default retention policies:

- **Maximum Log Size:** Usually 20MB (configurable)
- **When Maximum Reached:** 
  - Overwrite events as needed (oldest first)
  - Archive log when full
  - Do not overwrite events

You can configure retention in Event Viewer:
1. Right-click "Application" log
2. Select "Properties"
3. Configure "Maximum log size" and "When maximum event log size is reached"

## Exporting Logs

### From Event Viewer

1. Open Event Viewer
2. Navigate to: Windows Logs → Application
3. Filter by Source: Windows Network Manager
4. Right-click → "Save All Events As..."
5. Choose format:
   - **EVTX** - Event Log format (recommended)
   - **XML** - XML format
   - **CSV** - Comma-separated values
   - **TXT** - Plain text

### From PowerShell

**Export to CSV:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager" | Export-Csv "C:\logs\wnm_logs.csv"
```

**Export to Text:**
```powershell
Get-EventLog -LogName Application -Source "WindowsNetworkManager" | Format-List | Out-File "C:\logs\wnm_logs.txt"
```

## Quick Reference

| Scenario | Log Location | How to View |
|----------|-------------|-------------|
| Running as Service | Windows Event Log | Event Viewer (eventvwr.msc) |
| Running Manually | Console Output | Console window |
| Need to Export | Event Viewer → Save As | Right-click → Save All Events As... |
| Quick View | PowerShell | `Get-EventLog -LogName Application -Source "WindowsNetworkManager"` |
| Script Helper | view_logs.bat | Run the batch script |

## Troubleshooting

### No Logs Appearing

1. **Check if service is running:**
   ```cmd
   sc query WindowsNetworkManager
   ```

2. **Check Event Viewer:**
   - Make sure you're looking at "Application" log
   - Check if "Windows Network Manager" appears in sources
   - Try clearing filters

3. **Check service installation:**
   - Service must be installed for Event Log entries
   - Manual runs don't create Event Log entries

### Logs Not Showing Recent Events

1. **Refresh Event Viewer:**
   - Press F5 to refresh
   - Or close and reopen Event Viewer

2. **Check log retention:**
   - Logs may have been overwritten if log is full
   - Increase log size in Event Viewer properties

3. **Check time range:**
   - Make sure you're not filtering by date incorrectly

## Notes

- **Service Mode:** All logs go to Windows Event Log
- **Manual Mode:** All logs go to console (no file)
- **No File Logs:** The application does not create log files on disk
- **Event Log Only:** When running as service, Event Log is the only log location
- **Console Only:** When running manually, console is the only log location
