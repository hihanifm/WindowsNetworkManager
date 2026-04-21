let statsEventSource = null;
let upgradeStatusInterval;
let isServiceRunning = false;
let countdownInterval;
let pingEventSource = null;
let isPingRunning = false;
let pingStopRequested = false; // Flag to prevent multiple stop requests on unload

async function fetchJsonOrThrow(url, options) {
    const response = await fetch(url, options);
    const contentType = (response.headers.get('content-type') || '').toLowerCase();
    const bodyText = await response.text();

    const parseJsonSafely = () => {
        try {
            return JSON.parse(bodyText || 'null');
        } catch (e) {
            const snippet = (bodyText || '').slice(0, 200).replace(/\s+/g, ' ').trim();
            const hint = snippet.toLowerCase().startsWith('<!doctype') || snippet.toLowerCase().startsWith('<html')
                ? ' (received HTML, not JSON — is the backend running and serving /api/*?)'
                : '';
            throw new Error(`Response was not valid JSON${hint}. First bytes: "${snippet}"`);
        }
    };

    if (!response.ok) {
        if (contentType.includes('application/json')) {
            const errJson = parseJsonSafely();
            throw new Error(errJson?.error || `HTTP ${response.status} ${response.statusText}`);
        }
        const snippet = (bodyText || '').slice(0, 200).replace(/\s+/g, ' ').trim();
        throw new Error(`HTTP ${response.status} ${response.statusText}. First bytes: "${snippet}"`);
    }

    if (!contentType.includes('application/json')) {
        const snippet = (bodyText || '').slice(0, 200).replace(/\s+/g, ' ').trim();
        const hint = snippet.toLowerCase().startsWith('<!doctype') || snippet.toLowerCase().startsWith('<html')
            ? ' (received HTML, not JSON — is the backend running and serving /api/*?)'
            : '';
        throw new Error(`Expected JSON but got "${contentType || 'unknown'}"${hint}. First bytes: "${snippet}"`);
    }

    return parseJsonSafely();
}

// Load current configuration on page load
window.addEventListener('DOMContentLoaded', () => {
    loadMachineIdentity();
    loadConfig();
    loadUpgradeInfo();
    loadSchedule();
    loadLogs();
    loadDomainFilter();
    loadPingStatus();
    // Don't start stats updates yet - wait for config to load
    
    // Attach event listeners to buttons (more reliable than inline onclick)
    const refreshBtn = document.getElementById('refreshLogsBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', function(e) {
            e.preventDefault();
            console.log('Refresh button clicked via event listener');
            if (typeof refreshLogs === 'function') {
                refreshLogs();
            } else {
                console.error('refreshLogs function not found');
                alert('Error: refreshLogs function not loaded. Please refresh the page.');
            }
        });
    }
    
    const viewLocalLogsBtn = document.getElementById('viewLocalLogsBtn');
    if (viewLocalLogsBtn) {
        viewLocalLogsBtn.addEventListener('click', function(e) {
            e.preventDefault();
            console.log('View Local Logs button clicked');
            if (typeof loadLocalLogs === 'function') {
                loadLocalLogs();
            } else {
                console.error('loadLocalLogs function not found');
                alert('Error: loadLocalLogs function not loaded. Please refresh the page.');
            }
        });
    }
    
    // Attach event listeners to ping buttons
    const pingBtn = document.getElementById('pingBtn');
    if (pingBtn) {
        pingBtn.addEventListener('click', function(e) {
            e.preventDefault();
            if (typeof startPing === 'function') {
                startPing();
            } else {
                console.error('startPing function not found');
                alert('Error: startPing function not loaded. Please refresh the page.');
            }
        });
    }
    
    const pingStopBtn = document.getElementById('pingStopBtn');
    if (pingStopBtn) {
        pingStopBtn.addEventListener('click', function(e) {
            e.preventDefault();
            if (typeof stopPing === 'function') {
                stopPing();
            } else {
                console.error('stopPing function not found');
                alert('Error: stopPing function not loaded. Please refresh the page.');
            }
        });
    }
    
    // Stop ping when page is closed/unloaded
    function stopPingOnUnload() {
        if (isPingRunning && !pingStopRequested) {
            pingStopRequested = true;
            // Use fetch with keepalive for reliable delivery during page unload
            // keepalive ensures the request completes even if the page is closing
            fetch('/api/ping/stop', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({}),
                keepalive: true
            }).catch(err => {
                // Silently fail - page is unloading anyway
                console.log('Failed to stop ping on unload:', err);
            });
        }
    }
    
    // Use both beforeunload and pagehide for maximum reliability
    // pagehide is more reliable in modern browsers and works even when page is cached
    window.addEventListener('beforeunload', stopPingOnUnload);
    window.addEventListener('pagehide', stopPingOnUnload);
});

async function loadMachineIdentity() {
    const el = document.getElementById('pcIdentity');
    if (!el) return;
    try {
        const response = await fetch('/api/discover');
        const data = await response.json();
        const name = data.hostname ? String(data.hostname) : '';
        const ips = Array.isArray(data.local_ips) ? data.local_ips.filter(Boolean) : [];
        let text = '';
        if (name && ips.length) {
            text = 'This PC: ' + name + ' · LAN IP: ' + ips.join(', ');
        } else if (name) {
            text = 'This PC: ' + name;
        } else if (ips.length) {
            text = 'LAN IP: ' + ips.join(', ');
        }
        if (text) {
            el.textContent = text;
            el.hidden = false;
        }
    } catch (e) {
        // Non-fatal: UI works without identity line
    }
}

