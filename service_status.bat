@echo off
setlocal enabledelayedexpansion

echo ========================================
echo Windows Network Manager Service Status
echo ========================================
echo.

set SERVICE_NAME=WindowsNetworkManager
set EXE_NAME=WindowsNetworkManager.exe

:: Check if executable path was provided as argument
if not "%~1"=="" (
    set "EXE_PATH=%~1"
    echo Using provided executable path: !EXE_PATH!
    :: Extract directory from provided path
    for %%f in ("!EXE_PATH!") do (
        set "EXE_DIR=%%~dpf"
        :: Remove trailing backslash
        if "!EXE_DIR:~-1!"=="\" set "EXE_DIR=!EXE_DIR:~0,-1!"
    )
    echo Extracted directory: [!EXE_DIR!]
    echo.
    set PATH_PROVIDED=1
)

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
    ) else (
        if "%%a"=="STOPPED" (
            echo Status: Service is currently STOPPED
        ) else (
            echo Status: Service state is %%a
        )
    )
)
echo.

:: 4. Check if executable exists
echo [4] Executable Check:
echo ----------------------------------------
if defined PATH_PROVIDED (
    echo Executable path was provided as argument
    echo Service executable path: !EXE_PATH!
    if exist "!EXE_PATH!" (
        echo Executable exists: YES
        for %%b in ("!EXE_PATH!") do (
            echo File size: %%~zb bytes
            echo Modified: %%~tb
        )
    ) else (
        echo Executable exists: NO - ERROR!
        echo Checking path: !EXE_PATH!
    )
) else (
    set EXE_PATH=
    set EXE_DIR=
    echo Debug: Querying service configuration...
sc qc %SERVICE_NAME% 2>nul | findstr "BINARY_PATH_NAME" >nul
if %errorLevel% neq 0 (
    echo ERROR: Could not find BINARY_PATH_NAME in service configuration
    echo Service may not be installed
    echo.
    echo Full service configuration:
    sc qc %SERVICE_NAME% 2>nul
) else (
    echo Debug: BINARY_PATH_NAME found, parsing...
    for /f "tokens=*" %%a in ('sc qc %SERVICE_NAME% 2^>nul ^| findstr "BINARY_PATH_NAME"') do (
        echo Raw line: [%%a]
        :: Try different parsing methods
        for /f "tokens=2 delims=:" %%b in ("%%a") do (
            set "EXE_PATH=%%b"
            set "EXE_PATH=!EXE_PATH: =!"
            echo Parsed path (method 1): [!EXE_PATH!]
        )
        :: Alternative: tokens=2 delims==
        for /f "tokens=2 delims==" %%c in ("%%a") do (
            if "!EXE_PATH!"=="" (
                set "EXE_PATH=%%c"
                set "EXE_PATH=!EXE_PATH: =!"
                echo Parsed path (method 2): [!EXE_PATH!]
            )
        )
        :: Remove quotes if present
        set "EXE_PATH=!EXE_PATH:"=!"
        echo Final path: [!EXE_PATH!]
    )
    if defined EXE_PATH (
        echo Service executable path: !EXE_PATH!
        :: Extract directory from path
        for %%f in ("!EXE_PATH!") do (
            set "EXE_DIR=%%~dpf"
            :: Remove trailing backslash
            if "!EXE_DIR:~-1!"=="\" set "EXE_DIR=!EXE_DIR:~0,-1!"
        )
        echo Extracted directory: [!EXE_DIR!]
        if exist "!EXE_PATH!" (
            echo Executable exists: YES
            for %%b in ("!EXE_PATH!") do (
                echo File size: %%~zb bytes
                echo Modified: %%~tb
            )
        ) else (
            echo Executable exists: NO - ERROR!
            echo Checking path: !EXE_PATH!
        )
    ) else (
        echo ERROR: Could not extract executable path from BINARY_PATH_NAME
        echo Raw service query output:
        sc qc %SERVICE_NAME% 2>nul | findstr "BINARY_PATH_NAME"
        echo.
        echo Usage: service_status.bat [executable_path]
        echo Example: service_status.bat "C:\Program Files\WindowsNetworkManager\WindowsNetworkManager.exe"
    )
)
)
echo.

