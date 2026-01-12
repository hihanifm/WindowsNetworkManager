@echo off
setlocal enabledelayedexpansion

echo ========================================
echo Windows Network Manager Service Status
echo ========================================
echo.

set SERVICE_NAME=WindowsNetworkManager
set EXE_NAME=WindowsNetworkManager.exe

:: Check if running as Administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo WARNING: Some commands require Administrator privileges
    echo Right-click and select "Run as Administrator" for full functionality
    echo.
)

:: 1. Service Query
echo [1] Service Status (sc query):
echo ----------------------------------------
sc query %SERVICE_NAME% 2>nul
if %errorLevel% neq 0 (
    echo Service %SERVICE_NAME% is NOT installed
    echo.
) else (
    echo.
)

:: 2. Service Configuration
echo [2] Service Configuration (sc qc):
echo ----------------------------------------
sc qc %SERVICE_NAME% 2>nul
if %errorLevel% neq 0 (
    echo Service %SERVICE_NAME% is NOT installed
    echo.
) else (
    echo.
)

:: 3. Service State
echo [3] Service State:
echo ----------------------------------------
for /f "tokens=3" %%a in ('sc query %SERVICE_NAME% ^| findstr "STATE"') do (
    set STATE=%%a
    echo State: %%a
    if "%%a"=="RUNNING" (
        echo Status: Service is currently RUNNING
    ) else if "%%a"=="STOPPED" (
        echo Status: Service is currently STOPPED
    ) else (
        echo Status: Service state is %%a
    )
)
echo.

:: 4. Check if executable exists
echo [4] Executable Check:
echo ----------------------------------------
for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% ^| findstr "BINARY_PATH_NAME"') do (
    set EXE_PATH=%%a
    echo Service executable path: %%a
    if exist "%%a" (
        echo Executable exists: YES
        for %%b in ("%%a") do (
            echo File size: %%~zb bytes
            echo Modified: %%~tb
        )
    ) else (
        echo Executable exists: NO - ERROR!
    )
)
echo.

:: 5. Check if DLL exists
echo [5] WinDivert.dll Check:
echo ----------------------------------------
for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% ^| findstr "BINARY_PATH_NAME"') do (
    for %%b in ("%%a") do set EXE_DIR=%%~dpb
    if exist "!EXE_DIR!WinDivert.dll" (
        echo WinDivert.dll found: YES
        echo Location: !EXE_DIR!WinDivert.dll
        for %%c in ("!EXE_DIR!WinDivert.dll") do (
            echo File size: %%~zc bytes
        )
    ) else (
        echo WinDivert.dll found: NO - ERROR!
        echo Expected location: !EXE_DIR!WinDivert.dll
    )
)
echo.

:: 6. Check if web files exist
echo [6] Web Files Check:
echo ----------------------------------------
for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% ^| findstr "BINARY_PATH_NAME"') do (
    for %%b in ("%%a") do set EXE_DIR=%%~dpb
    if exist "!EXE_DIR!web\index.html" (
        echo web\index.html: EXISTS
    ) else (
        echo web\index.html: MISSING - ERROR!
    )
    if exist "!EXE_DIR!web\static\app.js" (
        echo web\static\app.js: EXISTS
    ) else (
        echo web\static\app.js: MISSING - ERROR!
    )
)
echo.

:: 7. Port Check
echo [7] Port 18080 Status:
echo ----------------------------------------
netstat -ano | findstr ":18080" >nul 2>&1
if %errorLevel% equ 0 (
    echo Port 18080 is LISTENING
    echo Connections:
    netstat -ano | findstr ":18080"
) else (
    echo Port 18080 is NOT listening
    if "!STATE!"=="RUNNING" (
        echo WARNING: Service is running but port is not listening!
    )
)
echo.

:: 8. Process Check
echo [8] Process Check:
echo ----------------------------------------
tasklist /FI "IMAGENAME eq %EXE_NAME%" 2>nul | findstr /I "%EXE_NAME%" >nul
if %errorLevel% equ 0 (
    echo Process %EXE_NAME% is RUNNING
    echo Process details:
    tasklist /FI "IMAGENAME eq %EXE_NAME%" /FO LIST
) else (
    echo Process %EXE_NAME% is NOT running
    if "!STATE!"=="RUNNING" (
        echo WARNING: Service shows as RUNNING but process not found!
    )
)
echo.

:: 9. Event Log Check (last 5 entries)
echo [9] Recent Event Log Entries:
echo ----------------------------------------
echo Checking Windows Event Log for %SERVICE_NAME%...
powershell -Command "Get-EventLog -LogName Application -Source '%SERVICE_NAME%' -Newest 5 -ErrorAction SilentlyContinue | Format-Table TimeGenerated, EntryType, Message -AutoSize" 2>nul
if %errorLevel% neq 0 (
    echo No recent event log entries found (or PowerShell not available)
)
echo.

:: 10. Service Dependencies
echo [10] Service Dependencies:
echo ----------------------------------------
sc enumdepend %SERVICE_NAME% 2>nul
if %errorLevel% neq 0 (
    echo No dependencies found or service not installed
)
echo.

:: 11. Startup Type
echo [11] Startup Type:
echo ----------------------------------------
for /f "tokens=3" %%a in ('sc qc %SERVICE_NAME% ^| findstr "START_TYPE"') do (
    set START_TYPE=%%a
    echo Startup Type: %%a
    if "%%a"=="AUTO_START" (
        echo Service will start automatically on boot
    ) else if "%%a"=="DEMAND_START" (
        echo Service must be started manually
    ) else if "%%a"=="DISABLED" (
        echo Service is DISABLED
    )
)
echo.

:: 12. Service Account
echo [12] Service Account:
echo ----------------------------------------
for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% ^| findstr "SERVICE_START_NAME"') do (
    echo Service runs as: %%a
)
echo.

:: Summary
echo ========================================
echo Summary:
echo ========================================
if "!STATE!"=="RUNNING" (
    echo [OK] Service is RUNNING
) else if "!STATE!"=="STOPPED" (
    echo [STOPPED] Service is STOPPED
) else (
    echo [UNKNOWN] Service status is !STATE!
)

:: Check for common issues
echo.
echo Common Issues Check:
echo ----------------------------------------
set ISSUES=0

for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% ^| findstr "BINARY_PATH_NAME"') do (
    for %%b in ("%%a") do set EXE_DIR=%%~dpb
    if not exist "%%a" (
        echo [ERROR] Executable not found at: %%a
        set /a ISSUES+=1
    )
    if not exist "!EXE_DIR!WinDivert.dll" (
        echo [ERROR] WinDivert.dll not found in: !EXE_DIR!
        set /a ISSUES+=1
    )
    if not exist "!EXE_DIR!web\index.html" (
        echo [ERROR] web\index.html not found in: !EXE_DIR!
        set /a ISSUES+=1
    )
)

netstat -ano | findstr ":18080" >nul 2>&1
if %errorLevel% neq 0 (
    if "!STATE!"=="RUNNING" (
        echo [WARNING] Service is running but port 18080 is not listening
        set /a ISSUES+=1
    )
)

if %ISSUES% equ 0 (
    echo [OK] No issues detected
) else (
    echo [ISSUES FOUND] %ISSUES% issue(s) detected
)

echo.
echo ========================================
echo Quick Commands:
echo ========================================
echo Start service:   net start %SERVICE_NAME%
echo Stop service:    net stop %SERVICE_NAME%
echo Restart service: net stop %SERVICE_NAME% ^&^& net start %SERVICE_NAME%
echo.
pause
