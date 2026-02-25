// Package main provides a CLI tool for Android SDK setup.
// This tool automates Android SDK installation and configuration
// for developers building the Cheapskate Android app.
//
// Usage:
//
//	android-setup setup              Full setup (SDK + packages + Gradle wrapper)
//	android-setup setup --skip-emulator  Skip emulator installation
//	android-setup check              Check prerequisites only
//	android-setup help               Show help
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	command := "setup"
	if len(os.Args) >= 2 {
		command = os.Args[1]
	}

	switch command {
	case "setup":
		opts := parseSetupFlags(os.Args[2:])
		if err := RunSetup(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "check":
		info, issues := CheckPrerequisites()
		printCheckResults(info, issues)
		if len(issues) > 0 {
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func parseSetupFlags(args []string) SetupOptions {
	opts := SetupOptions{}
	for _, arg := range args {
		switch {
		case arg == "--skip-emulator":
			opts.SkipEmulator = true
		case arg == "--verbose" || arg == "-v":
			opts.Verbose = true
		case strings.HasPrefix(arg, "--sdk-path="):
			opts.SDKPath = strings.TrimPrefix(arg, "--sdk-path=")
		}
	}
	return opts
}

func printCheckResults(info *SystemInfo, issues []string) {
	fmt.Println("Android SDK Setup — Prerequisite Check")
	fmt.Println("=======================================")
	fmt.Println()

	printField := func(label, value string, ok bool) {
		status := "✓"
		if !ok {
			status = "✗"
		}
		fmt.Printf("  %s %-6s %s\n", status, label, value)
	}

	printField("OS:", fmt.Sprintf("%s (%s)", info.OS, info.Arch), info.OS != "")
	printField("Java:", info.JavaVersion, info.JavaVersion != "")
	printField("Go:", info.GoVersion, info.GoVersion != "")

	if info.SDKPath != "" {
		printField("SDK:", info.SDKPath, true)
	} else {
		printField("SDK:", "(not found)", false)
	}

	fmt.Println()

	if len(issues) > 0 {
		fmt.Println("Issues:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
	} else {
		fmt.Println("All prerequisites met.")
	}
}

func printUsage() {
	fmt.Println(`android-setup - Android SDK setup tool for Cheapskate Finance Tracker

Usage:
  android-setup <command> [flags]

Commands:
  setup    Install Android SDK, packages, and Gradle wrapper (default)
  check    Check prerequisites without installing anything
  help     Show this help message

Flags for setup:
  --skip-emulator    Skip emulator and system image installation
  --sdk-path=<path>  Install SDK to a custom path (default: ~/android-sdk)
  --verbose, -v      Show detailed output from SDK tools

Examples:
  android-setup                        Run full setup with defaults
  android-setup setup --skip-emulator  Skip emulator installation
  android-setup check                  Check what's already installed
  go run ./scripts/android-setup check Check from source`)
}