:: 5. Check if DLL exists
echo [5] WinDivert.dll Check:
echo ----------------------------------------
set DLL_FOUND=0
if defined EXE_DIR (
    echo Service executable directory: !EXE_DIR!
    set "DLL_PATH=!EXE_DIR!\WinDivert.dll"
    echo Checking for DLL at: !DLL_PATH!
    if exist "!DLL_PATH!" (
        echo WinDivert.dll found: YES
        echo Location: !DLL_PATH!
        for %%c in ("!DLL_PATH!") do (
            echo File size: %%~zc bytes
            echo Modified: %%~tc
        )
        set DLL_FOUND=1
    ) else (
        echo WinDivert.dll found: NO - ERROR!
        echo Expected location: !DLL_PATH!
        echo.
        echo Debugging info:
        echo   EXE_DIR variable: [!EXE_DIR!]
        echo   DLL_PATH variable: [!DLL_PATH!]
        echo   EXE_DIR exists check:
        if exist "!EXE_DIR!" (
            echo     Directory exists: YES
            echo   Directory contents:
            dir /B "!EXE_DIR!" 2>nul
            echo.
            echo   Looking for DLL files:
            dir /B "!EXE_DIR!\*.dll" 2>nul
        ) else (
            echo     Directory exists: NO
        )
    )
) else (
    echo ERROR: Could not determine executable directory
    echo Service may not be installed or BINARY_PATH_NAME not found
    echo.
    echo Debug: Service query output:
    sc qc %SERVICE_NAME% 2>nul | findstr "BINARY_PATH_NAME"
    echo.
    echo Raw BINARY_PATH_NAME value:
    for /f "tokens=2* delims==" %%a in ('sc qc %SERVICE_NAME% 2^>nul ^| findstr "BINARY_PATH_NAME"') do (
        echo [%%a]
        if "%%b" neq "" echo [%%b]
    )
)
if !DLL_FOUND! equ 0 (
    if not defined EXE_DIR (
        echo.
        echo Service may not be installed or BINARY_PATH_NAME not found
    )
)
echo.

