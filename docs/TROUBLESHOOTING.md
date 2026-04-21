# Troubleshooting Guide

This guide helps you resolve common issues when using Windows Network Manager.

## Table of Contents

- [404 Errors (Page Not Found)](#404-errors-page-not-found)
- [Firewall and Network Access Issues](#firewall-and-network-access-issues)
- [Application Won't Start](#application-wont-start)
- [WinDivert Errors](#windivert-errors)
- [Service Issues](#service-issues)
- [Packet Delay Not Working](#packet-delay-not-working)
- [Remote Upgrade Issues](#remote-upgrade-issues)
- [Port Conflicts](#port-conflicts)
- [General Tips](#general-tips)

---

## 404 Errors (Page Not Found)

**Symptoms:**
- Getting "404 Not Found" or "404 page not found" errors
- Web interface doesn't load
- API endpoints return 404
- Browser shows "This site can't be reached" or "404" error

### Step 1: Verify Application is Running

**Check if the service is running:**
```cmd
sc query "Windows Push Notification Service"
```
Look for `STATE: 4 RUNNING`. If it shows `STOPPED`, start it:
```cmd
net start "Windows Push Notification Service"
```

**Check if running manually:**
```cmd
tasklist | findstr WindowsNetworkManager
```
If no process found, the application is not running.

**Check if port is listening:**
```cmd
netstat -ano | findstr :18080
```
Should show `LISTENING` status. If nothing appears, the server is not running.

### Step 2: Verify Correct URL

**Valid URLs:**
- Main page: `http://localhost:18080/` or `http://localhost:18080`
- API endpoints:
  - `http://localhost:18080/api/config`
  - `http://localhost:18080/api/stats`
  - `http://localhost:18080/api/network`
  - `http://localhost:18080/api/discover`
  - `http://localhost:18080/api/upgrade/check`

**Common mistakes:**
- ❌ `http://localhost:18080/index.html` (should be just `/`)
- ❌ `http://localhost:18080/web/index.html` (wrong path)
- ❌ `http://localhost:18080/api` (missing endpoint name)
- ❌ `https://localhost:18080` (should be `http://`, not `https://`)
- ❌ `http://localhost:8080` (wrong port, should be 18080)

### Step 3: Check Port Number

**Verify the port:**
```cmd
netstat -ano | findstr :18080
```
If nothing appears, check if a different port is being used:
```cmd
netstat -ano | findstr LISTENING
```
Look for the WindowsNetworkManager process.

**Check service configuration:**
- Service always uses port 18080 (hardcoded)
- Manual run can use different port: `WindowsNetworkManager.exe -port 8080`
- If you changed the port, use the correct port in the URL

### Step 4: Check Web Files Location

**If running as service:**
The service looks for `web/index.html` relative to the executable directory.

1. **Find executable location:**
   ```cmd
   sc qc "Windows Push Notification Service"
   ```
   Look for `BINARY_PATH_NAME` - this is where the EXE is located.

2. **Verify web files exist:**
   - Navigate to the executable directory
   - Check that `web/index.html` exists
   - Check that `web/static/app.js` exists

3. **If files are missing:**
   - **Note:** Older releases (before v2.4.0) may not include the `web/` folder in the ZIP
   - Download the latest release which includes `web/index.html` and `web/static/app.js`
   - Or manually add the `web/` folder from the source code repository
   - Extract all files to the same directory
   - Ensure `web/` folder is present with `index.html` and `static/app.js`

**If running manually:**
1. Run from the directory containing the `web/` folder:
   ```cmd
   cd C:\path\to\WindowsNetworkManager
   WindowsNetworkManager.exe
   ```

2. Or ensure the executable and `web/` folder are in the same directory.

### Step 5: Check Application Logs

**If running as service:**
```cmd
# Check Event Viewer
eventvwr.msc
```
Navigate to: Windows Logs → Application
Look for entries from "Windows Network Manager" or "WindowsNetworkManager"
Check for errors about file paths or web server startup.

**If running manually:**
- Check the console window for error messages
- Look for messages like:
  - "Failed to change directory"
  - "Failed to start web server"
  - "File not found"

### Step 6: Test Basic Connectivity

**Test if server is responding:**
```cmd
curl http://localhost:18080/api/network
```
Or use PowerShell:
```powershell
Invoke-WebRequest -Uri http://localhost:18080/api/network
```

**Expected response:**
```json
{"local_ips":["192.168.1.100"],"port":"18080"}
```

**If this works but the web page doesn't:**
- The server is running, but web files might be missing
- Check Step 4 above

**If this fails:**
- Server is not running or not accessible
- Check Steps 1-3 above

### Step 7: Restart the Application

**If running as service:**
```cmd
net stop "Windows Push Notification Service"
net start "Windows Push Notification Service"
```

**If running manually:**
1. Close the console window
2. Restart as Administrator:
   ```cmd
   WindowsNetworkManager.exe
   ```

### Step 8: Check for Path Issues

**Common path-related 404 causes:**

1. **Spaces in path:**
   - If installed in a path with spaces (e.g., `C:\Program Files\...`)
   - Service might have issues finding web files
   - Try installing to a path without spaces

2. **Working directory:**
   - Application changes to executable directory on startup
   - If this fails, web files won't be found
   - Check logs for "Failed to change directory" warnings

3. **File permissions:**
   - Ensure web files are readable
   - Check folder permissions on `web/` directory

### Step 9: Verify from Network IP

**If accessing from another device:**
1. **Get the correct IP:**
   ```cmd
   ipconfig
   ```
   Look for IPv4 Address (e.g., `192.168.1.100`)

2. **Use correct URL:**
   - `http://192.168.1.100:18080` (replace with your IP)
   - Not `http://localhost:18080` (localhost only works on the host PC)

3. **Check firewall:**
   - See [Firewall and Network Access Issues](#firewall-and-network-access-issues) section

### Step 10: Complete Reset

If nothing works, try a complete reset:

1. **Stop and uninstall service:**
   ```cmd
   net stop "Windows Push Notification Service"
   WindowsNetworkManager.exe -service uninstall
   ```

2. **Delete old installation:**
   - Remove the entire directory

3. **Fresh installation:**
   - Download latest release
   - Extract to a new directory (preferably without spaces in path)
   - Run `install_service.bat` as Administrator
   - Start service: `net start "Windows Push Notification Service"`

4. **Test:**
   - Open `http://localhost:18080`
   - Should load the web interface

### Quick Diagnostic Checklist

Run these commands in order:

```cmd
# 1. Check service status
sc query "Windows Push Notification Service"

# 2. Check if port is listening
netstat -ano | findstr :18080

# 3. Test API endpoint
curl http://localhost:18080/api/network

# 4. Check Event Viewer for errors
eventvwr.msc
```

**If all pass but still getting 404:**
- Check web files exist in executable directory
- Verify URL is correct (no typos, correct port)
- Try accessing from different browser
- Clear browser cache

---

## Firewall and Network Access Issues

### Cannot Access Web Interface from Other Devices on Network

**Symptoms:**
- Can access `http://localhost:18080` locally
- Cannot access from other devices using `http://<PC_IP>:18080`
- Connection timeout or refused errors

**Solution 1: Run the Firewall Configuration Script**

1. Right-click `configure_firewall.bat`
2. Select "Run as Administrator"
3. Follow the prompts

If the script doesn't work, proceed to Solution 2.

**Solution 2: Manual Windows Firewall Configuration**

1. **Open Windows Defender Firewall with Advanced Security:**
   - Press `Win + R`, type `wf.msc`, press Enter
   - Or search for "Windows Defender Firewall with Advanced Security" in Start menu

2. **Create Inbound Rule:**
   - Click "Inbound Rules" in the left panel
   - Click "New Rule..." in the right panel
   - Select "Port" → Next
   - Select "TCP" and "Specific local ports": enter `18080` → Next
   - Select "Allow the connection" → Next
   - Check all three profiles (Domain, Private, Public) → Next
   - Name it "Windows Network Manager" → Finish

3. **Verify the rule was created:**
   - Look for "Windows Network Manager" in the Inbound Rules list
   - Make sure it's Enabled (check the Status column)
   - Right-click the rule → Properties to verify settings

**Solution 3: Allow Application Through Firewall**

Sometimes Windows blocks the application executable itself, not just the port:

1. Open Windows Defender Firewall
2. Click "Allow an app or feature through Windows Firewall"
3. Click "Change settings" (requires admin)
4. Click "Allow another app..."
5. Browse to `WindowsNetworkManager.exe`
6. Check both "Private" and "Public" networks
7. Click "Add" and "OK"

**Solution 4: Check Third-Party Firewall/Antivirus**

If you have third-party firewall or antivirus software (Norton, McAfee, Kaspersky, etc.):

1. Check their firewall settings
2. Add `WindowsNetworkManager.exe` to allowed applications
3. Add port 18080 to allowed ports
4. Temporarily disable to test if it's the cause

**Solution 5: Verify Network Configuration**

1. Ensure devices are on the same network:
   ```cmd
   ipconfig
   ```
   Check the IPv4 Address on both devices - they should be on the same subnet (e.g., 192.168.1.x)

2. Test localhost first:
   - Try `http://localhost:18080` on the host PC first
   - If this works but network access doesn't, it's a firewall issue

3. Check if the server is binding to all interfaces:
   - Look for log messages like: "Web interface accessible from network at: http://192.168.x.x:18080"
   - If you only see localhost, there may be a binding issue

**Solution 6: Windows Firewall Blocking Even with Rule**

If you've created the rule but it still doesn't work:

1. Delete the existing rule and recreate it:
   ```cmd
   netsh advfirewall firewall delete rule name="Windows Network Manager"
   ```
   Then recreate using Solution 2 above.

2. Check if Windows Firewall is actually running:
   ```cmd
   netsh advfirewall show allprofiles state
   ```

3. Temporarily disable firewall to test (not recommended for production):
   - Only for testing to confirm firewall is the issue
   - Re-enable immediately after testing

---

## Application Won't Start

**Symptoms:**
- Application crashes on startup
- No web interface accessible
- Error messages in console

**Solutions:**

1. **Run as Administrator:**
   - Right-click `WindowsNetworkManager.exe`
   - Select "Run as Administrator"
   - Required for WinDivert to function

2. **Check Port Availability:**
   ```cmd
   netstat -ano | findstr :18080
   ```
   If another process is using port 18080, either:
   - Stop that process
   - Or use a different port: `WindowsNetworkManager.exe -port 8080`

3. **Verify WinDivert.dll:**
   - Ensure `WinDivert.dll` is in the same directory as the executable
   - Check file size (should be ~47KB for 64-bit)
   - Re-download if corrupted

4. **Check Windows Version:**
   - Requires Windows 10/11 (64-bit)
   - WinDivert requires Windows 7 or later

5. **Check Console Output:**
   - Look for specific error messages
   - Common errors are documented in other sections below

6. **Check Windows Event Viewer:**
   - Open Event Viewer (`eventvwr.msc`)
   - Check Windows Logs → Application
   - Look for errors related to WindowsNetworkManager

---

## WinDivert Driver Installation

**Symptoms:**
- "DLL is not loaded" error
- "The handle is invalid" error during Recv
- Handle creation succeeds but packet interception fails
- Driver-related errors in Event Viewer

### Quick Fix: Install Driver Script

1. **Run the installation script:**
   ```cmd
   REM Right-click and "Run as Administrator"
   install_windivert_driver.bat
   ```

2. **The script will:**
   - Check if `WinDivert64.sys` (or `WinDivert32.sys`) exists in the executable directory
   - Verify if the driver service is already installed
   - Optionally install the driver manually if needed

### Manual Driver Installation

**Step 1: Locate Driver File**

Ensure you have the WinDivert driver file:
- Download from: https://www.reqrypt.org/windivert.html
- Extract the archive
- Copy `WinDivert64.sys` (for 64-bit Windows) or `WinDivert32.sys` (for 32-bit) to the same directory as `WindowsNetworkManager.exe`

**Step 2: Install Driver (if auto-install fails)**

```cmd
REM Run Command Prompt as Administrator
cd "C:\path\to\WindowsNetworkManager"

REM Create driver service
sc create WinDivert type= kernel binPath= "C:\path\to\WindowsNetworkManager\WinDivert64.sys"

REM Start driver
sc start WinDivert
```

**Step 3: Verify Installation**

```cmd
REM Check driver service status
sc query WinDivert

REM Check if driver file exists
dir WinDivert*.sys
```

### Common Driver Issues

1. **Driver File Not Found:**
   - Ensure `WinDivert64.sys` is in the same directory as `WindowsNetworkManager.exe`
   - Check file name matches exactly (case-sensitive on some systems)

2. **Driver Blocked by Windows:**
   - Right-click `WinDivert64.sys` → Properties → Unblock (if available)
   - Windows may block unsigned drivers - you may need to disable driver signature enforcement temporarily

3. **Driver Auto-Install Fails:**
   - The driver should auto-install when `WinDivertOpen()` is called
   - If it doesn't, try manual installation using `sc create` command
   - Ensure you're running as Administrator

4. **"Invalid Handle" After Driver Install:**
   - Restart `WindowsNetworkManager.exe` after installing the driver
   - Check Event Viewer for driver errors
   - Verify the driver service is running: `sc query WinDivert`

5. **Driver Signature Issues:**
   - Windows may require signed drivers
   - For testing, you can disable driver signature enforcement (not recommended for production)
   - Or use a signed version of WinDivert if available

### Driver File Locations

The driver file should be in one of these locations:
1. **Same directory as executable** (recommended): `C:\path\to\WindowsNetworkManager\WinDivert64.sys`
2. **System32\drivers**: `C:\Windows\System32\drivers\WinDivert64.sys` (auto-installed location)

## WinDivert Errors

### "Failed to open WinDivert handle" or "DLL is not loaded"

**Causes and Solutions:**

1. **Not Running as Administrator:**
   - WinDivert requires Administrator privileges
   - Solution: Always run as Administrator

2. **Missing WinDivert.dll:**
   - DLL must be in the same directory as executable
   - **When running as service:** The DLL is loaded from the same directory where the executable is located (the service's binary path)
   - **When running manually:** The DLL is loaded from the same directory as the executable
   - Solution: Copy `WinDivert.dll` to the executable directory
   - **To find executable directory when running as service:**
     ```cmd
     sc qc "Windows Push Notification Service"
     ```
     Look for `BINARY_PATH_NAME` - the DLL must be in that same directory

3. **Wrong DLL Architecture:**
   - Must use the correct DLL for your Windows architecture
   - **x64 (AMD64) Windows:** Use DLL from `x64` folder of WinDivert package
   - **ARM64 Windows:** WinDivert may not have official ARM64 support
   - **Solution:** 
     - Check your Windows architecture: `systeminfo | findstr /C:"System Type"`
     - For x64: Use `WinDivert.dll` from the `x64` folder
     - For ARM64: You may need to check if WinDivert provides ARM64 builds, or use x64 emulation

4. **ARM Windows Emulator Issues:**
   - If running Windows on ARM (WoA) or in an ARM emulator:
     - **Native ARM64:** WinDivert may not support ARM64 natively - check WinDivert documentation
     - **x64 Emulation:** If emulating x64 Windows, use the x64 DLL (from `x64` folder)
     - **How to Determine Your Architecture:**
       
       **Method 1: Check System Information (Recommended)**
       ```cmd
       systeminfo | findstr /C:"System Type"
       ```
       - If it shows `x64-based PC` → You're running x64 (use x64 DLL)
       - If it shows `ARM64-based PC` → You're running native ARM64
       
       **Method 2: Check Processor Architecture**
       ```cmd
       echo %PROCESSOR_ARCHITECTURE%
       ```
       - `AMD64` → x64 architecture (use x64 DLL)
       - `ARM64` → Native ARM64 architecture
       
       **Method 3: Check via PowerShell**
       ```powershell
       [Environment]::Is64BitOperatingSystem
       Get-ComputerInfo | Select-Object CsProcessors
       ```
       
       **Method 4: Check Running Processes**
       ```cmd
       tasklist /FI "IMAGENAME eq WindowsNetworkManager.exe"
       ```
       Then check Task Manager → Details tab → Right-click column headers → Select "Platform"
       - If it shows "64-bit" and you're on ARM hardware → Likely x64 emulation
       - If it shows "ARM64" → Native ARM64
     
     - **Solution:** 
       - Determine if you're running native ARM64 or x64 emulation using the methods above
       - **For x64 emulation:** Use the x64 DLL (from `x64` folder of WinDivert package)
       - **For native ARM64:** Check if WinDivert provides ARM64 builds, or the DLL may not be compatible
       - Note: The application will attempt to load the DLL from the executable directory regardless of architecture

5. **WinDivert Driver Not Installed:**
   - WinDivert requires both `WinDivert.dll` (user-mode) and `WinDivert64.sys` or `WinDivert32.sys` (kernel driver)
   - The driver file must be in the same directory as `WindowsNetworkManager.exe`
   - **Solution:** Use `install_windivert_driver.bat` script (run as Administrator) to check and install the driver
   - **Manual Installation:**
     ```cmd
     REM Run as Administrator
     sc create WinDivert type= kernel binPath= "C:\path\to\WinDivert64.sys"
     sc start WinDivert
     ```
   - **Note:** The driver typically auto-installs when you run `WindowsNetworkManager.exe` as Administrator, but manual installation may be needed in some cases

6. **Windows Version Incompatibility:**
   - Some Windows versions may not support WinDivert
   - Solution: Update Windows or check WinDivert compatibility

7. **Antivirus Blocking:**
   - Some antivirus software blocks WinDivert
   - Solution: Add exception for `WindowsNetworkManager.exe` and `WinDivert.dll`

### "Access Denied" Errors

**Solutions:**

1. **Run as Administrator:**
   - The application MUST run with Administrator privileges
   - This is required by WinDivert, not optional

2. **Disable UAC Temporarily (for testing):**
   - Open User Account Control settings
   - Lower the slider (not recommended long-term)
   - Only for testing to confirm UAC is the issue

3. **Check File Permissions:**
   - Ensure executable directory is not read-only
   - Check folder permissions allow execution

---

## Service Issues

### Service Won't Install

**Solutions:**

1. **Run Installation as Administrator:**
   ```cmd
   WindowsNetworkManager.exe -service install
   ```
   Or right-click `install_service.bat` → Run as Administrator

2. **Check Existing Service:**
   ```cmd
   sc query "Windows Push Notification Service"
   ```
   If service exists but won't install, uninstall first:
   ```cmd
   WindowsNetworkManager.exe -service uninstall
   ```

3. **Check Service Logs:**
   - Open Event Viewer (`eventvwr.msc`)
   - Check Windows Logs → Application
   - Look for service-related errors

### Service Won't Start

**Solutions:**

1. **Check Service Status:**
   ```cmd
   sc query "Windows Push Notification Service"
   ```

2. **Check Event Viewer:**
   - Open Event Viewer
   - Check Windows Logs → Application and System
   - Look for errors related to WindowsNetworkManager

3. **Verify WinDivert.dll:**
   - Service runs from installation directory
   - **The DLL must be in the same directory as the executable**
   - When running as a service, Windows loads DLLs from the executable's directory (not the current working directory)
   - To find the executable directory:
     ```cmd
     sc qc "Windows Push Notification Service"
     ```
     Look for `BINARY_PATH_NAME` - ensure `WinDivert.dll` is in that same directory
   - **Important:** The service uses the same DLL location as regular execution - both look in the executable directory

4. **Manual Start:**
   ```cmd
   net start "Windows Push Notification Service"
   ```
   Check error message for specific issue

5. **Check Service Account:**
   - Service runs as Local System (required for WinDivert)
   - Should be automatic - no changes needed

### Service Starts But Web Interface Not Accessible

**Solutions:**

1. **Check Port 18080:**
   - Verify firewall allows port (see Firewall section above)
   - Check if port is in use: `netstat -ano | findstr :18080`

2. **Check Service Logs:**
   - Event Viewer → Windows Logs → Application
   - Look for HTTP server errors

3. **Verify Service is Running:**
   ```cmd
   sc query "Windows Push Notification Service"
   ```
   Should show "RUNNING" state

4. **Restart Service:**
   ```cmd
   net stop "Windows Push Notification Service"
   net start "Windows Push Notification Service"
   ```

### Service Stops Unexpectedly

**Solutions:**

1. **Check Event Viewer:**
   - Look for crash logs or errors
   - Check for out-of-memory errors

2. **Verify WinDivert Compatibility:**
   - Update Windows if possible
   - Check WinDivert documentation for known issues

3. **Check System Resources:**
   - Monitor memory and CPU usage
   - Service may crash if system is overloaded

4. **Check for Windows Updates:**
   - Some updates can affect WinDivert
   - May need to reinstall service after major updates

---

## Packet Delay Not Working

**Symptoms:**
- Status shows "Running" but no delay is noticeable
- Statistics show packets processed but latency unchanged

**Solutions:**

1. **Verify Delay Setting:**
   - Ensure delay is set to a value > 0
   - Try a large delay (1000ms) to make it obvious

2. **Ensure Started:**
   - Click "Start" button - status should show "Running"
   - Check statistics are updating

3. **Check Network Traffic:**
   - Application only delays outbound packets
   - Generate some network traffic (browse web, download file)
   - Check statistics to see if packets are being processed

4. **Verify WinDivert is Working:**
   - Check console/logs for WinDivert errors
   - If "Failed to open WinDivert handle" appears, see WinDivert Errors section

5. **Test with Simple Traffic:**
   ```cmd
   ping google.com
   ```
   With delay enabled, ping should take longer

6. **Check Firewall/Antivirus:**
   - Some security software can interfere with packet interception
   - Temporarily disable to test

---

## Remote Upgrade Issues

### Upgrade Check Fails

**Solutions:**

1. **Check Internet Connection:**
   - Upgrade system requires internet to check GitHub Releases
   - Verify connectivity: `ping github.com`

2. **Check GitHub API Access:**
   - Verify URL is correct: `https://api.github.com/repos/hihanifm/WindowsNetworkManager/releases/latest`
   - Check if GitHub is accessible from your network

3. **Check Firewall:**
   - Ensure outbound HTTPS (443) is allowed
   - Some corporate firewalls block GitHub

### Upgrade Download Fails

**Solutions:**

1. **Check Disk Space:**
   - Ensure enough space for download and backup
   - At least 50MB free recommended

2. **Check Permissions:**
   - Application directory must be writable
   - May need to run as Administrator

3. **Check Release Asset:**
   - Verify GitHub release has correct asset name
   - Should be `WindowsNetworkManager.exe` or similar

4. **Manual Upgrade:**
   - Download release ZIP manually
   - Stop service: `net stop "Windows Push Notification Service"`
   - Replace executable
   - Start service: `net start "Windows Push Notification Service"`

### Upgrade Installation Fails

**Solutions:**

1. **Stop Service First:**
   - Upgrade process should stop service automatically
   - If it fails, manually stop: `net stop "Windows Push Notification Service"`

2. **Check Backup:**
   - Old executable should be backed up as `.bak`
   - Can restore manually if needed

3. **Check File Locks:**
   - Ensure no other process is using the executable
   - Close all instances before upgrading

---

## Port Conflicts

### Port 18080 Already in Use

**Symptoms:**
- Error: "bind: address already in use"
- Application won't start

**Solutions:**

1. **Find Process Using Port:**
   ```cmd
   netstat -ano | findstr :18080
   ```
   Note the PID (last column)

2. **Identify Process:**
   ```cmd
   tasklist | findstr <PID>
   ```

3. **Stop Conflicting Process:**
   - If it's another instance of WindowsNetworkManager, stop it first
   - If it's another application, either stop it or use different port

4. **Use Different Port:**
   ```cmd
   WindowsNetworkManager.exe -port 8080
   ```
   Note: Update firewall rules if using different port

5. **Change Default Port (for service):**
   - Edit service configuration or use different port
   - Remember to update firewall rules

---

## General Tips

### Finding Your IP Address

**Method 1: Command Prompt**
```cmd
ipconfig
```
Look for "IPv4 Address" under your network adapter.

**Method 2: From Application**
Access: `http://localhost:18080/api/network`
Returns JSON with local IP addresses.

**Method 3: Windows Settings**
- Settings → Network & Internet → Wi-Fi (or Ethernet)
- Click on your connection
- Look for IPv4 address

### Checking Application Status

**If Running as Service:**
```cmd
sc query "Windows Push Notification Service"
```

**If Running Manually:**
- Check for console window
- Or check process: `tasklist | findstr WindowsNetworkManager`

### Viewing Logs

**Event Viewer (for service):**
1. Open Event Viewer (`eventvwr.msc`)
2. Windows Logs → Application
3. Filter for "Windows Network Manager" source

**Console Output (for manual run):**
- Check the console window where you started the application
- Errors and status messages appear there

### Testing Network Access

1. **Test Locally First:**
   ```cmd
   curl http://localhost:18080/api/network
   ```
   Or open browser: `http://localhost:18080`

2. **Test from Network:**
   - From another device: `http://<PC_IP>:18080`
   - Replace `<PC_IP>` with your PC's IP address

3. **Check Firewall:**
   - Verify rule exists and is enabled
   - Test with firewall temporarily disabled (for debugging only)

### Reset Configuration

If all else fails:

1. **Stop Service:**
   ```cmd
   net stop "Windows Push Notification Service"
   WindowsNetworkManager.exe -service uninstall
   ```

2. **Remove Firewall Rule:**
   ```cmd
   netsh advfirewall firewall delete rule name="Windows Network Manager"
   ```

3. **Fresh Installation:**
   - Download fresh release
   - Extract to new directory
   - Follow installation instructions from scratch

---

## Still Having Issues?

If none of the above solutions work:

1. **Check System Requirements:**
   - Windows 10/11 (64-bit)
   - Administrator privileges
   - WinDivert 2.2 or later

2. **Verify Installation:**
   - Both `WindowsNetworkManager.exe` and `WinDivert.dll` present
   - Both files are not corrupted
   - File sizes are correct (~10MB for EXE, ~47KB for DLL)

3. **Check for Conflicts:**
   - Other network monitoring software
   - VPN software
   - Corporate network policies

4. **Collect Information:**
   - Windows version: `winver`
   - Error messages from Event Viewer
   - Network configuration: `ipconfig /all`

5. **Open an Issue:**
   - GitHub: https://github.com/hihanifm/WindowsNetworkManager/issues
   - Include system information and error messages

---

## Quick Reference Commands

```cmd
# Service Management
net start "Windows Push Notification Service"
net stop "Windows Push Notification Service"
sc query "Windows Push Notification Service"
WindowsNetworkManager.exe -service install
WindowsNetworkManager.exe -service uninstall

# Firewall
netsh advfirewall firewall show rule name="Windows Network Manager"
netsh advfirewall firewall delete rule name="Windows Network Manager"
wf.msc  # Open Firewall GUI

# Network
ipconfig
netstat -ano | findstr :18080

# Logs
eventvwr.msc  # Event Viewer
```

---

## "Could not detect local IP addresses for network access" Error

**Symptoms:**
- Error message appears in Event Viewer logs
- Service starts successfully but IP detection fails
- Web interface works on localhost but IP not shown

**This is NOT a critical error:**
- The web interface still works at `http://localhost:18080`
- Network access still works - you just need to find your IP manually
- The service is running correctly

**Solutions:**

1. **Find your IP address manually:**
   ```cmd
   ipconfig
   ```
   Look for "IPv4 Address" under your active network adapter

2. **Use the web interface API:**
   - Open `http://localhost:18080/api/network`
   - This will return your IP addresses in JSON format
   - Even if detection fails at startup, the API may work

3. **Check network interfaces:**
   ```cmd
   ipconfig /all
   ```
   Verify you have active network adapters

4. **Restart network adapter:**
   - Open Network Connections
   - Disable and re-enable your network adapter
   - Restart the service

5. **Check if running as service:**
   - When running as a service, network detection may be delayed
   - Wait a few seconds after service start
   - Check Event Viewer again after network is fully initialized

**Why this happens:**
- Network interfaces may not be fully initialized when service starts
- Some network adapters may be filtered out (virtual adapters, etc.)
- Service context may have limited network access during startup

**Workaround:**
The web interface is always accessible at `http://localhost:18080` regardless of IP detection. To access from other devices, manually find your IP using `ipconfig` and use that address.

---

*Last updated: Version 2.12.0*
