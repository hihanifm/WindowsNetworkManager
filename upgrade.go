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
	LatestVersion   string    `json:"latest_version,omitempty"`
	UpdateAvailable bool      `json:"update_available,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	Error           string    `json:"error,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
}

type UpgradeManager struct {
	exePath    string
	exeDir     string
	backupPath string
	tempPath   string
	zipPath    string
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
		zipPath:    filepath.Join(exeDir, "upgrade.zip"),
	}

	upgradeStatus = &UpgradeStatus{
		Status:         "idle",
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
