@echo off
echo ========================================
echo WinDivert Driver Installation Script
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

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "EXE_DIR=%SCRIPT_DIR%"

echo Checking for WinDivert driver files...
echo.

REM Check architecture
set "DRIVER_FILE="
if exist "%EXE_DIR%WinDivert64.sys" (
    set "DRIVER_FILE=%EXE_DIR%WinDivert64.sys"
    set "DRIVER_NAME=WinDivert64"
    echo [OK] Found WinDivert64.sys in executable directory
) else (
    if exist "%EXE_DIR%WinDivert32.sys" (
        set "DRIVER_FILE=%EXE_DIR%WinDivert32.sys"
        set "DRIVER_NAME=WinDivert32"
        echo [OK] Found WinDivert32.sys in executable directory
    ) else (
        echo [ERROR] WinDivert driver file (.sys) not found!
        echo.
        echo Please ensure WinDivert64.sys (or WinDivert32.sys) is in the same
        echo directory as WindowsNetworkManager.exe
        echo.
        echo Download from: https://www.reqrypt.org/windivert.html
        echo Extract and copy the .sys file to: %EXE_DIR%
        echo.
        pause
        exit /b 1
    )
)

echo.
echo Driver file: %DRIVER_FILE%
echo.

REM Check if driver is already installed
sc query WinDivert >nul 2>&1
if %errorLevel% equ 0 (
    echo [INFO] WinDivert driver service already exists
    echo.
    echo Checking driver status...
    sc query WinDivert
    echo.
    echo The driver should auto-load when WindowsNetworkManager.exe starts.
    echo If you're experiencing issues, try:
    echo   1. Restart WindowsNetworkManager.exe as Administrator
    echo   2. Check Event Viewer for driver errors
    echo   3. Verify the driver file is not blocked by Windows
    echo.
    pause
    exit /b 0
)

echo [INFO] WinDivert driver service not found
echo.
echo IMPORTANT: WinDivert driver typically auto-installs when you run
echo WindowsNetworkManager.exe as Administrator. The driver file just needs
echo to be in the same directory as the executable.
echo.
echo However, if you need to manually install the driver, you can use:
echo.
echo   sc create WinDivert type= kernel binPath= "%DRIVER_FILE%"
echo   sc start WinDivert
echo.
echo Do you want to manually install the driver now? (Y/N)
set /p INSTALL_CHOICE=

if /i "%INSTALL_CHOICE%"=="Y" (
    echo.
    echo Installing WinDivert driver...
    sc create WinDivert type= kernel binPath= "%DRIVER_FILE%"
    
    if %ERRORLEVEL% EQU 0 (
        echo [OK] Driver service created successfully
        echo.
        echo Starting driver...
        sc start WinDivert
        
        if %ERRORLEVEL% EQU 0 (
            echo [OK] Driver started successfully
            echo.
            echo ========================================
            echo Driver installation complete!
            echo ========================================
            echo.
            echo You can now run WindowsNetworkManager.exe
            echo.
        ) else (
            echo [WARNING] Driver service created but failed to start
            echo This might be normal - the driver may start automatically when needed
            echo.
        )
    ) else (
        echo [ERROR] Failed to create driver service
        echo Error code: %ERRORLEVEL%
        echo.
        echo Common causes:
        echo   - Driver file path is incorrect
        echo   - Driver file is blocked or unsigned
        echo   - Insufficient privileges (though you're running as Admin)
        echo.
        echo Try running WindowsNetworkManager.exe as Administrator instead.
        echo The driver should auto-install when the application starts.
        echo.
    )
) else (
    echo.
    echo Skipping manual installation.
    echo.
    echo The driver will auto-install when you run WindowsNetworkManager.exe
    echo as Administrator. Just ensure the .sys file is in the same directory.
    echo.
)

echo.
echo ========================================
echo Next Steps:
echo ========================================
echo 1. Ensure WinDivert.dll is in: %EXE_DIR%
echo 2. Ensure %DRIVER_NAME%.sys is in: %EXE_DIR%
echo 3. Run WindowsNetworkManager.exe as Administrator
echo 4. The driver should auto-load when the app starts
echo.
echo If you still get "invalid handle" errors:
echo   - Check Event Viewer for driver errors
echo   - Verify driver file is not blocked (right-click - Properties - Unblock)
echo   - Try restarting Windows
echo   - Check Windows Defender / Antivirus isn't blocking the driver
echo.

pause
