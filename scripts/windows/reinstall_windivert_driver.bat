@echo off
setlocal enabledelayedexpansion
echo ========================================
echo WinDivert Driver Reinstallation Script
echo ========================================
echo.
echo This script will:
echo   1. Delete the existing WinDivert driver service
echo   2. Reinstall it with the correct driver file path
echo.

REM Check for administrator privileges
echo [DEBUG] Checking administrator privileges...
net session >nul 2>&1
set ADMIN_CHECK=%ERRORLEVEL%
if !ADMIN_CHECK! neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Right-click and select "Run as Administrator"
    pause
    exit /b 1
)
echo [OK] Administrator privileges confirmed
echo.

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "EXE_DIR=%SCRIPT_DIR%"
echo [DEBUG] Script directory: %SCRIPT_DIR%
echo [DEBUG] Executable directory: %EXE_DIR%
echo.

REM Find driver file
set "DRIVER_FILE="
set "DRIVER_NAME="

echo Checking for WinDivert driver files...
if exist "%EXE_DIR%WinDivert64.sys" (
    set "DRIVER_FILE=%EXE_DIR%WinDivert64.sys"
    set "DRIVER_NAME=WinDivert64"
    echo [OK] Found WinDivert64.sys
    goto :found_driver
)

if exist "%EXE_DIR%WinDivert32.sys" (
    set "DRIVER_FILE=%EXE_DIR%WinDivert32.sys"
    set "DRIVER_NAME=WinDivert32"
    echo [OK] Found WinDivert32.sys
    goto :found_driver
)

echo [ERROR] WinDivert driver file (.sys) is missing!
echo.
echo Please ensure WinDivert64.sys (or WinDivert32.sys) is in the same
echo directory as this script: %EXE_DIR%
echo.
echo Download from: https://www.reqrypt.org/windivert.html
echo.
pause
exit /b 1

:found_driver
echo.
echo Driver file: !DRIVER_FILE!
echo.

REM Check if service exists
echo Checking if WinDivert service exists...
sc query WinDivert >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [INFO] WinDivert service exists
    echo.
    echo Current service configuration:
    sc qc WinDivert
    echo.
    echo Stopping service if running...
    sc stop WinDivert >nul 2>&1
    timeout /t 2 /nobreak >nul
    echo.
    echo Deleting existing WinDivert service...
    sc delete WinDivert
    if %ERRORLEVEL% equ 0 (
        echo [OK] Service deleted successfully
        echo.
        timeout /t 2 /nobreak >nul
    ) else (
        echo [ERROR] Failed to delete service. Error code: %ERRORLEVEL%
        echo.
        echo You may need to:
        echo   1. Stop the service manually: sc stop WinDivert
        echo   2. Wait a few seconds
        echo   3. Try running this script again
        pause
        exit /b 1
    )
) else (
    echo [INFO] WinDivert service does not exist (will create new one)
    echo.
)

REM Create new service with correct path
echo Creating WinDivert driver service with path:
echo   !DRIVER_FILE!
echo.
sc create WinDivert type= kernel binPath= "!DRIVER_FILE!"
set CREATE_RESULT=%ERRORLEVEL%

if !CREATE_RESULT! neq 0 (
    echo [ERROR] Failed to create service. Error code: !CREATE_RESULT!
    echo.
    echo Common causes:
    echo   - Driver file path is incorrect
    echo   - Driver file is blocked or unsigned
    echo   - Insufficient privileges
    echo.
    pause
    exit /b 1
)

echo [OK] Service created successfully
echo.

REM Start the service
echo Starting WinDivert driver service...
sc start WinDivert
set START_RESULT=%ERRORLEVEL%

if !START_RESULT! equ 0 (
    echo [OK] Driver service started successfully
    echo.
    echo Checking service status...
    sc query WinDivert
    echo.
    echo ========================================
    echo Reinstallation complete!
    echo ========================================
    echo.
    echo The WinDivert driver service has been reinstalled and started.
    echo You can now run WindowsNetworkManager.exe
    echo.
) else (
    echo [WARNING] Service created but failed to start. Error code: !START_RESULT!
    echo.
    echo Checking service status...
    sc query WinDivert
    echo.
    echo The service may start automatically when WindowsNetworkManager.exe runs.
    echo If you continue to have issues:
    echo   1. Check Event Viewer for detailed error messages
    echo   2. Verify the driver file is not blocked (right-click - Properties - Unblock)
    echo   3. Check Windows Defender / Antivirus is not blocking the driver
    echo   4. Ensure the driver file matches your Windows architecture (x64/ARM64)
    echo.
)

pause
