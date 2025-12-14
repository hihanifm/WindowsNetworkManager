@echo off
echo ========================================
echo Installing Windows Network Manager Service
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

echo Installing service...
WindowsNetworkManager.exe -service install

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Service installed successfully!
    echo ========================================
    echo.
    echo The service will start automatically on boot.
    echo.
    echo To start the service now, run:
    echo   net start WindowsNetworkManager
    echo.
    echo Or use:
    echo   WindowsNetworkManager.exe -service start
    echo.
) else (
    echo.
    echo ========================================
    echo Service installation failed!
    echo ========================================
    pause
    exit /b 1
)

pause