function refreshActiveDelayUI(isRunning, activeDelayMs) {
    const span = document.getElementById('activeDelayValue');
    if (!span) return;
    if (!isRunning) {
        span.textContent = '—';
        return;
    }
    const n = typeof activeDelayMs === 'number' ? activeDelayMs : 0;
    span.textContent = n + ' ms';
}

function refreshActiveLossUI(isRunning, activeLossPct) {
    const span = document.getElementById('activeLossValue');
    if (!span) return;
    if (!isRunning) {
        span.textContent = '—';
        return;
    }
    const n = typeof activeLossPct === 'number' ? activeLossPct : 0;
    span.textContent = n + '%';
}

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        const config = await response.json();
        document.getElementById('delay').value = config.delay_ms != null ? config.delay_ms : 300;
        // Default to enabled unless backend explicitly says false.
        document.getElementById('randomDelay').checked = config.random_delay !== false;
        const pl = document.getElementById('packetLoss');
        if (pl) pl.value = config.packet_loss_percent != null ? config.packet_loss_percent : 0;
        isServiceRunning = config.is_running || false;
        updateStatus(isServiceRunning);
        updateButtonStates(isServiceRunning);
        refreshActiveDelayUI(isServiceRunning, config.active_delay_ms);
        refreshActiveLossUI(isServiceRunning, config.active_packet_loss_percent);
        
        // Handle duration fields
        if (config.duration_minutes) {
            document.getElementById('duration').value = config.duration_minutes;
        }
        
        // Start or stop stats updates based on service status
        if (isServiceRunning) {
            startStatsUpdates();
            // Start countdown timer if duration is set
            if (config.duration_minutes && config.duration_minutes > 0) {
                startCountdown();
            } else {
                stopCountdown();
            }
        } else {
            stopStatsUpdates();
            stopCountdown();
        }
    } catch (error) {
        showError('Failed to load configuration: ' + error.message);
    }
}

async function updateDelay() {
    const delayMs = parseInt(document.getElementById('delay').value);
    const randomDelay = document.getElementById('randomDelay').checked;
    const packetLossPercent = parseInt(document.getElementById('packetLoss').value, 10);
    
    if (isNaN(delayMs) || delayMs < 0 || delayMs > 10000) {
        showError('Delay must be between 0 and 10000 milliseconds');
        return;
    }
    if (isNaN(packetLossPercent) || packetLossPercent < 0 || packetLossPercent > 100) {
        showError('Packet loss must be between 0 and 100 percent');
        return;
    }

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ delay_ms: delayMs, random_delay: randomDelay, packet_loss_percent: packetLossPercent })
        });

        const result = await response.json();
        
        if (result.error) {
            showError(result.error);
        } else {
            showError(''); // Clear error
            // Update status and button states based on response
            isServiceRunning = result.is_running || false;
            updateStatus(isServiceRunning);
            updateButtonStates(isServiceRunning);
            
            // Start/stop stats updates based on service status
            if (isServiceRunning) {
                startStatsUpdates();
            } else {
                stopStatsUpdates();
            }

            refreshActiveDelayUI(isServiceRunning, result.active_delay_ms);
            refreshActiveLossUI(isServiceRunning, result.active_packet_loss_percent);
            
            console.log('Profile updated: delay', delayMs, 'ms, loss', packetLossPercent, '%');
        }
    } catch (error) {
        showError('Failed to update delay: ' + error.message);
    }
}

async function startInterception() {
    try {
        // Always read latest values from screen (delay, random delay, duration)
        const delayMs = parseInt(document.getElementById('delay').value) || 0;
        const randomDelay = document.getElementById('randomDelay').checked;
        const packetLossPercent = parseInt(document.getElementById('packetLoss').value, 10);
        const durationMinutes = parseInt(document.getElementById('duration').value) || 0;
        
        const requestBody = {
            delay_ms: delayMs,
            random_delay: randomDelay,
            packet_loss_percent: isNaN(packetLossPercent) ? 0 : Math.min(100, Math.max(0, packetLossPercent))
        };
        if (durationMinutes > 0) {
            requestBody.duration_minutes = durationMinutes;
        }
        
        const response = await fetch('/api/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestBody)
        });
        const result = await response.json();
        
        if (result.error) {
            showError(result.error);
        } else {
            isServiceRunning = true;
            updateStatus(true);
            updateButtonStates(true);
            startStatsUpdates(); // Start polling stats when service starts
            // Start countdown timer if duration is set
            if (durationMinutes > 0) {
                startCountdown();
            } else {
                stopCountdown();
            }
            refreshActiveDelayUI(true, result.active_delay_ms);
            refreshActiveLossUI(true, result.active_packet_loss_percent);
            showError(''); // Clear error
        }
    } catch (error) {
        showError('Failed to start interception: ' + error.message);
    }
}

async function stopInterception() {
    try {
        const response = await fetch('/api/stop', { method: 'POST' });
        const result = await response.json();
        
        if (result.error) {
            showError(result.error);
        } else {
            isServiceRunning = false;
            updateStatus(false);
            updateButtonStates(false);
            stopStatsUpdates(); // Stop stats streaming when service stops
            stopCountdown(); // Stop countdown timer
            refreshActiveDelayUI(false, 0);
            refreshActiveLossUI(false, 0);
            showError(''); // Clear error
        }
    } catch (error) {
        showError('Failed to stop interception: ' + error.message);
    }
}

function updateStatus(isRunning) {
    const statusEl = document.getElementById('status');
    if (isRunning) {
        statusEl.textContent = 'Running';
        statusEl.className = 'status running';
    } else {
        statusEl.textContent = 'Stopped';
        statusEl.className = 'status stopped';
    }
}

