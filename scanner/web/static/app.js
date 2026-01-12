let refreshInterval;
let isScanning = false;

// Load instances on page load
window.addEventListener('DOMContentLoaded', () => {
    loadInstances();
    loadScannerVersion();
    // Don't start polling until a scan is started - polling will be initiated by startScan()
});

function startStatusPolling(interval) {
    if (refreshInterval) {
        clearInterval(refreshInterval);
    }
    refreshInterval = setInterval(checkScanStatus, interval);
}

function stopStatusPolling() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }
}

async function startScan() {
    const button = document.getElementById('scanButton');
    const status = document.getElementById('status');
    
    button.disabled = true;
    button.textContent = 'Scanning...';
    isScanning = true;
    startStatusPolling(300); // Fast polling during scan
    status.style.display = 'block';
    status.className = 'status scanning';
    status.innerHTML = '<strong>Scanning network...</strong><div class="progress">Initializing scan...</div>';

    try {
        const response = await fetch('/api/scan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        const result = await response.json();
        
        if (result.error) {
            isScanning = false;
            stopStatusPolling(); // Stop polling on error
            status.className = 'status error';
            status.innerHTML = `<strong>Error:</strong> ${result.error}`;
            button.disabled = false;
            button.textContent = 'Scan Network';
        } else {
            // Start polling for scan status (will be stopped when scan completes)
            isScanning = true;
            startStatusPolling(300);
            // Do initial check immediately
            checkScanStatus();
        }
    } catch (error) {
        isScanning = false;
        stopStatusPolling(); // Stop polling on error
        status.className = 'status error';
        status.innerHTML = `<strong>Error:</strong> ${error.message}`;
        button.disabled = false;
        button.textContent = 'Scan Network';
    }
}

async function checkScanStatus() {
    try {
        const response = await fetch('/api/instances');
        const data = await response.json();
        
        const status = document.getElementById('status');
        const button = document.getElementById('scanButton');
        
        if (data.scan) {
            if (data.scan.status === 'scanning') {
                // Continue polling during scanning (every 300ms for smooth updates)
                if (!isScanning) {
                    isScanning = true;
                    startStatusPolling(300);
                }
                
                status.style.display = 'block';
                status.className = 'status scanning';
                const progressText = data.scan.progress || 'In progress...';
                
                // Extract percentage and IP counts from progress text
                const progressMatch = progressText.match(/(\d+)\/(\d+).*?(\d+)%/);
                let progressBar = '';
                let percentage = 0;
                
                if (progressMatch) {
                    const scanned = parseInt(progressMatch[1]);
                    const total = parseInt(progressMatch[2]);
                    percentage = parseInt(progressMatch[3]);
                } else {
                    // Fallback: show animated progress bar if we can't parse
                    // This handles initial "Detecting network..." message
                    percentage = 5; // Small initial progress
                }
                
                progressBar = `
                    <div class="progress-bar-container">
                        <div class="progress-bar-fill" style="width: ${percentage}%"></div>
                        <div class="progress-bar-text">${percentage > 0 ? percentage + '%' : ''}</div>
                    </div>
                `;
                
                // Extract network info if available
                const networkInfo = data.scan.network_info ? 
                    `<div style="margin-top: 8px; font-size: 14px; color: #059669; font-weight: 600;">Network: ${data.scan.network_info}</div>` : '';
                
                status.innerHTML = `
                    <strong>Scanning...</strong>
                    <div class="progress">${progressText}</div>
                    ${networkInfo}
                    ${progressBar}
                `;
            } else if (data.scan.status === 'completed') {
                // Stop polling when scan is completed
                if (isScanning) {
                    isScanning = false;
                    stopStatusPolling();
                }
                
                status.style.display = 'block';
                status.className = 'status';
                
                // Get instances from scan result or instances list
                const instances = data.scan.instances || data.instances || [];
                const instanceCount = instances.length;
                
                status.innerHTML = `<strong>Scan Complete!</strong> Found ${instanceCount} instance(s)`;
                button.disabled = false;
                button.textContent = 'Scan Network';
                
                // Always display instances (will show "No instances" if empty)
                displayInstances(instances);
            } else if (data.scan.status === 'error') {
                // Stop polling on error
                if (isScanning) {
                    isScanning = false;
                    stopStatusPolling();
                }
                
                status.style.display = 'block';
                status.className = 'status error';
                status.innerHTML = `<strong>Error:</strong> ${data.scan.error}`;
                button.disabled = false;
                button.textContent = 'Scan Network';
            } else {
                // Idle - stop polling (no active scan)
                if (isScanning) {
                    isScanning = false;
                    stopStatusPolling();
                }
                
                status.style.display = 'none';
                button.disabled = false;
                button.textContent = 'Scan Network';
            }
        } else {
            // No scan status - stop polling if it was running
            if (isScanning) {
                isScanning = false;
                stopStatusPolling();
            }
        }
        
        // Update instances list only if we have instances
        if (data.instances && data.instances.length > 0) {
            displayInstances(data.instances);
        } else if (data.scan && data.scan.status === 'completed') {
            // Show "no instances" message when scan completes with no results
            displayNoInstances();
        }
    } catch (error) {
        console.error('Error checking scan status:', error);
        // Stop polling on error to prevent infinite retries
        stopStatusPolling();
        isScanning = false;
    }
}

async function loadInstances() {
    try {
        const response = await fetch('/api/instances');
        const data = await response.json();
        
        if (data.instances && data.instances.length > 0) {
            displayInstances(data.instances);
        } else {
            displayNoInstances();
        }
    } catch (error) {
        console.error('Error loading instances:', error);
    }
}

function displayInstances(instances) {
    const container = document.getElementById('instances');
    
    if (!instances || instances.length === 0) {
        displayNoInstances();
        return;
    }
    
    container.innerHTML = instances.map(inst => `
        <div class="instance-card">
            <div class="instance-header">
                <div class="instance-ip">${inst.ip}</div>
                <span class="instance-status ${inst.is_running ? 'status-running' : 'status-stopped'}">
                    ${inst.is_running ? 'Running' : 'Stopped'}
                </span>
            </div>
            <div class="instance-details">
                Delay: ${inst.delay_ms}ms | Port: ${inst.port}
            </div>
            <a href="http://${inst.ip}:${inst.port}" target="_blank" class="open-button">
                Open Web Interface
            </a>
        </div>
    `).join('');
}

function displayNoInstances() {
    const container = document.getElementById('instances');
    container.innerHTML = '<div class="no-instances">No instances discovered yet. Click "Scan Network" to search.</div>';
}

// Track retry attempts to prevent infinite loop
let versionLoadRetries = 0;
const MAX_VERSION_RETRIES = 3;
let versionLoadAttempted = false;

async function loadScannerVersion() {
    // Only attempt to load version once (on page load)
    if (versionLoadAttempted) {
        return;
    }
    versionLoadAttempted = true;

    try {
        const versionEl = document.getElementById('scannerVersion');
        if (!versionEl) {
            versionLoadRetries++;
            if (versionLoadRetries < MAX_VERSION_RETRIES) {
                // Retry after a short delay, max 3 times
                setTimeout(() => {
                    versionLoadAttempted = false;
                    loadScannerVersion();
                }, 500);
                return;
            } else {
                console.error('Scanner version element not found after', MAX_VERSION_RETRIES, 'retries');
                return;
            }
        }

        const response = await fetch('/api/status');
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const data = await response.json();
        
        if (data && data.version) {
            versionEl.textContent = data.version;
        } else {
            versionEl.textContent = 'Unknown';
        }
    } catch (error) {
        console.error('Error loading scanner version:', error);
        const versionEl = document.getElementById('scannerVersion');
        if (versionEl) {
            versionEl.textContent = 'Error';
        }
    }
}
