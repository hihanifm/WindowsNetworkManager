# How to Verify Packet Delay is Working

This guide helps you verify that packet delay is actually being applied to your network traffic.

## Method 1: Check Web Interface Status

1. **Open the web interface**: http://localhost:18080
2. **Check the Status indicator**:
   - Should show "Running" (green) if interception is active
   - If it shows "Stopped" (red), click the "Start" button first
3. **Check Statistics**:
   - Look at the "Statistics" section
   - **Total Packets** should be increasing (refreshes every 2 seconds)
   - **Delayed Packets** should match Total Packets (all packets are delayed)
   - **Bytes Processed** should be increasing
4. **Set a noticeable delay**:
   - Enter a delay value like **1000** (1 second) or **2000** (2 seconds)
   - Click "Set Delay"
   - Status should show "Running"

## Method 2: Test with Ping (Most Reliable)

This is the easiest way to verify delay is working:

1. **Open Command Prompt as Administrator**
2. **Set delay in web interface**: Set to **1000 ms** (1 second)
3. **Start interception** if not already running
4. **Ping a website**:
   ```cmd
   ping google.com
   ```
5. **Check the response time**:
   - Normal ping: ~10-50ms
   - **With 1000ms delay: Should be ~1000-1050ms** (normal ping + your delay)
   - If you see ~1000ms+ consistently, delay is working!

**Example output with delay working:**
```
Reply from 172.217.164.110: bytes=32 time=1042ms TTL=116
Reply from 172.217.164.110: bytes=32 time=1038ms TTL=116
```

**Example output without delay (not working):**
```
Reply from 172.217.164.110: bytes=32 time=45ms TTL=116
Reply from 172.217.164.110: bytes=32 time=42ms TTL=116
```

## Method 3: Test with YouTube/Browsing

YouTube may still work because:
- **Buffering**: YouTube buffers ahead, so small delays (100-500ms) may not be noticeable
- **Delay too small**: If delay is 0 or very small (< 100ms), you won't notice it

**To make it noticeable:**
1. Set delay to **2000-3000ms** (2-3 seconds)
2. Start a new YouTube video
3. You should notice:
   - Video takes longer to start loading
   - Buffering happens more frequently
   - Seeking/jumping takes longer

## Method 4: Check Console/Event Logs

### If running manually:
- Open the console window where you ran `WindowsNetworkManager.exe`
- Look for messages like:
  ```
  [INFO] Packet interception started successfully
  Starting packet interception engine...
  ```

### If running as service:
1. Run `view_logs.bat` (or open Event Viewer)
2. Look for "Windows Network Manager" entries
3. Check for any error messages

## Method 5: Network Statistics Verification

1. **Open web interface**: http://localhost:18080
2. **Check Statistics section**:
   - **Total Packets**: Should increase while browsing/downloading
   - **Delayed Packets**: Should equal Total Packets
   - **Bytes Processed**: Should increase
3. **Test while browsing**:
   - Open a new website
   - Watch the statistics update in real-time
   - If numbers are increasing, packets are being intercepted

## Troubleshooting: Delay Not Working

### Check 1: Is interception running?
- Web interface status should show "Running" (green)
- If "Stopped", click "Start" button

### Check 2: Is delay set correctly?
- Delay value should be > 0
- Try setting to 1000ms (1 second) for easy testing
- Click "Set Delay" after changing the value

### Check 3: Are you running as Administrator?
- The application MUST run with Administrator privileges
- Check with: `check_admin_privileges.bat`
- If not admin, restart as Administrator

### Check 4: Is WinDivert working?
- Check service status: `service_status.bat`
- Verify WinDivert driver: `install_windivert_driver.bat`
- Check Event Viewer for WinDivert errors

### Check 5: Is delay too small?
- Small delays (< 100ms) may not be noticeable
- Try 1000ms (1 second) or more for clear testing
- Use ping test (Method 2) for accurate measurement

## Quick Verification Test

**Fastest way to verify:**

1. Set delay to **1000ms** in web interface
2. Click "Start" if not running
3. Open Command Prompt
4. Run: `ping google.com -n 5`
5. Check response times:
   - **Working**: Times should be ~1000-1050ms
   - **Not working**: Times will be normal (~10-50ms)

## Expected Behavior

### When Delay is Working:
- ✅ Ping times increase by your delay amount
- ✅ Web pages load slower
- ✅ Downloads are slower
- ✅ Statistics show increasing packet counts
- ✅ Status shows "Running"

### When Delay is NOT Working:
- ❌ Ping times are normal
- ❌ No noticeable slowdown
- ❌ Statistics show 0 packets
- ❌ Status shows "Stopped"
- ❌ Error messages in console/logs

## Notes

- **YouTube buffering**: YouTube buffers ahead, so small delays may not affect playback once started
- **HTTPS/Encrypted traffic**: Delay still applies, but you won't see content, just timing
- **Local traffic**: Some local network traffic may not be delayed (depends on filter)
- **Delay applies to outbound packets**: The delay is added to packets leaving your computer