function updateButtonStates(isRunning) {
    const startBtn = document.getElementById('startBtn');
    const stopBtn = document.getElementById('stopBtn');
    
    if (isRunning) {
        startBtn.disabled = true;
        stopBtn.disabled = false;
    } else {
        startBtn.disabled = false;
        stopBtn.disabled = true;
    }
}

function startStatsUpdates() {
    // Close any existing EventSource connection
    stopStatsUpdates();
    
    // Only start if service is running
    if (!isServiceRunning) {
        return;
    }
    
    // Create EventSource connection to SSE endpoint
    statsEventSource = new EventSource('/api/stats/stream');
    
    // Handle incoming stats updates
    statsEventSource.addEventListener('message', (event) => {
        try {
            const stats = JSON.parse(event.data);
            updateStatsDisplay(stats);
        } catch (error) {
            console.error('Failed to parse stats:', error);
        }
    });
    
    // Handle errors
    statsEventSource.addEventListener('error', (error) => {
        console.error('Stats stream error:', error);
        // EventSource will automatically try to reconnect
        // If service stops, stopStatsUpdates will close the connection
    });
}

function stopStatsUpdates() {
    if (statsEventSource) {
        statsEventSource.close();
        statsEventSource = null;
    }
}

function updateStatsDisplay(stats) {
    document.getElementById('totalPackets').textContent = formatNumber(stats.total_packets);
    document.getElementById('delayedPackets').textContent = formatNumber(stats.delayed_packets);
    const dropEl = document.getElementById('droppedPackets');
    if (dropEl) dropEl.textContent = formatNumber(stats.dropped_packets || 0);
    document.getElementById('bytesProcessed').textContent = formatBytes(stats.bytes_processed);
    document.getElementById('uptime').textContent = formatUptime(stats.uptime_seconds);
    if (stats.active_delay_ms !== undefined) {
        refreshActiveDelayUI(isServiceRunning, stats.active_delay_ms);
    }
}

