# Troubleshooting Guide

This guide helps you resolve common issues when using Windows Network Manager.

## Table of Contents

- [Firewall and Network Access Issues](#firewall-and-network-access-issues)
- [Application Won't Start](#application-wont-start)
- [WinDivert Errors](#windivert-errors)
- [Service Issues](#service-issues)
- [Packet Delay Not Working](#packet-delay-not-working)
- [Remote Upgrade Issues](#remote-upgrade-issues)
- [Port Conflicts](#port-conflicts)
- [General Tips](#general-tips)

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

## WinDivert Errors

### "Failed to open WinDivert handle"

**Causes and Solutions:**

1. **Not Running as Administrator:**
   - WinDivert requires Administrator privileges
   - Solution: Always run as Administrator

2. **Missing WinDivert.dll:**
   - DLL must be in the same directory as executable
   - Solution: Copy `WinDivert.dll` to the executable directory

3. **Wrong DLL Architecture:**
   - Must use 64-bit DLL for 64-bit Windows
   - Solution: Use DLL from `x64` folder of WinDivert package

4. **WinDivert Driver Not Installed:**
   - WinDivert may require driver installation on some systems
   - Solution: Check WinDivert documentation for driver installation

5. **Windows Version Incompatibility:**
   - Some Windows versions may not support WinDivert
   - Solution: Update Windows or check WinDivert compatibility

6. **Antivirus Blocking:**
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
   sc query WindowsNetworkManager
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
   sc query WindowsNetworkManager
   ```

2. **Check Event Viewer:**
   - Open Event Viewer
   - Check Windows Logs → Application and System
   - Look for errors related to WindowsNetworkManager

3. **Verify WinDivert.dll:**
   - Service runs from installation directory
   - Ensure DLL is in the same directory as the executable

4. **Manual Start:**
   ```cmd
   net start WindowsNetworkManager
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
   sc query WindowsNetworkManager
   ```
   Should show "RUNNING" state

4. **Restart Service:**
   ```cmd
   net stop WindowsNetworkManager
   net start WindowsNetworkManager
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
   - Stop service: `net stop WindowsNetworkManager`
   - Replace executable
   - Start service: `net start WindowsNetworkManager`

### Upgrade Installation Fails

**Solutions:**

1. **Stop Service First:**
   - Upgrade process should stop service automatically
   - If it fails, manually stop: `net stop WindowsNetworkManager`

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
sc query WindowsNetworkManager
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
   net stop WindowsNetworkManager
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
net start WindowsNetworkManager
net stop WindowsNetworkManager
sc query WindowsNetworkManager
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

*Last updated: Version 2.3.0*
