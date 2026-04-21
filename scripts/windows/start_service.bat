@echo off
echo ========================================
echo Starting Windows Push Notification Service
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

echo Starting service...
net start "Windows Push Notification Service"

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Service started successfully!
    echo ========================================
    echo.
    echo The service is now running.
    echo.
    echo Web interface is available at:
    echo   http://localhost:18080
    echo.
    echo To check service status, run:
    echo   sc query "Windows Push Notification Service"
    echo.
) else (
    echo.
    echo ========================================
    echo Service start failed!
    echo ========================================
    echo.
    echo Possible reasons:
    echo - Service is not installed (run install_service.bat first)
    echo - Service is already running
    echo - Service encountered an error
    echo.
    echo To check service status, run:
    echo   sc query "Windows Push Notification Service"
    echo.
    echo To view service logs, check Event Viewer:
    echo   eventvwr.msc
    echo.
    pause
    exit /b 1
)

pause
