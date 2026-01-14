let statsEventSource = null;
let upgradeStatusInterval;
let isServiceRunning = false;
let countdownInterval;
let pingEventSource = null;
let isPingRunning = false;

// Load current configuration on page load
window.addEventListener('DOMContentLoaded', () => {
    loadConfig();
    loadUpgradeInfo();
    loadSchedule();
    loadLogs();
    // Don't start stats updates yet - wait for config to load
});

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        const config = await response.json();
        document.getElementById('delay').value = config.delay_ms || 0;
        document.getElementById('randomDelay').checked = config.random_delay || false;
        isServiceRunning = config.is_running || false;
        updateStatus(isServiceRunning);
        updateButtonStates(isServiceRunning);
        
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
    
    if (isNaN(delayMs) || delayMs < 0 || delayMs > 10000) {
        showError('Delay must be between 0 and 10000 milliseconds');
        return;
    }

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ delay_ms: delayMs, random_delay: randomDelay })
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
            
            console.log('Delay updated to', delayMs, 'ms');
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
        const durationMinutes = parseInt(document.getElementById('duration').value) || 0;
        
        const requestBody = {
            delay_ms: delayMs,
            random_delay: randomDelay
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
    document.getElementById('bytesProcessed').textContent = formatBytes(stats.bytes_processed);
    document.getElementById('uptime').textContent = formatUptime(stats.uptime_seconds);
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
    }, 1000); // Update every second
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
        
        if (result.error) {
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
            showError(result.error);
        } else {
            isPingRunning = true;
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
            showError(result.error);
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
        pingStopBtn.disabled = false;
        pingDomainInput.disabled = true;
    } else {
        pingBtn.disabled = false;
        pingStopBtn.disabled = true;
        pingDomainInput.disabled = false;
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
        document.getElementById('scheduleMaxDelay').value = schedule.max_delay_ms || 1000;
        
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
        const maxDelayMs = parseInt(document.getElementById('scheduleMaxDelay').value);
        
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
        
        if (isNaN(maxDelayMs) || maxDelayMs < 1 || maxDelayMs > 10000) {
            showError('Max delay must be between 1 and 10000 milliseconds');
            return;
        }
        
        const schedule = {
            enabled: enabled,
            days: days,
            start_time: startTime,
            end_time: endTime,
            max_delay_ms: maxDelayMs
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
    
    if (!schedule.enabled) {
        statusEl.style.display = 'none';
        return;
    }
    
    statusEl.style.display = 'block';
    
    // Check if current time is within schedule
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
                statusTextEl.textContent = 'Active (within schedule time range)';
                statusEl.style.background = '#d1fae5';
                statusTextEl.style.color = '#065f46';
                return;
            }
        }
    }
    
    const withinTimeRange = currentTimeMinutes >= startTimeMinutes && currentTimeMinutes <= endTimeMinutes;
    const isActive = dayInSchedule && withinTimeRange;
    
    if (isActive) {
        statusTextEl.textContent = 'Active (within schedule time range)';
        statusEl.style.background = '#d1fae5';
        statusTextEl.style.color = '#065f46';
    } else {
        let reason = '';
        if (!dayInSchedule) {
            reason = 'Current day not in schedule';
        } else if (!withinTimeRange) {
            reason = 'Outside schedule time range';
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
    
    if (!logsContainer) return;
    
    try {
        const response = await fetch('/api/logs?count=50');
        const result = await response.json();
        
        if (result.error) {
            logsError.textContent = result.error;
            logsError.style.display = 'block';
            logsContainer.innerHTML = '';
        } else {
            logsError.style.display = 'none';
            renderLogs(result.entries || []);
        }
    } catch (error) {
        logsError.textContent = 'Failed to load logs: ' + error.message;
        logsError.style.display = 'block';
        logsContainer.innerHTML = '';
        console.error('Failed to load logs:', error);
    }
}

function refreshLogs() {
    loadLogs();
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