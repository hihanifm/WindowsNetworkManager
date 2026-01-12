@echo off
echo ========================================
echo Windows Architecture Checker
echo ========================================
echo.

echo [1] System Type:
systeminfo | findstr /C:"System Type"
echo.

echo [2] Processor Architecture:
for /f "tokens=2 delims==" %%a in ('wmic cpu get architecture /format:list ^| findstr "Architecture"') do (
    set ARCH=%%a
    if "%%a"=="9" (
        echo Architecture: 9 (x64/AMD64)
    ) else if "%%a"=="12" (
        echo Architecture: 12 (ARM64)
    ) else (
        echo Architecture: %%a
    )
)
echo.

echo [3] OS Architecture:
wmic os get osarchitecture /format:list | findstr "Architecture"
echo.

echo [4] Environment Check:
echo Is 64-bit OS: 
powershell -Command "[Environment]::Is64BitOperatingSystem"
echo Is 64-bit Process:
powershell -Command "[Environment]::Is64BitProcess"
echo.

echo [5] Processor Name:
wmic cpu get name /format:list | findstr "Name"
echo.

echo [6] Program Files Check:
if exist "C:\Program Files (x86)" (
    echo Program Files (x86) exists - Running x64 architecture
) else (
    echo Program Files (x86) does NOT exist - May be ARM64
)
echo.

echo [7] Quick Architecture Summary:
for /f "tokens=2 delims=:" %%a in ('systeminfo ^| findstr /C:"System Type"') do (
    set SYSTYPE=%%a
    echo System Type: %%a
)
echo.

echo ========================================
echo Interpretation:
echo ========================================
echo.
echo For WinDivert DLL:
echo - If System Type shows "x64-based PC" = Use x64 DLL (from x64 folder)
echo - If System Type shows "ARM64-based PC" = Check WinDivert ARM64 support
echo - Architecture 9 = x64 (AMD64) - Use x64 DLL
echo - Architecture 12 = ARM64 - May need x64 emulation or ARM64 DLL
echo.
echo x64 and AMD64 are the SAME - use x64 DLL
echo.
pause
