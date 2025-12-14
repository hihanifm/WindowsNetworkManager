@echo off
echo ========================================
echo Building Windows Network Manager
echo ========================================
echo.

echo Installing dependencies...
go mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to install dependencies
    pause
    exit /b 1
)

echo.
echo Building executable...
go build -o WindowsNetworkManager.exe
if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Build successful!
    echo ========================================
    echo.
    echo IMPORTANT: Run as Administrator:
    echo   WindowsNetworkManager.exe
    echo.
    echo The web interface will be available at:
    echo   http://localhost:18080
    echo.
) else (
    echo.
    echo ========================================
    echo Build failed!
    echo ========================================
    pause
    exit /b 1
)

pause

