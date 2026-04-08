# How to Check Windows Machine Architecture

This guide helps you determine if your Windows machine is x64 (AMD64) or ARM64, and whether it's running natively or emulated.

## Quick Methods

### Method 1: System Information (GUI)
1. Press `Win + R`
2. Type `msinfo32` and press Enter
3. Look for **"System Type"** - it will show:
   - `x64-based PC` = Native x64 (AMD64)
   - `ARM64-based PC` = Native ARM64
   - `x86-based PC` = 32-bit (rare on modern systems)

### Method 2: Settings App
1. Press `Win + I` to open Settings
2. Go to **System** → **About**
3. Look for **"System type"** - shows:
   - `64-bit operating system, x64-based processor` = Native x64
   - `64-bit operating system, ARM-based processor` = Native ARM64

### Method 3: Command Prompt
```cmd
systeminfo | findstr /C:"System Type"
```
Output examples:
- `System Type: x64-based PC` = Native x64
- `System Type: ARM64-based PC` = Native ARM64

### Method 4: PowerShell
```powershell
Get-ComputerInfo | Select-Object CsProcessors, OsArchitecture, CsSystemType
```
Or simpler:
```powershell
[Environment]::Is64BitOperatingSystem
[Environment]::Is64BitProcess
```

### Method 5: Check Processor Architecture
```cmd
wmic cpu get architecture
```
Output values:
- `9` = x64 (AMD64)
- `12` = ARM64
- `5` = ARM (32-bit)
- `0` = x86 (32-bit)

### Method 6: Check Running Process Architecture
```cmd
tasklist /FI "IMAGENAME eq explorer.exe" /FO LIST | findstr "Architecture"
```

## Check if Running in Emulation

### Method 1: Check for Emulation Layer
```cmd
wmic os get osarchitecture
```
If you see `ARM64` but your processor shows `x64`, you might be in emulation.

### Method 2: Check Process Architecture
```powershell
Get-Process | Select-Object ProcessName, @{Name="Architecture";Expression={if ($_.Path -match "\\SysWOW64\\") {"x86"} elseif ($_.Path -match "\\Program Files (x86)\\" -or $_.Path -match "\\Windows\\SysWOW64\\") {"x86"} else {"x64"}}}
```

### Method 3: Check Program Files Folders
```cmd
dir "C:\Program Files" /B
dir "C:\Program Files (x86)" /B
```
- If both exist: You're running x64 (x86 folder is for 32-bit apps)
- If only `Program Files` exists: Could be ARM64 or pure x64

### Method 4: Check Windows Version
```cmd
winver
```
Then check:
- Windows 10/11 on ARM = ARM64
- Windows 10/11 standard = Usually x64

## Batch Script to Check Everything

Save this as `check_architecture.bat`:

```batch
@echo off
echo ========================================
echo Windows Architecture Checker
echo ========================================
echo.

echo [1] System Type:
systeminfo | findstr /C:"System Type"
echo.

echo [2] Processor Architecture:
wmic cpu get architecture /format:list | findstr "Architecture"
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

echo ========================================
echo Interpretation:
echo ========================================
echo.
echo For WinDivert DLL:
echo - If System Type shows "x64-based PC" = Use x64 DLL
echo - If System Type shows "ARM64-based PC" = Check WinDivert ARM64 support
echo - If Architecture = 9 = x64 (AMD64)
echo - If Architecture = 12 = ARM64
echo.
pause
```

## For WinDivert Specifically

### Check Current DLL Architecture
If you have WinDivert.dll installed, check its architecture:
```powershell
[System.Reflection.Assembly]::LoadFile("C:\path\to\WinDivert.dll").GetName().ProcessorArchitecture
```

### Check Executable Architecture
```powershell
Get-Item "C:\path\to\WindowsNetworkManager.exe" | Select-Object VersionInfo
```

## Common Scenarios

### Scenario 1: Native x64 Windows
- System Type: `x64-based PC`
- Architecture: `9`
- Program Files (x86) exists
- **Use**: x64 WinDivert.dll (from `x64` folder)

### Scenario 2: Native ARM64 Windows (Surface Pro X, etc.)
- System Type: `ARM64-based PC`
- Architecture: `12`
- Program Files (x86) may not exist
- **Use**: Check if WinDivert supports ARM64 (may need x64 emulation)

### Scenario 3: Windows on ARM with x64 Emulation
- System Type: `ARM64-based PC`
- Architecture: `12`
- But can run x64 apps
- **Use**: x64 WinDivert.dll (runs in emulation)

## Quick Test

Run this single command to get all info:
```cmd
echo System Type: && systeminfo | findstr /C:"System Type" && echo. && echo Processor: && wmic cpu get architecture,name /format:list | findstr "Architecture\|Name"
```

## Notes

- **x64 = AMD64** - They are the same thing
- **ARM64** - Different architecture, may need special DLL
- **Emulation** - ARM64 Windows can run x64 apps via emulation
- **WinDivert** - Currently only supports x64 natively, ARM64 support may be limited
