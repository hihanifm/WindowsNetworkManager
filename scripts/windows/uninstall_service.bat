@echo off
echo ========================================
echo Uninstalling Windows Network Manager Service
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

echo Stopping service if running...
net stop WindowsNetworkManager >nul 2>&1

echo Uninstalling service...
WindowsNetworkManager.exe -service uninstall

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Service uninstalled successfully!
    echo ========================================
    echo.
) else (
    echo.
    echo ========================================
    echo Service uninstallation failed!
    echo ========================================
    pause
    exit /b 1
)

pause

