let statsInterval;

// Load current configuration on page load
window.addEventListener('DOMContentLoaded', () => {
    loadConfig();
    startStatsUpdates();
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

