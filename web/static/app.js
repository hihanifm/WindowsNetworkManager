let statsInterval;

let upgradeStatusInterval;

// Load current configuration on page load
window.addEventListener('DOMContentLoaded', () => {
    loadConfig();
    startStatsUpdates();
    loadUpgradeInfo();
});

async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        const config = await response.json();
        document.getElementById('delay').value = config.delay_ms || 0;
        updateStatus(config.is_running);
        updateButtonStates(config.is_running);
    } catch (error) {
        showError('Failed to load configuration: ' + error.message);
    }
}

async function updateDelay() {
    const delayMs = parseInt(document.getElementById('delay').value);
    
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
            body: JSON.stringify({ delay_ms: delayMs })
        });

        const result = await response.json();
        
        if (result.error) {
            showError(result.error);
        } else {
            showError(''); // Clear error
            console.log('Delay updated to', delayMs, 'ms');
        }
    } catch (error) {
        showError('Failed to update delay: ' + error.message);
    }
}

async function startInterception() {
    try {
        const response = await fetch('/api/start', { method: 'POST' });
        const result = await response.json();
        
        if (result.error) {
            showError(result.error);
        } else {
            updateStatus(true);
            updateButtonStates(true);
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
            updateStatus(false);
            updateButtonStates(false);
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
    updateStats(); // Initial update
    statsInterval = setInterval(updateStats, 1000); // Update every second
}

async function updateStats() {
    try {
        const response = await fetch('/api/stats');
        const stats = await response.json();
        
        document.getElementById('totalPackets').textContent = formatNumber(stats.total_packets);
        document.getElementById('delayedPackets').textContent = formatNumber(stats.delayed_packets);
        document.getElementById('bytesProcessed').textContent = formatBytes(stats.bytes_processed);
        document.getElementById('uptime').textContent = formatUptime(stats.uptime_seconds);
    } catch (error) {
        console.error('Failed to update stats:', error);
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

async function loadUpgradeInfo() {
    try {
        const response = await fetch('/api/upgrade/status');
        const status = await response.json();
        
        if (status.current_version) {
            document.getElementById('currentVersion').textContent = status.current_version;
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
                loadUpgradeInfo(); // Refresh version info
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