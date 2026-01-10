@echo off
echo ========================================
echo Configuring Windows Firewall for Network Access
echo Port 18080 - Windows Network Manager
echo ========================================
echo.

REM Check for administrator privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo.
    echo Right-click on this file and select "Run as Administrator"
    echo Or run Command Prompt as Administrator and execute this script
    echo.
    pause
    exit /b 1
)

echo Checking if firewall rule already exists...
netsh advfirewall firewall show rule name="Windows Network Manager" >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Rule already exists. Removing old rule...
    netsh advfirewall firewall delete rule name="Windows Network Manager" >nul 2>&1
)

echo.
echo Adding Windows Firewall rule for port 18080...
echo This will allow incoming connections on TCP port 18080
echo.

REM Add inbound rule for port 18080 - applies to all profiles (Domain, Private, Public)
netsh advfirewall firewall add rule name="Windows Network Manager" dir=in action=allow protocol=TCP localport=18080 profile=any enable=yes

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Firewall rule added successfully!
    echo ========================================
    echo.
    echo Port 18080 is now open for incoming connections.
    echo The web interface can now be accessed from:
    echo   - Local: http://localhost:18080
    echo   - Network: http://^<your-ip^>:18080 (from other devices)
    echo.
    echo Rule applies to: Domain, Private, and Public networks
    echo.
    echo To verify the rule:
    echo   1. Open Windows Defender Firewall
    echo   2. Click "Advanced Settings"
    echo   3. Go to "Inbound Rules"
    echo   4. Look for "Windows Network Manager"
    echo.
    echo Testing the rule...
    netsh advfirewall firewall show rule name="Windows Network Manager"
    echo.
) else (
    echo.
    echo ========================================
    echo Failed to add firewall rule!
    echo ========================================
    echo.
    echo Try these steps manually:
    echo   1. Open Windows Defender Firewall
    echo   2. Click "Advanced Settings"
    echo   3. Inbound Rules -^> New Rule
    echo   4. Rule Type: Port -^> Next
    echo   5. Protocol: TCP, Specific local ports: 18080 -^> Next
    echo   6. Action: Allow the connection -^> Next
    echo   7. Profile: Check all (Domain, Private, Public) -^> Next
    echo   8. Name: "Windows Network Manager" -^> Finish
    echo.
    pause
    exit /b 1
)

echo Configuration complete!
pause

