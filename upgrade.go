package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"WindowsNetworkManager/version"
)

const (
	// Use CurrentVersion alias for backward compatibility
	CurrentVersion   = version.Version
	ServiceName      = version.ServiceName
	DefaultUpdateURL = "https://api.github.com/repos/hihanifm/WindowsNetworkManager/releases/latest"
)

var (
	upgradeMutex   sync.RWMutex
	upgradeStatus  *UpgradeStatus
	upgradeManager *UpgradeManager
)

type UpgradeStatus struct {
	Status          string    `json:"status"` // "idle", "checking", "downloading", "installing", "completed", "error"
	Progress        string    `json:"progress"`
	CurrentVersion  string    `json:"current_version"`
	CompiledVersion string    `json:"compiled_version,omitempty"` // Version baked into the binary
	LatestVersion   string    `json:"latest_version,omitempty"`
	UpdateAvailable bool      `json:"update_available,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	Error           string    `json:"error,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
}

type UpgradeManager struct {
	exePath       string
	exeDir        string
	backupPath    string
	tempPath      string
	zipPath       string
	upgradeScript string // Path to upgrade helper script
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func initUpgradeManager() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	exeName := filepath.Base(exePath)

	upgradeManager = &UpgradeManager{
		exePath:       exePath,
		exeDir:        exeDir,
		backupPath:    filepath.Join(exeDir, exeName+".backup"),
		tempPath:      filepath.Join(exeDir, exeName+".new"),
		zipPath:       filepath.Join(exeDir, "upgrade.zip"),
		upgradeScript: filepath.Join(exeDir, "upgrade_helper.bat"),
	}

	// Note: version.Version is a compile-time constant. If the binary was compiled
	// with an old version, it will always show that version until rebuilt.
	currentVer := version.Version
	log.Printf("[UPGRADE] Initializing upgrade manager with version: %s", currentVer)

	upgradeStatus = &UpgradeStatus{
		Status:         "idle",
		CurrentVersion: currentVer, // Always use current version from package
	}

	return nil
}

// CheckForUpdates checks for available updates from GitHub releases
func CheckForUpdates(updateURL string) (*UpgradeStatus, error) {
	upgradeMutex.Lock()
	upgradeStatus.Status = "checking"
	upgradeStatus.Progress = "Checking for updates..."
	upgradeMutex.Unlock()

	if updateURL == "" {
		updateURL = DefaultUpdateURL
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(updateURL)
	if err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Failed to check for updates: %v", err)
		upgradeMutex.Unlock()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Update check failed: HTTP %d", resp.StatusCode)
		upgradeMutex.Unlock()
		return nil, fmt.Errorf("update check failed: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Failed to parse release info: %v", err)
		upgradeMutex.Unlock()
		return nil, err
	}

	// Extract version from tag (remove 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// Find ZIP file asset (preferred) or EXE file asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".zip") && strings.Contains(asset.Name, "WindowsNetworkManager") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	// Fallback to EXE if ZIP not found
	if downloadURL == "" {
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, ".exe") && strings.Contains(asset.Name, "WindowsNetworkManager") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	// Always use current version from version package
	currentVer := version.Version
	updateAvailable := compareVersions(currentVer, latestVersion) < 0

	upgradeMutex.Lock()
	upgradeStatus.Status = "idle"
	upgradeStatus.Progress = ""
	upgradeStatus.Error = ""
	upgradeStatus.CurrentVersion = currentVer
	upgradeStatus.LatestVersion = latestVersion
	upgradeStatus.UpdateAvailable = updateAvailable
	upgradeStatus.DownloadURL = downloadURL
	upgradeMutex.Unlock()

	return upgradeStatus, nil
}

// compareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// DownloadUpdate downloads the update (ZIP or EXE)
func DownloadUpdate(downloadURL string) error {
	if upgradeManager == nil {
		return fmt.Errorf("upgrade manager not initialized")
	}

	upgradeMutex.Lock()
	upgradeStatus.Status = "downloading"
	upgradeStatus.Progress = "Downloading update..."
	upgradeMutex.Unlock()

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Download failed: %v", err)
		upgradeMutex.Unlock()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Download failed: HTTP %d", resp.StatusCode)
		upgradeMutex.Unlock()
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Determine if it's a ZIP or EXE file
	isZIP := strings.HasSuffix(downloadURL, ".zip")
	var outPath string
	if isZIP {
		outPath = upgradeManager.zipPath
	} else {
		outPath = upgradeManager.tempPath
	}

	// Create temp file
	outFile, err := os.Create(outPath)
	if err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Failed to create temp file: %v", err)
		upgradeMutex.Unlock()
		return err
	}
	defer outFile.Close()

	// Download with progress tracking
	totalSize := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			written, writeErr := outFile.Write(buf[:n])
			if writeErr != nil {
				os.Remove(outPath)
				upgradeMutex.Lock()
				upgradeStatus.Status = "error"
				upgradeStatus.Error = fmt.Sprintf("Write failed: %v", writeErr)
				upgradeMutex.Unlock()
				return writeErr
			}
			downloaded += int64(written)

			// Update progress
			if totalSize > 0 {
				percent := (downloaded * 100) / totalSize
				upgradeMutex.Lock()
				upgradeStatus.Progress = fmt.Sprintf("Downloading: %d%% (%d/%d bytes)", percent, downloaded, totalSize)
				upgradeMutex.Unlock()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(outPath)
			upgradeMutex.Lock()
			upgradeStatus.Status = "error"
			upgradeStatus.Error = fmt.Sprintf("Download error: %v", err)
			upgradeMutex.Unlock()
			return err
		}
	}

	return nil
}

// InstallUpdate installs the downloaded update
func InstallUpdate() error {
	if upgradeManager == nil {
		return fmt.Errorf("upgrade manager not initialized")
	}

	upgradeMutex.Lock()
	upgradeStatus.Status = "installing"
	upgradeStatus.Progress = "Installing update..."
	upgradeMutex.Unlock()

	// Check if ZIP file exists (preferred) or EXE file
	var isZIP bool
	var sourcePath string
	if _, err := os.Stat(upgradeManager.zipPath); err == nil {
		isZIP = true
		sourcePath = upgradeManager.zipPath
	} else if _, err := os.Stat(upgradeManager.tempPath); err == nil {
		isZIP = false
		sourcePath = upgradeManager.tempPath
	} else {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = "Downloaded file not found"
		upgradeMutex.Unlock()
		return fmt.Errorf("downloaded file not found")
	}

	// Check if service is installed and stop it if running
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Checking service status..."
	upgradeMutex.Unlock()

	serviceInstalled := isServiceInstalled()
	if serviceInstalled {
		upgradeMutex.Lock()
		upgradeStatus.Progress = "Stopping service..."
		upgradeMutex.Unlock()

		if err := stopService(); err != nil {
			log.Printf("Warning: Failed to stop service: %v", err)
			// Continue anyway - might not be running as service
		}
	} else {
		log.Printf("[UPGRADE] Service not installed, skipping service stop")
	}

	// Backup current executable
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Backing up current version..."
	upgradeMutex.Unlock()

	if err := copyFile(upgradeManager.exePath, upgradeManager.backupPath); err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Backup failed: %v", err)
		upgradeMutex.Unlock()
		return err
	}

	// Extract and install files
	if isZIP {
		upgradeMutex.Lock()
		upgradeStatus.Progress = "Extracting update package..."
		upgradeMutex.Unlock()

		if err := extractAndInstallZIP(sourcePath); err != nil {
			// Try to restore backup
			copyFile(upgradeManager.backupPath, upgradeManager.exePath)
			upgradeMutex.Lock()
			upgradeStatus.Status = "error"
			upgradeStatus.Error = fmt.Sprintf("Installation failed: %v", err)
			upgradeMutex.Unlock()
			return err
		}
	} else {
		// Legacy: just replace executable
		upgradeMutex.Lock()
		upgradeStatus.Progress = "Installing new version..."
		upgradeMutex.Unlock()

		if err := copyFile(sourcePath, upgradeManager.exePath); err != nil {
			// Try to restore backup
			copyFile(upgradeManager.backupPath, upgradeManager.exePath)
			upgradeMutex.Lock()
			upgradeStatus.Status = "error"
			upgradeStatus.Error = fmt.Sprintf("Installation failed: %v", err)
			upgradeMutex.Unlock()
			return err
		}
	}

	// Cleanup temp files
	os.Remove(upgradeManager.tempPath)
	os.Remove(upgradeManager.zipPath)

	// Restart service if it was installed
	if serviceInstalled {
		upgradeMutex.Lock()
		upgradeStatus.Progress = "Restarting service..."
		upgradeMutex.Unlock()

		// Wait for Windows to release file handles and service to fully stop
		log.Printf("[UPGRADE] Waiting for service to fully stop before restart...")
		time.Sleep(3 * time.Second)

		if err := startService(); err != nil {
			errorMsg := fmt.Sprintf(`Failed to start service: %v. Please start manually using: net start "%s"`, err, ServiceName)
			log.Printf("[UPGRADE] ERROR: %s", errorMsg)
			upgradeMutex.Lock()
			upgradeStatus.Error = errorMsg
			upgradeMutex.Unlock()
			// Don't mark as error - upgrade succeeded, just service restart failed
			// User can start service manually
		} else {
			log.Printf("[UPGRADE] Service restarted successfully")
		}
	} else {
		log.Printf("[UPGRADE] Service not installed, skipping service restart")
	}

	upgradeMutex.Lock()
	upgradeStatus.Status = "completed"
	upgradeStatus.Progress = "Upgrade completed successfully!"
	upgradeStatus.CurrentVersion = version.Version // Update to new version after upgrade
	upgradeStatus.CompletedAt = time.Now()
	upgradeMutex.Unlock()

	return nil
}