function formatNumber(num) {
    return num.toLocaleString();
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function formatUptime(seconds) {
    if (seconds < 60) return Math.floor(seconds) + 's';
    if (seconds < 3600) {
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return mins + 'm ' + secs + 's';
    }
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return hours + 'h ' + mins + 'm';
}

function showError(message) {
    const errorEl = document.getElementById('error');
    if (message) {
        errorEl.textContent = message;
        errorEl.classList.add('show');
    } else {
        errorEl.classList.remove('show');
    }
}

function startCountdown() {
    stopCountdown(); // Clear any existing interval
    
    countdownInterval = setInterval(async () => {
        try {
            const response = await fetch('/api/config');
            const config = await response.json();
            
            const remainingEl = document.getElementById('remainingTime');
            if (config.is_running && config.remaining_minutes && config.remaining_minutes > 0) {
                const minutes = Math.floor(config.remaining_minutes);
                const seconds = Math.floor((config.remaining_minutes - minutes) * 60);
                remainingEl.textContent = `⏱️ ${minutes}m ${seconds}s remaining`;
                remainingEl.style.display = 'block';
            } else {
                remainingEl.style.display = 'none';
                stopCountdown();
                // Session ended, reload config to update UI
                if (!config.is_running) {
                    loadConfig();
                }
            }
        } catch (error) {
            console.error('Failed to update countdown:', error);
        }
    }, 30000); // Update every 30 seconds
}

function stopCountdown() {
    if (countdownInterval) {
        clearInterval(countdownInterval);
        countdownInterval = null;
    }
    const remainingEl = document.getElementById('remainingTime');
    if (remainingEl) {
        remainingEl.style.display = 'none';
    }
}

async function loadUpgradeInfo() {
    try {
        // Add cache busting to ensure we get fresh version info
        const response = await fetch('/api/upgrade/status?' + new Date().getTime());
        const status = await response.json();
        
        if (status.current_version) {
            document.getElementById('currentVersion').textContent = status.current_version;
        }
        
        // Log compiled version for debugging (check browser console)
        if (status.compiled_version) {
            console.log('Compiled version (baked into binary):', status.compiled_version);
            // If there's a mismatch, warn the user
            if (status.current_version !== status.compiled_version) {
                console.warn('Version mismatch! Current:', status.current_version, 'Compiled:', status.compiled_version);
            }
        }
        
        // If upgrade is in progress, start polling
        if (status.status && status.status !== 'idle' && status.status !== 'completed' && status.status !== 'error') {
            startUpgradeStatusPolling();
        }
    } catch (error) {
        console.error('Failed to load upgrade info:', error);
    }
}

async function checkForUpdates() {
    const button = document.getElementById('checkUpdateBtn');
    const updateInfo = document.getElementById('updateInfo');
    
    button.disabled = true;
    button.textContent = 'Checking...';
    updateInfo.style.display = 'none';
    
    try {
        const response = await fetch('/api/upgrade/check');
        const result = await response.json();
        
        button.disabled = false;
        button.textContent = 'Check for Updates';
        
        // Stale UpgradeStatus.error can persist from a prior failed check; trust a fresh success payload.
        const staleErrorIgnored = result.update_available && result.latest_version;
        if (result.error && !staleErrorIgnored) {
            updateInfo.style.display = 'block';
            updateInfo.className = 'error show';
            updateInfo.innerHTML = `<strong>Error:</strong> ${result.error}`;
            return;
        }
        
        updateInfo.style.display = 'block';
        
        if (result.update_available) {
            updateInfo.className = 'upgrade-available';
            updateInfo.innerHTML = `
                <strong>Update Available!</strong>
                <p>Current: ${result.current_version} → Latest: ${result.latest_version}</p>
                <button class="btn-success" onclick="startUpgrade()" style="margin-top: 10px;">Upgrade Now</button>
            `;
        } else {
            updateInfo.className = 'info-box';
            updateInfo.innerHTML = `
                <p><strong>✓ You're up to date!</strong></p>
                <p>Current version: ${result.current_version}</p>
            `;
        }
    } catch (error) {
        button.disabled = false;
        button.textContent = 'Check for Updates';
        updateInfo.style.display = 'block';
        updateInfo.className = 'error show';
        updateInfo.innerHTML = `<strong>Error:</strong> ${error.message}`;
    }
}

async function startUpgrade() {
    const upgradeProgress = document.getElementById('upgradeProgress');
    const upgradeStatusText = document.getElementById('upgradeStatusText');
    const progressFill = document.getElementById('progressFill');
    const updateInfo = document.getElementById('updateInfo');
    
    upgradeProgress.classList.add('show');
    upgradeStatusText.textContent = 'Starting upgrade...';
    progressFill.style.width = '10%';
    updateInfo.style.display = 'none';
    
    try {
        const response = await fetch('/api/upgrade', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        
        const result = await response.json();
        
        if (result.error) {
            upgradeStatusText.textContent = `Error: ${result.error}`;
            upgradeProgress.classList.remove('show');
            return;
        }
        
        // Start polling for upgrade status
        startUpgradeStatusPolling();
    } catch (error) {
        upgradeStatusText.textContent = `Error: ${error.message}`;
        upgradeProgress.classList.remove('show');
    }
}

function startUpgradeStatusPolling() {
    if (upgradeStatusInterval) {
        clearInterval(upgradeStatusInterval);
    }
    
    upgradeStatusInterval = setInterval(checkUpgradeStatus, 1000);
    checkUpgradeStatus(); // Immediate check
}

function stopUpgradeStatusPolling() {
    if (upgradeStatusInterval) {
        clearInterval(upgradeStatusInterval);
        upgradeStatusInterval = null;
    }
}

async function checkUpgradeStatus() {
    try {
        const response = await fetch('/api/upgrade/status');
        const status = await response.json();
        
        const upgradeProgress = document.getElementById('upgradeProgress');
        const upgradeStatusText = document.getElementById('upgradeStatusText');
        const progressFill = document.getElementById('progressFill');
        const updateInfo = document.getElementById('updateInfo');
        
        if (!status || status.status === 'idle') {
            stopUpgradeStatusPolling();
            upgradeProgress.classList.remove('show');
            return;
        }
        
        upgradeProgress.classList.add('show');
        upgradeStatusText.textContent = status.progress || status.status;
        
        // Update progress bar based on status
        if (status.status === 'checking') {
            progressFill.style.width = '20%';
        } else if (status.status === 'downloading') {
            // Try to parse progress from text
            const progressMatch = status.progress.match(/(\d+)%/);
            if (progressMatch) {
                progressFill.style.width = progressMatch[1] + '%';
            } else {
                progressFill.style.width = '50%';
            }
        } else if (status.status === 'installing') {
            progressFill.style.width = '80%';
        } else if (status.status === 'completed') {
            progressFill.style.width = '100%';
            upgradeStatusText.textContent = 'Upgrade completed! The service will restart shortly.';
            stopUpgradeStatusPolling();
            setTimeout(() => {
                upgradeProgress.classList.remove('show');
                updateInfo.style.display = 'block';
                updateInfo.className = 'info-box';
                updateInfo.innerHTML = '<p><strong>✓ Upgrade completed successfully!</strong></p>';
                // Force refresh version info (with cache busting via API headers)
                loadUpgradeInfo();
                // Reload page after service restart to ensure new version is displayed
                setTimeout(() => {
                    location.reload(); // Force full page reload to get new version and web files
                }, 2000);
            }, 3000);
        } else if (status.status === 'error') {
            progressFill.style.width = '0%';
            upgradeStatusText.textContent = `Error: ${status.error || 'Upgrade failed'}`;
            stopUpgradeStatusPolling();
            setTimeout(() => {
                upgradeProgress.classList.remove('show');
            }, 5000);
        }
    } catch (error) {
        console.error('Failed to check upgrade status:', error);
    }
}

// Ping functions
async function startPing() {
    const domain = document.getElementById('pingDomain').value.trim();
    
    if (!domain) {
        showError('Please enter a domain name');
        return;
    }
    
    try {
        const response = await fetch('/api/ping/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ domain: domain })
        });
        
        const result = await response.json();
        
        if (result.error) {
            // If error says ping is already running, sync UI from server
            if (result.error.includes('already running')) {
                await loadPingStatus();
            }
            showError(result.error);
        } else {
            isPingRunning = true;
            pingStopRequested = false; // Reset flag when ping starts
            updatePingButtonStates(true);
            clearPingResults();
            startPingStream();
            showError(''); // Clear error
        }
    } catch (error) {
        showError('Failed to start ping: ' + error.message);
    }
}

async function stopPing() {
    try {
        const response = await fetch('/api/ping/stop', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });
        
        const result = await response.json();
        
        if (result.error) {
            if (result.error === 'Ping is not running') {
                isPingRunning = false;
                updatePingButtonStates(false);
                stopPingStream();
                showError('');
            } else {
                showError(result.error);
            }
        } else {
            isPingRunning = false;
            updatePingButtonStates(false);
            stopPingStream();
            appendPingResult('Ping stopped', 'info');
            showError(''); // Clear error
        }
    } catch (error) {
        showError('Failed to stop ping: ' + error.message);
    }
}

