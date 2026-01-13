package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"WindowsNetworkManager/version"
)

const (
	DefaultPort     = 18080
	DefaultWebPort  = 18081
	DefaultTimeout  = 2 * time.Second
	DefaultWorkers  = 30
	// Use shared version from version package
	ServiceName     = version.ServiceName
	Version         = version.Version
)

func main() {
	// Web server mode flags
	webMode := flag.Bool("web", false, "Run as web server")
	webPort := flag.Int("port", DefaultWebPort, "Web server port")

	scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
	workers := scanCmd.Int("workers", DefaultWorkers, "Number of parallel workers")
	timeout := scanCmd.Duration("timeout", DefaultTimeout, "Timeout per IP check")
	jsonOutput := scanCmd.Bool("json", false, "Output in JSON format")
	useBroadcast := scanCmd.Bool("broadcast", true, "Use UDP broadcast/multicast discovery (faster, default: true)")

	openCmd := flag.NewFlagSet("open", flag.ExitOnError)

	// Check for web mode first
	flag.Parse()
	if *webMode {
		// Only allow web mode when run as a service (by launchd)
		if !isRunningAsService() {
			fmt.Fprintf(os.Stderr, "Error: Web server mode can only be run as a service.\n")
			fmt.Fprintf(os.Stderr, "Please use: ./start_service.sh\n")
			os.Exit(1)
		}
		startWebServer(*webPort)
		return
	}

	// If no flags and no args, don't default to web mode - show usage instead
	if len(os.Args) == 1 {
		printUsage()
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		scanCmd.Parse(os.Args[2:])
		instances, err := ScanNetworkWithBroadcast(*workers, *timeout, *useBroadcast)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning network: %v\n", err)
			os.Exit(1)
		}

		if *jsonOutput {
			outputJSON(instances)
		} else {
			outputTable(instances)
		}

	case "open":
		openCmd.Parse(os.Args[2:])
		if openCmd.NArg() == 0 {
			fmt.Fprintf(os.Stderr, "Error: IP address required\n")
			fmt.Fprintf(os.Stderr, "Usage: wnm-scanner open <ip-address>\n")
			os.Exit(1)
		}
		ip := openCmd.Arg(0)
		openInBrowser(ip)

	case "list":
		// For now, just run a scan. Could implement caching later
		instances, err := ScanNetworkWithBroadcast(DefaultWorkers, DefaultTimeout, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning network: %v\n", err)
			os.Exit(1)
		}
		outputTable(instances)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Windows Network Manager Scanner

Usage:
  wnm-scanner <command> [options]

Commands:
  scan [options]    Scan network for Windows Network Manager instances
  list              List discovered instances (alias for scan)
  open <ip>         Open web interface for instance at IP address

Scan Options:
  -workers int      Number of parallel workers (default: %d)
  -timeout duration Timeout per IP check (default: %v)
  -json             Output results in JSON format
  -broadcast        Use UDP broadcast/multicast discovery (faster, default: true)

Examples:
  wnm-scanner scan
  wnm-scanner scan -workers 50 -timeout 1s
  wnm-scanner scan -json
  wnm-scanner open 192.168.1.100

`, DefaultWorkers, DefaultTimeout)
}

func outputJSON(instances []InstanceInfo) {
	// Always output an array, even if empty
	if instances == nil {
		instances = []InstanceInfo{}
	}
	jsonData, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))
}

func outputTable(instances []InstanceInfo) {
	if len(instances) == 0 {
		fmt.Println("No Windows Network Manager instances found on the network.")
		return
	}

	fmt.Printf("\nFound %d instance(s):\n\n", len(instances))
	fmt.Printf("%-18s %-10s %-10s %s\n", "IP Address", "Status", "Delay (ms)", "URL")
	fmt.Println("─────────────────────────────────────────────────────────────")

	for _, inst := range instances {
		status := "Stopped"
		if inst.IsRunning {
			status = "Running"
		}
		fmt.Printf("%-18s %-10s %-10d http://%s:%d\n",
			inst.IP, status, inst.DelayMs, inst.IP, inst.Port)
	}
	fmt.Println()
}

// isRunningAsService checks if the process is being run by launchd (macOS service)
func isRunningAsService() bool {
	if runtime.GOOS != "darwin" {
		// On non-macOS, allow direct execution
		return true
	}
	
	// Check parent process name
	ppid := os.Getppid()
	if ppid == 1 {
		// Parent is PID 1 (launchd), we're running as a service
		return true
	}
	
	// Check if parent process is launchd
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", ppid), "-o", "comm=")
	output, err := cmd.Output()
	if err == nil {
		parentName := strings.TrimSpace(string(output))
		if strings.Contains(parentName, "launchd") {
			return true
		}
	}
	
	return false
}