// extractAndInstallZIP extracts the ZIP file and installs all necessary files
func extractAndInstallZIP(zipPath string) error {
	// Open ZIP file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %v", err)
	}
	defer r.Close()

	// Extract files
	for _, f := range r.File {
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Determine destination path
		var destPath string
		fileName := f.Name

		// Handle different file types
		if fileName == "WindowsNetworkManager.exe" {
			destPath = upgradeManager.exePath
		} else if fileName == "WinDivert.dll" {
			destPath = filepath.Join(upgradeManager.exeDir, "WinDivert.dll")
		} else if strings.HasPrefix(fileName, "web/") {
			// Extract web directory files
			relPath := strings.TrimPrefix(fileName, "web/")
			destPath = filepath.Join(upgradeManager.exeDir, "web", relPath)
			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				log.Printf("Warning: Failed to create directory for %s: %v", destPath, err)
				continue
			}
		} else {
			// Skip other files (batch files, etc.) - they're optional
			continue
		}

		// Open file from ZIP
		rc, err := f.Open()
		if err != nil {
			log.Printf("Warning: Failed to open %s from ZIP: %v", fileName, err)
			continue
		}

		// Create destination file
		destFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			log.Printf("Warning: Failed to create %s: %v", destPath, err)
			continue
		}

		// Copy file contents
		_, err = io.Copy(destFile, rc)
		rc.Close()
		destFile.Close()

		if err != nil {
			log.Printf("Warning: Failed to extract %s: %v", fileName, err)
			continue
		}

		// Update progress for important files
		if fileName == "WindowsNetworkManager.exe" {
			upgradeMutex.Lock()
			upgradeStatus.Progress = "Installing executable..."
			upgradeMutex.Unlock()
		} else if fileName == "WinDivert.dll" {
			upgradeMutex.Lock()
			upgradeStatus.Progress = "Installing WinDivert.dll..."
			upgradeMutex.Unlock()
		} else if strings.HasPrefix(fileName, "web/") {
			upgradeMutex.Lock()
			upgradeStatus.Progress = fmt.Sprintf("Installing web files... (%s)", fileName)
			upgradeMutex.Unlock()
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// stopService stops the Windows service
func stopService() error {
	// Use sc stop for better control
	cmd := exec.Command("sc", "stop", ServiceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[UPGRADE] Service stop output: %s", string(output))
		// Try net stop as fallback
		cmd = exec.Command("net", "stop", ServiceName)
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("failed to stop service: %v (net stop also failed: %v)", err, err2)
		}
	}

	// Wait for service to fully stop (Windows needs time to release the executable)
	log.Printf("[UPGRADE] Waiting for service to stop...")
	time.Sleep(3 * time.Second)

	return nil
}

// isServiceInstalled checks if the Windows service is installed
func isServiceInstalled() bool {
	cmd := exec.Command("sc", "query", ServiceName)
	err := cmd.Run()
	return err == nil
}