function startPingStream() {
    // Close any existing EventSource connection
    stopPingStream();
    
    // Only start if ping is running
    if (!isPingRunning) {
        return;
    }
    
    // Create EventSource connection to SSE endpoint
    pingEventSource = new EventSource('/api/ping/stream');
    
    // Handle incoming ping updates
    pingEventSource.addEventListener('message', (event) => {
        try {
            const data = JSON.parse(event.data);
            appendPingResult(data.line, data.type, data.timestamp);
        } catch (error) {
            console.error('Failed to parse ping data:', error);
        }
    });
    
    // Handle errors
    pingEventSource.addEventListener('error', (error) => {
        console.error('Ping stream error:', error);
        // Check if ping is still running
        if (!isPingRunning) {
            // Ping was stopped, close connection
            stopPingStream();
        }
        // EventSource will automatically try to reconnect
    });
}

function stopPingStream() {
    if (pingEventSource) {
        pingEventSource.close();
        pingEventSource = null;
    }
}

function appendPingResult(line, type, timestamp) {
    const resultsDiv = document.getElementById('pingResults');
    if (!resultsDiv) return;
    
    const lineDiv = document.createElement('div');
    lineDiv.className = `ping-result-line ${type}`;
    
    const ts = timestamp || new Date().toLocaleTimeString();
    lineDiv.innerHTML = `<span class="ping-timestamp">[${ts}]</span>${escapeHtml(line)}`;
    
    resultsDiv.appendChild(lineDiv);
    
    // Auto-scroll to bottom
    resultsDiv.scrollTop = resultsDiv.scrollHeight;
}

function clearPingResults() {
    const resultsDiv = document.getElementById('pingResults');
    if (resultsDiv) {
        resultsDiv.innerHTML = '';
    }
}

function updatePingButtonStates(isRunning) {
    const pingBtn = document.getElementById('pingBtn');
    const pingStopBtn = document.getElementById('pingStopBtn');
    const pingDomainInput = document.getElementById('pingDomain');
    
    if (isRunning) {
        pingBtn.disabled = true;
        pingDomainInput.disabled = true;
    } else {
        pingBtn.disabled = false;
        pingDomainInput.disabled = false;
    }
    if (pingStopBtn) {
        pingStopBtn.disabled = false;
    }
}

