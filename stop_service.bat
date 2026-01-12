@echo off
echo ========================================
echo Stopping Windows Network Manager Service
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

echo Stopping service...
net stop WindowsNetworkManager

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Service stopped successfully!
    echo ========================================
    echo.
    echo The service has been stopped.
    echo.
    echo To start the service again, run:
    echo   start_service.bat
    echo.
    echo Or use:
    echo   net start WindowsNetworkManager
    echo.
) else (
    echo.
    echo ========================================
    echo Service stop failed!
    echo ========================================
    echo.
    echo Possible reasons:
    echo - Service is not installed
    echo - Service is already stopped
    echo - Service encountered an error
    echo.
    echo To check service status, run:
    echo   sc query WindowsNetworkManager
    echo.
    pause
    exit /b 1
)

pause
