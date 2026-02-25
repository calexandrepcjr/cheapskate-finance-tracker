package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// cmdlineToolsVersion is the latest stable Android command-line tools revision.
	cmdlineToolsVersion = "11076708"

	// cmdlineToolsURLTemplate is the download URL template.
	// Placeholders: {os} = "linux" or "mac", {version} = revision number.
	cmdlineToolsURLTemplate = "https://dl.google.com/android/repository/commandlinetools-%s-%s_latest.zip"
)

var requiredPackages = []string{
	"platform-tools",
	"platforms;android-34",
	"build-tools;34.0.0",
}

// DownloadAndInstallSDK downloads the Android command-line tools and installs
// the required SDK packages. It skips steps that are already complete.
func DownloadAndInstallSDK(sdkPath string, info *SystemInfo, verbose bool) error {
	cmdlineToolsDir := filepath.Join(sdkPath, "cmdline-tools", "latest")

	// Step 1: Download and extract command-line tools if not present
	if _, err := os.Stat(filepath.Join(cmdlineToolsDir, "bin", "sdkmanager")); os.IsNotExist(err) {
		if err := downloadCmdlineTools(sdkPath, info.OS); err != nil {
			return fmt.Errorf("downloading command-line tools: %w", err)
		}
	} else {
		fmt.Println("  Command-line tools already installed, skipping download.")
	}

	// Step 2: Accept licenses
	fmt.Println()
	fmt.Println("Accepting SDK licenses...")
	if err := acceptLicenses(sdkPath, verbose); err != nil {
		return fmt.Errorf("accepting licenses: %w", err)
	}

	// Step 3: Install required packages
	fmt.Println()
	fmt.Println("Installing SDK packages...")
	if err := installSDKPackages(sdkPath, verbose); err != nil {
		return fmt.Errorf("installing SDK packages: %w", err)
	}

	return nil
}

// downloadCmdlineTools downloads and extracts the Android command-line tools.
func downloadCmdlineTools(sdkPath, osName string) error {
	// Map Go OS name to Google's naming
	dlOS := osName
	if dlOS == "darwin" {
		dlOS = "mac"
	}

	url := fmt.Sprintf(cmdlineToolsURLTemplate, dlOS, cmdlineToolsVersion)
	zipName := fmt.Sprintf("commandlinetools-%s-%s_latest.zip", dlOS, cmdlineToolsVersion)

	fmt.Printf("  Downloading: %s\n", zipName)

	// Download to a temp file
	tmpFile, err := os.CreateTemp("", "android-cmdline-tools-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(url)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed with HTTP %d for %s", resp.StatusCode, url)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("saving download: %w", err)
	}
	tmpFile.Close()

	fmt.Printf("  Downloaded %.1f MB\n", float64(written)/(1024*1024))

	// Extract to SDK path
	// The zip contains cmdline-tools/ at the root, but we need it at
	// sdkPath/cmdline-tools/latest/
	destDir := filepath.Join(sdkPath, "cmdline-tools", "latest")
	fmt.Printf("  Extracting to %s\n", destDir)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", destDir, err)
	}

	if err := extractZip(tmpPath, destDir, "cmdline-tools/"); err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	fmt.Println("  Command-line tools installed.")
	return nil
}

// extractZip extracts a zip file to destDir. If stripPrefix is non-empty,
// that prefix is removed from each entry's path before extraction.
func extractZip(zipPath, destDir, stripPrefix string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name

		// Strip the prefix (e.g., "cmdline-tools/") from paths
		if stripPrefix != "" {
			if !strings.HasPrefix(name, stripPrefix) {
				continue
			}
			name = strings.TrimPrefix(name, stripPrefix)
		}

		if name == "" {
			continue
		}

		targetPath := filepath.Join(destDir, name)

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// acceptLicenses runs sdkmanager --licenses and pipes "yes" to accept all.
func acceptLicenses(sdkPath string, verbose bool) error {
	sdkmanager := filepath.Join(sdkPath, "cmdline-tools", "latest", "bin", "sdkmanager")

	cmd := exec.Command(sdkmanager, "--licenses", "--sdk_root="+sdkPath)
	// Pipe repeated "yes\n" to stdin to accept all licenses
	cmd.Stdin = strings.NewReader(strings.Repeat("yes\n", 20))

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running sdkmanager --licenses: %w", err)
	}

	fmt.Println("  Licenses accepted.")
	return nil
}

// installSDKPackages installs the required Android SDK packages.
func installSDKPackages(sdkPath string, verbose bool) error {
	sdkmanager := filepath.Join(sdkPath, "cmdline-tools", "latest", "bin", "sdkmanager")

	for _, pkg := range requiredPackages {
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

	fmt.Println("  All SDK packages installed.")
	return nil
}
