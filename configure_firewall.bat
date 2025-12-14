@echo off
echo ========================================
echo Configuring Windows Firewall for Network Access
echo ========================================
echo.

REM Check for administrator privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Right-click and select "Run as Administrator"
    pause
    exit /b 1
)

echo Adding Windows Firewall rule for port 18080...
echo.

REM Add inbound rule for port 18080
netsh advfirewall firewall add rule name="Windows Network Manager" dir=in action=allow protocol=TCP localport=18080

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Firewall rule added successfully!
    echo ========================================
    echo.
    echo Port 18080 is now open for incoming connections.
    echo The web interface can now be accessed from other devices on your network.
    echo.
    echo To verify, check Windows Firewall:
    echo   Windows Defender Firewall -^> Advanced Settings -^> Inbound Rules
    echo.
) else (
    echo.
    echo ========================================
    echo Failed to add firewall rule!
    echo ========================================
    echo.
    echo You may need to manually configure Windows Firewall:
    echo   1. Open Windows Defender Firewall
    echo   2. Advanced Settings
    echo   3. Inbound Rules -^> New Rule
    echo   4. Port -^> TCP -^> 18080 -^> Allow
    echo.
    pause
    exit /b 1
)

pause