// startService starts the Windows service with retry logic
func startService() error {
	maxRetries := 5
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Use sc start for better control
		cmd := exec.Command("sc", "start", ServiceName)
		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		if err == nil {
			log.Printf("[UPGRADE] Service started successfully")
			// Wait a moment to ensure service is running
			time.Sleep(2 * time.Second)
			return nil
		}

		log.Printf("[UPGRADE] Service start attempt %d/%d failed: %v, output: %s", i+1, maxRetries, err, outputStr)

		// If service is already running, that's okay
		if strings.Contains(outputStr, "already been started") ||
			strings.Contains(outputStr, "is already running") ||
			strings.Contains(outputStr, "START_PENDING") {
			log.Printf("[UPGRADE] Service is already running or starting")
			time.Sleep(2 * time.Second)
			return nil
		}

		// Try net start as fallback
		if i == maxRetries-1 {
			log.Printf("[UPGRADE] Trying net start as fallback...")
			cmd = exec.Command("net", "start", ServiceName)
			output2, err2 := cmd.CombinedOutput()
			if err2 == nil {
				log.Printf("[UPGRADE] Service started successfully using net start")
				time.Sleep(2 * time.Second)
				return nil
			}
			log.Printf("[UPGRADE] net start also failed: %v, output: %s", err2, string(output2))
		}

		// Wait before retry
		if i < maxRetries-1 {
			log.Printf("[UPGRADE] Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed to start service after %d attempts", maxRetries)
}

// createUpgradeHelperScript creates a batch script that will handle the upgrade
// in a separate process after the current service stops
func createUpgradeHelperScript() error {
	scriptContent := fmt.Sprintf(`@echo off
setlocal enabledelayedexpansion

REM Upgrade Helper Script - Runs after service stops
REM This script handles the actual file replacement and service restart

echo ========================================
echo Windows Network Manager Upgrade Helper
echo ========================================
echo.

REM Wait for service to fully stop
echo Waiting for service to stop...
timeout /t 5 /nobreak >nul

REM Check if service is installed
sc query "%s" >nul 2>&1
set SERVICE_INSTALLED=%%errorLevel%%

if !SERVICE_INSTALLED! neq 0 (
    echo Service not installed, skipping service operations
    goto :install_files
)

REM Stop service if still running
echo Stopping service...
sc stop "%s"
timeout /t 5 /nobreak >nul

:install_files
echo.
echo Installing new version...

REM Extract and install files from ZIP
if exist "%s" (
    echo Extracting upgrade package...
    
    REM Create temp extraction directory
    set "TEMP_EXTRACT=%s\\upgrade_temp"
    if exist "!TEMP_EXTRACT!" rmdir /s /q "!TEMP_EXTRACT!"
    mkdir "!TEMP_EXTRACT!"
    
    REM Extract ZIP to temp directory using PowerShell
    powershell -NoProfile -Command "$ErrorActionPreference='Stop'; Expand-Archive -Path '%s' -DestinationPath '!TEMP_EXTRACT!' -Force"
    
    if %%errorLevel%% neq 0 (
        echo ERROR: Failed to extract ZIP file!
        pause
        exit /b 1
    )
    
    REM Backup current executable
    echo Backing up current version...
    if exist "%s" copy /Y "%s" "%s" >nul 2>&1
    
    REM Copy new files from extracted ZIP
    echo Copying new files...
    
    REM Copy executable
    if exist "!TEMP_EXTRACT!\\WindowsNetworkManager.exe" (
        copy /Y "!TEMP_EXTRACT!\\WindowsNetworkManager.exe" "%s" >nul
        if %%errorLevel%% equ 0 echo   - WindowsNetworkManager.exe
    )
    
    REM Copy DLL
    if exist "!TEMP_EXTRACT!\\WinDivert.dll" (
        copy /Y "!TEMP_EXTRACT!\\WinDivert.dll" "%s\\WinDivert.dll" >nul
        if %%errorLevel%% equ 0 echo   - WinDivert.dll
    )
    
    REM Copy web directory
    if exist "!TEMP_EXTRACT!\\web" (
        xcopy /E /I /Y "!TEMP_EXTRACT!\\web" "%s\\web\\" >nul
        if %%errorLevel%% equ 0 echo   - web directory
    )
    
    REM Clean up temp extraction directory
    rmdir /s /q "!TEMP_EXTRACT!" >nul 2>&1
    
    REM Clean up downloaded ZIP
    del "%s" >nul 2>&1
) else if exist "%s" (
    REM Legacy: just replace EXE
    echo Replacing executable...
    if exist "%s" copy /Y "%s" "%s" >nul 2>&1
    copy /Y "%s" "%s" >nul
    if %%errorLevel%% equ 0 (
        echo   - WindowsNetworkManager.exe
        del "%s" >nul 2>&1
    ) else (
        echo ERROR: Failed to replace executable!
        pause
        exit /b 1
    )
) else (
    echo ERROR: Upgrade files not found!
    echo Expected: %s or %s
    pause
    exit /b 1
)

echo.
echo Upgrade files installed successfully!

REM Restart service if it was installed
if !SERVICE_INSTALLED! equ 0 (
    echo.
    echo Restarting service...
    timeout /t 2 /nobreak >nul
    
    REM Try to start service with retries
    set retries=0
    :retry_start
    sc start "%s"
    if %%errorLevel%% equ 0 (
        echo Service restarted successfully!
        goto :done
    )
    set /a retries+=1
    if !retries! lss 5 (
        echo Retrying service start (attempt !retries!/5)...
        timeout /t 2 /nobreak >nul
        goto :retry_start
    )
    
    REM Fallback to net start
    echo Trying net start...
    net start "%s"
)

:done
echo.
echo ========================================
echo Upgrade completed!
echo ========================================
echo.
echo The service should now be running with the new version.
echo If the service did not start automatically, please run:
echo   net start "%s"
echo.
timeout /t 3 /nobreak >nul

REM Clean up this script
del "%%~f0"
`, ServiceName, ServiceName,
		upgradeManager.zipPath,
		upgradeManager.exeDir,
		upgradeManager.zipPath,
		upgradeManager.exePath, upgradeManager.exePath, upgradeManager.backupPath,
		upgradeManager.exePath,
		upgradeManager.exeDir,
		upgradeManager.exeDir,
		upgradeManager.zipPath,
		upgradeManager.tempPath,
		upgradeManager.exePath, upgradeManager.exePath, upgradeManager.backupPath,
		upgradeManager.tempPath, upgradeManager.exePath,
		upgradeManager.tempPath,
		upgradeManager.zipPath, upgradeManager.tempPath,
		ServiceName, ServiceName, ServiceName)

	return os.WriteFile(upgradeManager.upgradeScript, []byte(scriptContent), 0755)
}

// launchUpgradeHelper launches the upgrade helper script in a separate process
func launchUpgradeHelper() error {
	// Use cmd.exe with /B flag to run in background, detached from parent
	// This ensures it continues running even after the service stops
	// /MIN starts minimized, /B runs in background
	scriptPath := upgradeManager.upgradeScript

	// Use PowerShell to launch the script in a truly detached process
	// This ensures it survives the service stop
	psScript := fmt.Sprintf(`Start-Process -FilePath "%s" -WindowStyle Hidden -WorkingDirectory "%s"`,
		scriptPath, upgradeManager.exeDir)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", psScript)
	cmd.Dir = upgradeManager.exeDir

	// Don't wait for the command - let it run independently
	if err := cmd.Start(); err != nil {
		// Fallback to cmd.exe if PowerShell fails
		log.Printf("[UPGRADE] PowerShell launch failed, trying cmd.exe fallback: %v", err)
		cmd = exec.Command("cmd.exe", "/C", "start", "/MIN", "/B", scriptPath)
		cmd.Dir = upgradeManager.exeDir
		if err2 := cmd.Start(); err2 != nil {
			return fmt.Errorf("failed to start upgrade helper: %v (fallback also failed: %v)", err, err2)
		}
	}

	// Detach from parent process immediately
	cmd.Process.Release()

	log.Printf("[UPGRADE] Upgrade helper script launched in detached process: %s", scriptPath)
	return nil
}

// GetUpgradeStatus returns the current upgrade status
func GetUpgradeStatus() *UpgradeStatus {
	upgradeMutex.RLock()
	defer upgradeMutex.RUnlock()

	// Always return current version from version package (not cached)
	// IMPORTANT: version.Version is a compile-time constant. If the binary was
	// compiled with an old version (e.g., 2.6.0), it will always return that version
	// until the binary is rebuilt with the new version.
	status := *upgradeStatus
	currentVer := version.Version
	status.CurrentVersion = currentVer
	status.CompiledVersion = currentVer // This is the version baked into the binary at compile time

	// Log if there's a mismatch (for debugging)
	if upgradeStatus.CurrentVersion != currentVer && upgradeStatus.CurrentVersion != "" {
		log.Printf("[UPGRADE] Version mismatch detected: cached=%s, actual=%s", upgradeStatus.CurrentVersion, currentVer)
	}

	return &status
}
