package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	DefaultPort     = 18080
	DefaultTimeout  = 2 * time.Second
	DefaultWorkers  = 30
	ServiceName     = "WindowsNetworkManager"
)

func main() {
	scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
	workers := scanCmd.Int("workers", DefaultWorkers, "Number of parallel workers")
	timeout := scanCmd.Duration("timeout", DefaultTimeout, "Timeout per IP check")
	jsonOutput := scanCmd.Bool("json", false, "Output in JSON format")

	openCmd := flag.NewFlagSet("open", flag.ExitOnError)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		scanCmd.Parse(os.Args[2:])
		instances, err := ScanNetwork(*workers, *timeout)
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
		instances, err := ScanNetwork(DefaultWorkers, DefaultTimeout)
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