async function loadPingStatus() {
    try {
        const response = await fetch('/api/ping/status');
        const status = await response.json();
        
        if (status.is_running) {
            isPingRunning = true;
            pingStopRequested = false; // Reset flag when ping is running
            updatePingButtonStates(true);
            // If ping is already running, start the stream to show results
            if (status.domain) {
                document.getElementById('pingDomain').value = status.domain;
            }
            startPingStream();
        } else {
            isPingRunning = false;
            updatePingButtonStates(false);
        }
    } catch (error) {
        console.error('Failed to load ping status:', error);
        // On error, assume ping is not running
        isPingRunning = false;
        updatePingButtonStates(false);
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Schedule functions
async function loadSchedule() {
    try {
        const response = await fetch('/api/schedule');
        const schedule = await response.json();
        
        document.getElementById('scheduleEnabled').checked = schedule.enabled || false;
        document.getElementById('scheduleStartTime').value = schedule.start_time || '09:00';
        document.getElementById('scheduleEndTime').value = schedule.end_time || '18:00';
        document.getElementById('scheduleMaxSessionsPerHour').value = schedule.max_sessions_per_hour || 6;

        const ov = schedule.impairment_override || {};
        document.getElementById('scheduleOverrideEnabled').checked = !!ov.enabled;
        document.getElementById('scheduleOverrideDelay').value = ov.delay_ms != null ? ov.delay_ms : 1000;
        document.getElementById('scheduleOverrideRandom').checked = ov.random_delay !== false;
        document.getElementById('scheduleOverrideLoss').value = ov.packet_loss_percent != null ? ov.packet_loss_percent : 0;
        
        // Set day checkboxes
        if (schedule.days && Array.isArray(schedule.days)) {
            for (let i = 0; i <= 6; i++) {
                const checkbox = document.getElementById('day' + i);
                if (checkbox) {
                    checkbox.checked = schedule.days.includes(i);
                }
            }
        } else {
            // Default: Monday-Friday (1-5)
            document.getElementById('day1').checked = true;
            document.getElementById('day2').checked = true;
            document.getElementById('day3').checked = true;
            document.getElementById('day4').checked = true;
            document.getElementById('day5').checked = true;
        }
        
        updateScheduleStatus(schedule);
    } catch (error) {
        console.error('Failed to load schedule:', error);
        showError('Failed to load schedule configuration: ' + error.message);
    }
}

async function saveSchedule() {
    try {
        const enabled = document.getElementById('scheduleEnabled').checked;
        const startTime = document.getElementById('scheduleStartTime').value;
        const endTime = document.getElementById('scheduleEndTime').value;
        const maxSessionsPerHour = parseInt(document.getElementById('scheduleMaxSessionsPerHour').value);
        const overrideEnabled = document.getElementById('scheduleOverrideEnabled').checked;
        const overrideDelay = parseInt(document.getElementById('scheduleOverrideDelay').value, 10);
        const overrideRandom = document.getElementById('scheduleOverrideRandom').checked;
        const overrideLoss = parseInt(document.getElementById('scheduleOverrideLoss').value, 10);
        
        // Get selected days
        const days = [];
        for (let i = 0; i <= 6; i++) {
            const checkbox = document.getElementById('day' + i);
            if (checkbox && checkbox.checked) {
                days.push(i);
            }
        }
        
        if (days.length === 0) {
            showError('Please select at least one day');
            return;
        }
        
        if (overrideEnabled) {
            if (isNaN(overrideDelay) || overrideDelay < 0 || overrideDelay > 10000) {
                showError('Schedule override delay must be between 0 and 10000 milliseconds');
                return;
            }
            if (isNaN(overrideLoss) || overrideLoss < 0 || overrideLoss > 100) {
                showError('Schedule override packet loss must be between 0 and 100');
                return;
            }
        }

        if (isNaN(maxSessionsPerHour) || maxSessionsPerHour < 2 || maxSessionsPerHour > 60) {
            showError('Max sessions per hour must be between 2 and 60');
            return;
        }
        
        const schedule = {
            enabled: enabled,
            days: days,
            start_time: startTime,
            end_time: endTime,
            max_sessions_per_hour: maxSessionsPerHour,
            impairment_override: {
                enabled: overrideEnabled,
                delay_ms: overrideEnabled ? overrideDelay : 0,
                random_delay: overrideRandom,
                packet_loss_percent: overrideEnabled ? overrideLoss : 0
            }
        };
        
        const response = await fetch('/api/schedule', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(schedule)
        });
        
        const result = await response.json();
        
        if (result.error) {
            showError('Failed to save schedule: ' + result.error);
        } else {
            showError(''); // Clear error
            updateScheduleStatus(result);
            console.log('Schedule saved successfully');
        }
    } catch (error) {
        showError('Failed to save schedule: ' + error.message);
    }
}

function updateScheduleStatus(schedule) {
    const statusEl = document.getElementById('scheduleStatus');
    const statusTextEl = document.getElementById('scheduleStatusText');
    const nextSessionTimeEl = document.getElementById('nextSessionTime');
    const sessionsCompletedEl = document.getElementById('sessionsCompleted');
    
    if (!schedule.enabled) {
        statusEl.style.display = 'none';
        return;
    }
    
    statusEl.style.display = 'block';
    
    // Update sessions completed count
    if (schedule.sessions_completed !== undefined) {
        sessionsCompletedEl.textContent = schedule.sessions_completed;
    } else {
        sessionsCompletedEl.textContent = '0';
    }
    
    // Update next session time
    if (schedule.next_session_time_local) {
        nextSessionTimeEl.textContent = schedule.next_session_time_local;
    } else if (schedule.next_session_time) {
        // Parse ISO format and show local time
        try {
            const nextSession = new Date(schedule.next_session_time);
            const hours = String(nextSession.getHours()).padStart(2, '0');
            const minutes = String(nextSession.getMinutes()).padStart(2, '0');
            const seconds = String(nextSession.getSeconds()).padStart(2, '0');
            nextSessionTimeEl.textContent = `${hours}:${minutes}:${seconds}`;
        } catch (e) {
            nextSessionTimeEl.textContent = schedule.next_session_time;
        }
    } else {
        nextSessionTimeEl.textContent = 'No upcoming session';
    }
    
    // Check if current time is within schedule (use server data if available, otherwise calculate client-side)
    let isActive = false;
    if (schedule.is_within_schedule !== undefined) {
        isActive = schedule.is_within_schedule;
    } else {
        // Fallback to client-side calculation
        const now = new Date();
        const currentDay = now.getDay(); // 0=Sunday, 1=Monday, ..., 6=Saturday
        const currentHour = now.getHours();
        const currentMinute = now.getMinutes();
        
        // Check if current day is in schedule
        const dayInSchedule = schedule.days && schedule.days.includes(currentDay);
        
        // Parse start and end times
        const [startHour, startMin] = (schedule.start_time || '09:00').split(':').map(Number);
        const [endHour, endMin] = (schedule.end_time || '18:00').split(':').map(Number);
        
        const currentTimeMinutes = currentHour * 60 + currentMinute;
        const startTimeMinutes = startHour * 60 + startMin;
        let endTimeMinutes = endHour * 60 + endMin;
        
        // Handle case where end time is next day
        if (endTimeMinutes <= startTimeMinutes) {
            endTimeMinutes += 24 * 60;
            if (currentTimeMinutes < startTimeMinutes) {
                // Check if we're in the previous day's end period
                const adjustedCurrentTime = currentTimeMinutes + 24 * 60;
                if (adjustedCurrentTime >= startTimeMinutes - 24 * 60 && adjustedCurrentTime <= endTimeMinutes) {
                    isActive = true;
                }
            }
        }
        
        const withinTimeRange = currentTimeMinutes >= startTimeMinutes && currentTimeMinutes <= endTimeMinutes;
        isActive = dayInSchedule && withinTimeRange;
    }
    
    if (isActive) {
        statusTextEl.textContent = 'Active (within schedule time range)';
        statusEl.style.background = '#d1fae5';
        statusTextEl.style.color = '#065f46';
    } else {
        let reason = '';
        if (!schedule.is_within_schedule) {
            // Use server-side info if available
            const now = new Date();
            const currentDay = now.getDay();
            const dayInSchedule = schedule.days && schedule.days.includes(currentDay);
            if (!dayInSchedule) {
                reason = 'Current day not in schedule';
            } else {
                reason = 'Outside schedule time range';
            }
        } else {
            // Fallback client-side calculation
            const now = new Date();
            const currentDay = now.getDay();
            const dayInSchedule = schedule.days && schedule.days.includes(currentDay);
            if (!dayInSchedule) {
                reason = 'Current day not in schedule';
            } else {
                reason = 'Outside schedule time range';
            }
        }
        statusTextEl.textContent = 'Inactive - ' + reason;
        statusEl.style.background = '#fee2e2';
        statusTextEl.style.color = '#991b1b';
    }
}

// Logs functions
async function loadLogs() {
    const logsContainer = document.getElementById('logsContainer');
    const logsError = document.getElementById('logsError');
    const refreshBtn = document.getElementById('refreshLogsBtn');
    
    if (!logsContainer) {
        console.error('Logs container not found');
        return;
    }
    
    // Show loading state
    if (refreshBtn) {
        refreshBtn.disabled = true;
        refreshBtn.textContent = 'Loading...';
    }
    
    // Show loading message in container
    logsContainer.innerHTML = '<div style="color: #64748b; font-style: italic;">Loading logs...</div>';
    logsError.style.display = 'none';
    
    try {
        const response = await fetch('/api/logs?count=50');
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        
        if (result.error) {
            logsError.textContent = result.error;
            logsError.style.display = 'block';
            logsContainer.innerHTML = '<div style="color: #64748b; font-style: italic;">Error loading logs. See error message below.</div>';
        } else {
            logsError.style.display = 'none';
            renderLogs(result.entries || []);
        }
    } catch (error) {
        logsError.textContent = 'Failed to load logs: ' + error.message;
        logsError.style.display = 'block';
        logsContainer.innerHTML = '<div style="color: #ef4444; font-style: italic;">Failed to load logs. Check console for details.</div>';
        console.error('Failed to load logs:', error);
    } finally {
        // Restore button state
        if (refreshBtn) {
            refreshBtn.disabled = false;
            refreshBtn.textContent = 'Refresh';
        }
    }
}

function refreshLogs() {
    console.log('Refresh logs clicked');
    try {
        loadLogs();
    } catch (error) {
        console.error('Error in refreshLogs:', error);
        const logsError = document.getElementById('logsError');
        if (logsError) {
            logsError.textContent = 'Error refreshing logs: ' + error.message;
            logsError.style.display = 'block';
        }
    }
}


function renderLogs(entries) {
    const logsContainer = document.getElementById('logsContainer');
    if (!logsContainer) return;
    
    if (entries.length === 0) {
        logsContainer.innerHTML = '<div style="color: #64748b; font-style: italic;">No log entries found.</div>';
        return;
    }
    
    // Reverse entries to show newest first (PowerShell returns oldest first)
    const reversedEntries = [...entries].reverse();
    
    logsContainer.innerHTML = reversedEntries.map(entry => {
        const levelClass = entry.level ? entry.level.toLowerCase() : 'info';
        const levelDisplay = entry.level || 'Info';
        const timestamp = entry.timestamp || 'N/A';
        const message = escapeHtml(entry.message || '');
        const eventID = entry.event_id ? ` [EventID: ${entry.event_id}]` : '';
        
        return `<div class="log-entry ${levelClass}">
            <span class="log-timestamp">[${timestamp}]</span>
            <span class="log-level">${levelDisplay}</span>
            <span class="log-message">${message}${eventID}</span>
        </div>`;
    }).join('');
    
    // Auto-scroll to bottom (newest entries)
    logsContainer.scrollTop = logsContainer.scrollHeight;
}

// Local logs functions
async function loadLocalLogs() {
    const localLogsSection = document.getElementById('localLogsSection');
    const localLogsFiles = document.getElementById('localLogsFiles');
    const localLogsError = document.getElementById('localLogsError');
    const viewLocalLogsBtn = document.getElementById('viewLocalLogsBtn');
    
    if (!localLogsSection || !localLogsFiles) {
        console.error('Local logs elements not found');
        return;
    }
    
    // Show loading state
    if (viewLocalLogsBtn) {
        viewLocalLogsBtn.disabled = true;
        viewLocalLogsBtn.textContent = 'Loading...';
    }
    
    localLogsSection.style.display = 'block';
    localLogsFiles.innerHTML = '<div style="color: #64748b; font-style: italic;">Loading log files...</div>';
    localLogsError.style.display = 'none';
    
    try {
        const response = await fetch('/api/logs/local');
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        
        if (result.error) {
            localLogsError.textContent = result.error;
            localLogsError.style.display = 'block';
            localLogsFiles.innerHTML = '<div style="color: #64748b; font-style: italic;">Error loading log files. See error message below.</div>';
        } else if (result.files && result.files.length > 0) {
            localLogsError.style.display = 'none';
            renderLocalLogFiles(result.files);
        } else {
            localLogsError.style.display = 'none';
            localLogsFiles.innerHTML = '<div style="color: #64748b; font-style: italic;">No local log files found.</div>';
        }
    } catch (error) {
        localLogsError.textContent = 'Failed to load local logs: ' + error.message;
        localLogsError.style.display = 'block';
        localLogsFiles.innerHTML = '<div style="color: #ef4444; font-style: italic;">Failed to load log files. Check console for details.</div>';
        console.error('Failed to load local logs:', error);
    } finally {
        if (viewLocalLogsBtn) {
            viewLocalLogsBtn.disabled = false;
            viewLocalLogsBtn.textContent = 'View Local Logs';
        }
    }
}

function renderLocalLogFiles(files) {
    const localLogsFiles = document.getElementById('localLogsFiles');
    if (!localLogsFiles) return;
    
    localLogsFiles.innerHTML = files.map(file => {
        const sizeKB = (file.size / 1024).toFixed(2);
        const sizeDisplay = file.size > 1024 * 1024 
            ? `${(file.size / (1024 * 1024)).toFixed(2)} MB`
            : `${sizeKB} KB`;
        
        return `
            <div class="domain-item" style="margin-bottom: 10px;">
                <div style="flex: 1;">
                    <div style="font-weight: 600; color: #333; margin-bottom: 4px;">${escapeHtml(file.name)}</div>
                    <div style="font-size: 12px; color: #666;">
                        Size: ${sizeDisplay} | Modified: ${file.modified}
                    </div>
                </div>
                <button class="btn-primary" onclick="openLocalLog('${escapeHtml(file.path)}')" style="margin-left: 10px; padding: 8px 16px; font-size: 14px;">View</button>
                <button class="btn-primary" onclick="downloadLocalLog('${escapeHtml(file.path)}', '${escapeHtml(file.name)}')" style="margin-left: 5px; padding: 8px 16px; font-size: 14px;">Download</button>
            </div>
        `;
    }).join('');
}

async function openLocalLog(filePath) {
    const localLogsContent = document.getElementById('localLogsContent');
    const localLogsError = document.getElementById('localLogsError');
    
    if (!localLogsContent) return;
    
    // Show loading state
    localLogsContent.style.display = 'block';
    localLogsContent.innerHTML = '<div style="color: #64748b; font-style: italic;">Loading log file...</div>';
    localLogsError.style.display = 'none';
    
    try {
        const response = await fetch(`/api/logs/local?file=${encodeURIComponent(filePath)}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        
        if (result.error) {
            localLogsError.textContent = result.error;
            localLogsError.style.display = 'block';
            localLogsContent.innerHTML = '<div style="color: #ef4444; font-style: italic;">Error loading log file.</div>';
        } else if (result.content) {
            localLogsError.style.display = 'none';
            // Display content with proper formatting
            localLogsContent.innerHTML = '<pre style="white-space: pre-wrap; word-wrap: break-word; margin: 0;">' + escapeHtml(result.content) + '</pre>';
            // Auto-scroll to bottom
            localLogsContent.scrollTop = localLogsContent.scrollHeight;
        } else {
            localLogsContent.innerHTML = '<div style="color: #64748b; font-style: italic;">No content available.</div>';
        }
    } catch (error) {
        localLogsError.textContent = 'Failed to load log file: ' + error.message;
        localLogsError.style.display = 'block';
        localLogsContent.innerHTML = '<div style="color: #ef4444; font-style: italic;">Failed to load log file. Check console for details.</div>';
        console.error('Failed to load log file:', error);
    }
}

async function downloadLocalLog(filePath, fileName) {
    try {
        const response = await fetch(`/api/logs/local?file=${encodeURIComponent(filePath)}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const result = await response.json();
        
        if (result.error) {
            alert('Error downloading log file: ' + result.error);
            return;
        }
        
        // Create a blob and download it
        const blob = new Blob([result.content], { type: 'text/plain' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = fileName || 'log.txt';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
    } catch (error) {
        alert('Failed to download log file: ' + error.message);
        console.error('Failed to download log file:', error);
    }
}

// Ensure all functions are globally accessible for inline onclick handlers
// This must be done after all functions are defined
(function() {
    'use strict';
    try {
        window.refreshLogs = refreshLogs;
        window.loadLogs = loadLogs;
        window.startPing = startPing;
        window.stopPing = stopPing;
        window.loadLocalLogs = loadLocalLogs;
        window.openLocalLog = openLocalLog;
        window.downloadLocalLog = downloadLocalLog;
        console.log('Functions registered globally:', {
            refreshLogs: typeof window.refreshLogs,
            loadLogs: typeof window.loadLogs,
            startPing: typeof window.startPing,
            stopPing: typeof window.stopPing,
            loadLocalLogs: typeof window.loadLocalLogs,
            openLocalLog: typeof window.openLocalLog,
            downloadLocalLog: typeof window.downloadLocalLog
        });
    } catch (error) {
        console.error('Error registering global functions:', error);
    }
})();

// Domain filter functions
let filteredDomainsList = [];
let domainFilterEnabled = false;

async function loadDomainFilter() {
    try {
        const data = await fetchJsonOrThrow('/api/domains');
        filteredDomainsList = data.filtered_domains || [];
        domainFilterEnabled = data.domain_filter_enabled || false;
        
        document.getElementById('domainFilterEnabled').checked = domainFilterEnabled;
        updateDomainList();
    } catch (error) {
        console.error('Failed to load domain filter:', error);
        showError('Failed to load domain filter: ' + error.message);
    }
}

function updateDomainList() {
    const domainList = document.getElementById('domainList');
    if (filteredDomainsList.length === 0) {
        domainList.innerHTML = '<p style="color: #666; font-style: italic;">No domains configured. When the filter is off, impairment applies to all outbound packets.</p>';
        return;
    }
    
    domainList.innerHTML = filteredDomainsList.map(domain => `
        <div class="domain-item">
            <span class="domain-name">${escapeHtml(domain)}</span>
            <button class="domain-remove" onclick="removeDomain('${escapeHtml(domain)}')">Remove</button>
        </div>
    `).join('');
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

async function addDomain() {
    const input = document.getElementById('newDomain');
    const domain = input.value.trim();
    
    if (!domain) {
        showError('Please enter a domain name');
        return;
    }
    
    // Basic validation
    if (domain.length > 255) {
        showError('Domain name too long (max 255 characters)');
        return;
    }
    
    // Check if already exists (case-insensitive)
    const domainLower = domain.toLowerCase();
    if (filteredDomainsList.some(d => d.toLowerCase() === domainLower)) {
        showError('Domain already in list');
        return;
    }
    
    filteredDomainsList.push(domain);
    input.value = '';
    updateDomainList();
}

async function removeDomain(domain) {
    filteredDomainsList = filteredDomainsList.filter(d => d !== domain);
    updateDomainList();
}

async function saveDomainFilter() {
    try {
        const enabled = document.getElementById('domainFilterEnabled').checked;
        
        const data = await fetchJsonOrThrow('/api/domains', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                filtered_domains: filteredDomainsList,
                domain_filter_enabled: enabled
            })
        });

        filteredDomainsList = data.filtered_domains || [];
        domainFilterEnabled = data.domain_filter_enabled || false;
        updateDomainList();
        
        showSuccess('Domain filter saved successfully');
    } catch (error) {
        console.error('Failed to save domain filter:', error);
        showError('Failed to save domain filter: ' + error.message);
    }
}

// Make functions available globally
window.addDomain = addDomain;
window.removeDomain = removeDomain;
window.saveDomainFilter = saveDomainFilter;