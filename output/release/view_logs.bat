@echo off
echo ========================================
echo View Windows Network Manager Logs
echo ========================================
echo.

REM Check if running as Administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo WARNING: Some log viewing features require Administrator privileges
    echo Right-click and select "Run as Administrator" for full functionality
    echo.
)

echo [1] Windows Event Viewer (Recommended)
echo ----------------------------------------
echo Opening Event Viewer...
echo.
echo Logs are stored in Windows Event Log:
echo   - Application Log
echo   - Source: WindowsNetworkManager
echo.
echo To view logs:
echo   1. Event Viewer will open automatically
echo   2. Navigate to: Windows Logs ^> Application
echo   3. Filter by Source: WindowsNetworkManager
echo   4. Or search for "Windows Network Manager"
echo.
start eventvwr.msc
echo.
echo Press any key to continue...
pause >nul
echo.

echo [2] Recent Event Log Entries (PowerShell)
echo ----------------------------------------
powershell -Command "Get-EventLog -LogName Application -Source 'WindowsNetworkManager' -Newest 20 -ErrorAction SilentlyContinue | Format-Table TimeGenerated, EntryType, Message -AutoSize -Wrap"
if %errorLevel% neq 0 (
    echo No log entries found or PowerShell not available
    echo.
    echo Note: Logs are written to Windows Event Log when running as service
    echo       When running manually, logs appear in the console window
)
echo.

echo [3] Service Status and Recent Activity
echo ----------------------------------------
sc query "Windows Push Notification Service"
echo.
echo Recent service state changes:
wevtutil qe Application /c:10 /rd:true /f:text /q:"*[System[Provider[@Name='Service Control Manager'] and (EventID=7034 or EventID=7035 or EventID=7036) and TimeCreated[timediff(@SystemTime) <= 86400000]]]" 2>nul | findstr /I "WindowsNetworkManager"
echo.

echo [4] Log File Locations
echo ----------------------------------------
echo.
echo When running as SERVICE:
echo   Location: Windows Event Log
echo   Path: Not a file - stored in Windows Event Log database
echo   View: Event Viewer (eventvwr.msc)
echo   Source: WindowsNetworkManager
echo   Log: Application
echo.
echo When running MANUALLY (not as service):
echo   Location: Console output
echo   Path: Standard output (console window)
echo   View: Console window where you started the application
echo   Note: No log file is created in manual mode
echo.
echo To export logs from Event Viewer:
echo   1. Open Event Viewer
echo   2. Navigate to: Windows Logs ^> Application
echo   3. Filter by Source: WindowsNetworkManager
echo   4. Right-click ^> Save All Events As...
echo   5. Choose format (EVTX, XML, CSV, TXT)
echo.
echo ========================================
echo Quick Commands:
echo ========================================
echo.
echo View Event Viewer:
echo   eventvwr.msc
echo.
echo View recent logs (PowerShell):
echo   Get-EventLog -LogName Application -Source WindowsNetworkManager -Newest 20
echo.
echo Export logs to file (PowerShell):
echo   Get-EventLog -LogName Application -Source WindowsNetworkManager ^| Export-Csv logs.csv
echo.
pause
