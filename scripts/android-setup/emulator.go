package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const avdName = "cheapskate_test"

// InstallEmulator installs the Android emulator and an appropriate system image,
// then creates an AVD for testing.
func InstallEmulator(sdkPath string, info *SystemInfo, verbose bool) error {
	sdkmanager := filepath.Join(sdkPath, "cmdline-tools", "latest", "bin", "sdkmanager")
	image := selectSystemImage(info.Arch)

	packages := []string{"emulator", image}

	for _, pkg := range packages {
		fmt.Printf("  Installing: %s\n", pkg)

		cmd := exec.Command(sdkmanager, "--install", pkg, "--sdk_root="+sdkPath)
		if verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("installing %s: %w", pkg, err)
		}
	}

	fmt.Println("  Emulator packages installed.")
	fmt.Println()

	// Create AVD
	if err := CreateAVD(sdkPath, image); err != nil {
		return fmt.Errorf("creating AVD: %w", err)
	}

	return nil
}

// selectSystemImage returns the appropriate system image package name
// based on the host architecture.
func selectSystemImage(arch string) string {
	if arch == "arm64" {
		// Apple Silicon Macs or ARM64 Linux
		return "system-images;android-34;google_apis;arm64-v8a"
	}
	// x86_64 hosts — use x86_64 image with ARM translation (API 30+)
	return "system-images;android-34;google_apis;x86_64"
}

// CreateAVD creates an Android Virtual Device for testing.
func CreateAVD(sdkPath, systemImage string) error {
	avdmanager := filepath.Join(sdkPath, "cmdline-tools", "latest", "bin", "avdmanager")

	// Check if AVD already exists
	listCmd := exec.Command(avdmanager, "list", "avd", "-c")
	output, err := listCmd.Output()
	if err == nil {
		for _, line := range splitLines(string(output)) {
			if line == avdName {
				fmt.Printf("  AVD %q already exists, skipping creation.\n", avdName)
				return nil
			}
		}
	}

	fmt.Printf("  Creating AVD: %s\n", avdName)

	cmd := exec.Command(avdmanager, "create", "avd",
		"--name", avdName,
		"--package", systemImage,
		"--device", "pixel_6",
		"--force",
	)
	// avdmanager prompts "Do you wish to create a custom hardware profile?" — answer no
	cmd.Stdin = strings.NewReader("no\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running avdmanager: %w", err)
	}

	fmt.Printf("  AVD %q created.\n", avdName)
	return nil
}

// splitLines splits a string by newlines, trimming empty trailing entries.
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
