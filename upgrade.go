package main

import (
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
	CurrentVersion = version.Version
	ServiceName    = version.ServiceName
	DefaultUpdateURL = "https://api.github.com/repos/hihanifm/WindowsNetworkManager/releases/latest"
)

var (
	upgradeMutex    sync.RWMutex
	upgradeStatus   *UpgradeStatus
	upgradeManager  *UpgradeManager
)

type UpgradeStatus struct {
	Status      string    `json:"status"` // "idle", "checking", "downloading", "installing", "completed", "error"
	Progress    string    `json:"progress"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	UpdateAvailable bool  `json:"update_available,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type UpgradeManager struct {
	exePath      string
	exeDir       string
	backupPath   string
	tempPath     string
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
		exePath:    exePath,
		exeDir:     exeDir,
		backupPath: filepath.Join(exeDir, exeName+".backup"),
		tempPath:   filepath.Join(exeDir, exeName+".new"),
	}

	upgradeStatus = &UpgradeStatus{
		Status:        "idle",
		CurrentVersion: CurrentVersion,
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
	
	// Find Windows executable asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".exe") && strings.Contains(asset.Name, "WindowsNetworkManager") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	updateAvailable := compareVersions(CurrentVersion, latestVersion) < 0

	upgradeMutex.Lock()
	upgradeStatus.Status = "idle"
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

// DownloadUpdate downloads the new executable
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

	// Create temp file
	outFile, err := os.Create(upgradeManager.tempPath)
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
				os.Remove(upgradeManager.tempPath)
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
			os.Remove(upgradeManager.tempPath)
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

	// Verify temp file exists
	if _, err := os.Stat(upgradeManager.tempPath); os.IsNotExist(err) {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = "Downloaded file not found"
		upgradeMutex.Unlock()
		return fmt.Errorf("downloaded file not found")
	}

	// Stop the service if running
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Stopping service..."
	upgradeMutex.Unlock()

	if err := stopService(); err != nil {
		log.Printf("Warning: Failed to stop service: %v", err)
		// Continue anyway - might not be running as service
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

	// Replace executable
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Installing new version..."
	upgradeMutex.Unlock()

	if err := copyFile(upgradeManager.tempPath, upgradeManager.exePath); err != nil {
		// Try to restore backup
		copyFile(upgradeManager.backupPath, upgradeManager.exePath)
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Installation failed: %v", err)
		upgradeMutex.Unlock()
		return err
	}

	// Cleanup temp file
	os.Remove(upgradeManager.tempPath)

	// Restart service
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Restarting service..."
	upgradeMutex.Unlock()

	if err := startService(); err != nil {
		log.Printf("Warning: Failed to start service: %v", err)
		// Continue - user can start manually
	}

	upgradeMutex.Lock()
	upgradeStatus.Status = "completed"
	upgradeStatus.Progress = "Upgrade completed successfully!"
	upgradeStatus.CompletedAt = time.Now()
	upgradeMutex.Unlock()

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
	cmd := exec.Command("net", "stop", ServiceName)
	return cmd.Run()
}

// startService starts the Windows service
func startService() error {
	cmd := exec.Command("net", "start", ServiceName)
	return cmd.Run()
}

// GetUpgradeStatus returns the current upgrade status
func GetUpgradeStatus() *UpgradeStatus {
	upgradeMutex.RLock()
	defer upgradeMutex.RUnlock()
	return upgradeStatus
}