:: 6. Check if web files exist
echo [6] Web Files Check:
echo ----------------------------------------
if defined EXE_DIR (
    set "WEB_INDEX=!EXE_DIR!web\index.html"
    set "WEB_JS=!EXE_DIR!web\static\app.js"
    if exist "!WEB_INDEX!" (
        echo web\index.html: EXISTS
        for %%f in ("!WEB_INDEX!") do echo   Size: %%~zf bytes
    ) else (
        echo web\index.html: MISSING - ERROR!
        echo   Expected: !WEB_INDEX!
    )
    if exist "!WEB_JS!" (
        echo web\static\app.js: EXISTS
        for %%f in ("!WEB_JS!") do echo   Size: %%~zf bytes
    ) else (
        echo web\static\app.js: MISSING - ERROR!
        echo   Expected: !WEB_JS!
    )
) else (
    echo Cannot check web files - executable directory not determined
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
for /f "tokens=3" %%a in ('sc qc %SERVICE_NAME% 2^>nul ^| findstr "START_TYPE"') do (
    set START_TYPE=%%a
    echo Startup Type: %%a
    if "%%a"=="AUTO_START" (
        echo Service will start automatically on boot
    ) else (
        if "%%a"=="DEMAND_START" (
            echo Service must be started manually
        ) else (
            if "%%a"=="DISABLED" (
                echo Service is DISABLED
            )
        )
    )
)
echo.

:: 12. Service Account
echo [12] Service Account:
echo ----------------------------------------
set SERVICE_ACCOUNT=
set IS_ADMIN=0
set ACCOUNT_FOUND=0
for /f "tokens=2 delims==" %%a in ('sc qc %SERVICE_NAME% 2^>nul ^| findstr "SERVICE_START_NAME"') do (
    set "SERVICE_ACCOUNT=%%a"
    set ACCOUNT_FOUND=1
    echo Service runs as: %%a
    set "ACCT=%%a"
    if "!ACCT!"=="LocalSystem" (
        echo Privilege Level: Administrator (LocalSystem has full admin rights)
        set IS_ADMIN=1
    )
    if "!ACCT!"=="NT AUTHORITY\SYSTEM" (
        echo Privilege Level: Administrator (SYSTEM account has full admin rights)
        set IS_ADMIN=1
    )
    if "!ACCT!"=="" (
        echo Privilege Level: Unknown
        set IS_ADMIN=0
    )
    if not "!ACCT!"=="LocalSystem" if not "!ACCT!"=="NT AUTHORITY\SYSTEM" if not "!ACCT!"=="" (
        echo Privilege Level: Checking if account has admin rights...
        set IS_ADMIN=0
    )
)
if !ACCOUNT_FOUND! equ 0 (
    echo Service account information not available
    echo Service may not be installed
)
echo.

:: 13. Check Process Elevation (if process is running)
echo [13] Process Elevation Check:
echo ----------------------------------------
if "!STATE!"=="RUNNING" (
    echo Checking if process is running with elevated privileges...
    powershell -Command "$proc = Get-Process -Name '%EXE_NAME%' -ErrorAction SilentlyContinue; if ($proc) { $isElevated = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator); $procInfo = Get-WmiObject Win32_Process -Filter \"ProcessId = $($proc.Id)\"; $owner = $procInfo.GetOwner(); Write-Host \"Process Owner: $($owner.Domain)\$($owner.User)\"; if ($isElevated -or $owner.User -eq 'SYSTEM' -or $owner.User -eq 'LOCAL SERVICE' -or $owner.User -eq 'NETWORK SERVICE') { Write-Host 'Privilege Level: Administrator/Elevated' } else { Write-Host 'Privilege Level: Standard User' } } else { Write-Host 'Process not running' }" 2>nul
    if %errorLevel% neq 0 (
        echo Using alternative method to check privileges...
        for /f "tokens=*" %%p in ('tasklist /FI "IMAGENAME eq %EXE_NAME%" /V /FO CSV ^| findstr /I "%EXE_NAME%"') do (
            echo Process found, checking account...
            echo Note: Full privilege check requires PowerShell
        )
    )
) else (
    echo Process is not running - cannot check elevation
)
echo.

:: Summary
echo ========================================
echo Summary:
echo ========================================
if "!STATE!"=="RUNNING" (
    echo [OK] Service is RUNNING
) else (
    if "!STATE!"=="STOPPED" (
        echo [STOPPED] Service is STOPPED
    ) else (
        echo [UNKNOWN] Service status is !STATE!
    )
)

:: Admin Privilege Summary
echo.
echo Admin Privilege Status:
echo ----------------------------------------
if defined IS_ADMIN (
    if !IS_ADMIN! equ 1 (
        echo [OK] Service is running with Administrator privileges
        echo       This is REQUIRED for WinDivert to function
    ) else (
        echo [WARNING] Service may not have Administrator privileges
        echo           WinDivert requires Administrator privileges to work
    )
) else (
    echo [INFO] Check service account above for privilege level
    if defined SERVICE_ACCOUNT (
        if "!SERVICE_ACCOUNT!"=="LocalSystem" (
            echo [OK] LocalSystem account has full admin rights
        ) else (
            if "!SERVICE_ACCOUNT!"=="NT AUTHORITY\SYSTEM" (
                echo [OK] SYSTEM account has full admin rights
            ) else (
                echo [CHECK] Verify account !SERVICE_ACCOUNT! has admin rights
            )
        )
    )
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
