@echo off
echo ========================================
echo Check Administrator Privileges
echo ========================================
echo.

:: Method 1: Check current process elevation
echo [1] Current Process (this script):
echo ----------------------------------------
net session >nul 2>&1
if %errorLevel% equ 0 (
    echo Status: Running with Administrator privileges
    echo This script has admin rights
) else (
    echo Status: Running WITHOUT Administrator privileges
    echo Right-click and select "Run as Administrator" for full functionality
)
echo.

:: Method 2: Check Windows Network Manager service account
echo [2] Windows Network Manager Service:
echo ----------------------------------------
set SERVICE_NAME=Windows Push Notification Service
sc query "%SERVICE_NAME%" >nul 2>&1
if %errorLevel% equ 0 (
    for /f "tokens=2 delims==" %%a in ('sc qc "%SERVICE_NAME%" ^| findstr "SERVICE_START_NAME"') do (
        set ACCOUNT=%%a
        echo Service Account: %%a
        if "%%a"=="LocalSystem" (
            echo Privilege Level: Administrator (LocalSystem = full admin)
            echo Status: [OK] Service has Administrator privileges
        ) else if "%%a"=="NT AUTHORITY\SYSTEM" (
            echo Privilege Level: Administrator (SYSTEM = full admin)
            echo Status: [OK] Service has Administrator privileges
        ) else if "%%a"=="" (
            echo Privilege Level: Unknown
            echo Status: [UNKNOWN] Could not determine account
        ) else (
            echo Privilege Level: User Account
            echo Status: [CHECK] Verify account has admin rights
            echo Note: WinDivert requires Administrator privileges
        )
    )
) else (
    echo Service is not installed
)
echo.

:: Method 3: Check if WindowsNetworkManager.exe process is elevated
echo [3] WindowsNetworkManager.exe Process:
echo ----------------------------------------
tasklist /FI "IMAGENAME eq WindowsNetworkManager.exe" 2>nul | findstr /I "WindowsNetworkManager.exe" >nul
if %errorLevel% equ 0 (
    echo Process is running
    echo Checking elevation...
    powershell -Command "$proc = Get-Process -Name 'WindowsNetworkManager' -ErrorAction SilentlyContinue; if ($proc) { try { $procInfo = Get-WmiObject Win32_Process -Filter \"ProcessId = $($proc.Id)\"; $owner = $procInfo.GetOwner(); Write-Host \"Process Owner: $($owner.Domain)\$($owner.User)\"; if ($owner.User -eq 'SYSTEM' -or $owner.User -eq 'LOCAL SERVICE' -or $owner.User -eq 'NETWORK SERVICE' -or $owner.Domain -eq 'NT AUTHORITY') { Write-Host 'Privilege Level: Administrator/Elevated' } else { $identity = [Security.Principal.WindowsIdentity]::GetCurrent(); $principal = New-Object Security.Principal.WindowsPrincipal $identity; $isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator); if ($isAdmin) { Write-Host 'Privilege Level: Administrator' } else { Write-Host 'Privilege Level: Standard User - WARNING!' } } } catch { Write-Host 'Could not determine privilege level' } } else { Write-Host 'Process not running' }" 2>nul
    if %errorLevel% neq 0 (
        echo Note: Full privilege check requires PowerShell
        echo Process is running - check service account above for privilege level
    )
) else (
    echo Process is not running
)
echo.

:: Method 4: Quick test - Try to access admin-only resource
echo [4] Quick Admin Test:
echo ----------------------------------------
echo Testing access to admin-only resource...
dir "%SystemRoot%\System32\config\sam" >nul 2>&1
if %errorLevel% equ 0 (
    echo Status: Has access to admin-only resource
    echo This indicates Administrator privileges
) else (
    echo Status: Cannot access admin-only resource
    echo This indicates standard user privileges
)
echo.

:: Summary
echo ========================================
echo Summary:
echo ========================================
echo.
echo For WinDivert to work:
echo - Service MUST run with Administrator privileges
echo - Recommended: Use LocalSystem account (default for services)
echo - Alternative: Use an account with Administrator rights
echo.
echo If service is not running with admin privileges:
echo 1. Stop the service: net stop "Windows Push Notification Service"
echo 2. Uninstall: WindowsNetworkManager.exe -service uninstall
echo 3. Reinstall: WindowsNetworkManager.exe -service install
echo    (Make sure to run install as Administrator)
echo.
pause
