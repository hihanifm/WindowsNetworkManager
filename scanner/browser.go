package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openInBrowser opens the web interface in the default browser
func openInBrowser(ip string) {
	url := fmt.Sprintf("http://%s:%d", ip, DefaultPort)
	
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		fmt.Printf("Please open this URL in your browser: %s\n", url)
		return
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to open browser. Please open this URL manually: %s\n", url)
	} else {
		fmt.Printf("Opening %s in your browser...\n", url)
	}
}
