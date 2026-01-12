@echo off
REM Run Windows Network Manager in manual/console mode
REM This script runs the application directly (not as a service)
REM Console output will be visible in this window

echo ========================================
echo Windows Network Manager - Manual Mode
echo ========================================
echo.
echo This will run the application in console mode.
echo You will see all logs and output in this window.
echo.
echo To stop the application, press Ctrl+C
echo.
echo ========================================
echo.

REM Check if executable exists
if not exist "WindowsNetworkManager.exe" (
    echo ERROR: WindowsNetworkManager.exe not found!
    echo Please ensure you are running this script from the application directory.
    pause
    exit /b 1
)

REM Check if DLL exists
if not exist "WinDivert.dll" (
    echo ERROR: WinDivert.dll not found!
    echo Please ensure WinDivert.dll is in the same directory as WindowsNetworkManager.exe
    pause
    exit /b 1
)

REM Check for admin privileges (required for packet interception)
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo WARNING: Not running as Administrator!
    echo.
    echo The application requires Administrator privileges to intercept packets.
    echo You can still access the web interface, but packet interception may not work.
    echo.
    echo To run with admin privileges:
    echo   1. Right-click this script
    echo   2. Select "Run as Administrator"
    echo.
    timeout /t 5 /nobreak >nul
    echo.
)

echo Starting Windows Network Manager...
echo.
echo Web interface will be available at:
echo   http://localhost:18080
echo.
echo Press Ctrl+C to stop the application
echo.
echo ========================================
echo.

REM Run the application
WindowsNetworkManager.exe

REM If we get here, the application has exited
echo.
echo ========================================
echo Application stopped.
echo ========================================
pause
