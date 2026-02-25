package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SetupOptions configures the setup process.
type SetupOptions struct {
	SDKPath      string // Custom SDK install path (default: ~/android-sdk)
	SkipEmulator bool   // Skip emulator installation
	Verbose      bool   // Show verbose output from SDK tools
}

// RunSetup orchestrates the full Android SDK setup.
func RunSetup(opts SetupOptions) error {
	fmt.Println()
	fmt.Println("Android SDK Setup for Cheapskate")
	fmt.Println("================================")
	fmt.Println()

	// Step 1: Check prerequisites
	fmt.Println("Checking prerequisites...")
	info, issues := CheckPrerequisites()

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

	// Filter out non-critical issues (missing SDK is expected)
	var criticalIssues []string
	for _, issue := range issues {
		if strings.HasPrefix(issue, "Java:") || strings.HasPrefix(issue, "Go:") || strings.HasPrefix(issue, "Unsupported") {
			criticalIssues = append(criticalIssues, issue)
		}
	}

	if len(criticalIssues) > 0 {
		fmt.Println()
		fmt.Println("Cannot proceed — fix these issues first:")
		for _, issue := range criticalIssues {
			fmt.Printf("  - %s\n", issue)
		}
		return fmt.Errorf("prerequisites not met")
	}

	// Step 2: Determine SDK path
	sdkPath := opts.SDKPath
	if sdkPath == "" {
		sdkPath = info.SDKPath
	}
	if sdkPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		sdkPath = filepath.Join(homeDir, "android-sdk")
	}

	if info.SDKPath != "" {
		fmt.Printf("\n  ✓ SDK:   %s (existing)\n", sdkPath)
	} else {
		fmt.Printf("\n  SDK will be installed to: %s\n", sdkPath)
	}

	// Step 3: Download and install SDK
	fmt.Println()
	fmt.Println("Installing Android SDK...")
	if err := DownloadAndInstallSDK(sdkPath, info, opts.Verbose); err != nil {
		return err
	}

	// Step 4: Optional emulator setup
	if !opts.SkipEmulator {
		fmt.Println()
		if promptYesNo("Install Android emulator? (large download, ~1 GB)") {
			fmt.Println()
			fmt.Println("Installing emulator...")
			if err := InstallEmulator(sdkPath, info, opts.Verbose); err != nil {
				return err
			}
		} else {
			fmt.Println("  Skipping emulator. You can install it later with:")
			fmt.Printf("    go run ./scripts/android-setup setup --sdk-path=%s\n", sdkPath)
		}
	}

	// Step 5: Generate Gradle wrapper
	fmt.Println()
	fmt.Println("Setting up Gradle wrapper...")
	androidDir, err := findAndroidDir()
	if err != nil {
		return err
	}
	if err := GenerateGradleWrapper(androidDir, opts.Verbose); err != nil {
		return err
	}

	// Step 6: Print success and next steps
	printSuccess(sdkPath)

	return nil
}

// findAndroidDir locates the android/ directory relative to the git repo root.
func findAndroidDir() (string, error) {
	// Try git repo root first
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err == nil {
		repoRoot := strings.TrimSpace(string(output))
		androidDir := filepath.Join(repoRoot, "android")
		if info, err := os.Stat(androidDir); err == nil && info.IsDir() {
			return androidDir, nil
		}
	}

	// Fallback: relative to working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}

	androidDir := filepath.Join(cwd, "android")
	if info, err := os.Stat(androidDir); err == nil && info.IsDir() {
		return androidDir, nil
	}

	return "", fmt.Errorf("android/ directory not found — run this from the project root")
}

// promptYesNo asks a yes/no question and returns true for yes.
// Defaults to no if input is empty or non-interactive.
func promptYesNo(question string) bool {
	fmt.Printf("%s (y/N): ", question)

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// printSuccess prints the final success message with next steps.
func printSuccess(sdkPath string) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Setup complete!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Add these to your ~/.bashrc or ~/.zshrc:")
	fmt.Println()
	fmt.Printf("  export ANDROID_HOME=\"%s\"\n", sdkPath)
	fmt.Printf("  export PATH=\"$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator:$PATH\"\n")
	fmt.Println()
	fmt.Println("Then reload your shell and build the APK:")
	fmt.Println()
	fmt.Println("  source ~/.bashrc    # or ~/.zshrc")
	fmt.Println("  make build-android-apk")
	fmt.Println()
	fmt.Println("The APK will be at:")
	fmt.Println("  android/app/build/outputs/apk/debug/app-debug.apk")
	fmt.Println()
	fmt.Println("Install it with:")
	fmt.Println("  adb install android/app/build/outputs/apk/debug/app-debug.apk")
}
