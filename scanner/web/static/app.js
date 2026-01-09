let refreshInterval;

// Load instances on page load
window.addEventListener('DOMContentLoaded', () => {
    loadInstances();
    // Refresh every 2 seconds if scan is in progress
    refreshInterval = setInterval(checkScanStatus, 2000);
});

async function startScan() {
    const button = document.getElementById('scanButton');
    const status = document.getElementById('status');
    
    button.disabled = true;
    button.textContent = 'Scanning...';
    status.style.display = 'block';
    status.className = 'status scanning';
    status.innerHTML = '<strong>Scanning network...</strong><div class="progress">This may take 10-15 seconds</div>';

    try {
        const response = await fetch('/api/scan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        const result = await response.json();
        
        if (result.error) {
            status.className = 'status error';
            status.innerHTML = `<strong>Error:</strong> ${result.error}`;
            button.disabled = false;
            button.textContent = 'Scan Network';
        } else {
            // Start checking scan status
            checkScanStatus();
        }
    } catch (error) {
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
                status.style.display = 'block';
                status.className = 'status scanning';
                status.innerHTML = `<strong>Scanning...</strong><div class="progress">${data.scan.progress || 'In progress'}</div>`;
            } else if (data.scan.status === 'completed') {
                status.style.display = 'block';
                status.className = 'status';
                status.innerHTML = `<strong>Scan Complete!</strong> Found ${data.scan.instances.length} instance(s)`;
                button.disabled = false;
                button.textContent = 'Scan Network';
                displayInstances(data.instances || data.scan.instances);
            } else if (data.scan.status === 'error') {
                status.style.display = 'block';
                status.className = 'status error';
                status.innerHTML = `<strong>Error:</strong> ${data.scan.error}`;
                button.disabled = false;
                button.textContent = 'Scan Network';
            } else {
                // Idle
                status.style.display = 'none';
                button.disabled = false;
                button.textContent = 'Scan Network';
            }
        }
        
        // Always update instances list
        if (data.instances && data.instances.length > 0) {
            displayInstances(data.instances);
        }
    } catch (error) {
        console.error('Error checking scan status:', error);
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
