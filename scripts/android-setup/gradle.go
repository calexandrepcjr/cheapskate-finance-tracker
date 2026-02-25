package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	gradleVersion = "8.7"
	gradleDistURL = "https://services.gradle.org/distributions/gradle-8.7-bin.zip"
)

// GenerateGradleWrapper generates the Gradle wrapper JAR in the android/ directory.
// It first checks if the wrapper already exists, then tries a local gradle installation,
// and falls back to downloading Gradle if needed.
func GenerateGradleWrapper(androidDir string, verbose bool) error {
	wrapperJar := filepath.Join(androidDir, "gradle", "wrapper", "gradle-wrapper.jar")

	// Skip if wrapper JAR already exists
	if _, err := os.Stat(wrapperJar); err == nil {
		fmt.Println("  Gradle wrapper JAR already exists, skipping.")
		return nil
	}

	gradleBin, cleanup, err := findOrDownloadGradle()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	return runGradleWrapper(gradleBin, androidDir, verbose)
}

// findOrDownloadGradle checks for a local gradle installation, and if not found,
// downloads the Gradle distribution to a temp directory.
// Returns the path to the gradle binary and an optional cleanup function.
func findOrDownloadGradle() (string, func(), error) {
	// Try to find gradle on PATH
	if path, err := exec.LookPath("gradle"); err == nil {
		fmt.Println("  Using system Gradle.")
		return path, nil, nil
	}

	// Download Gradle distribution
	fmt.Printf("  Gradle not found on PATH — downloading Gradle %s...\n", gradleVersion)

	tmpDir, err := os.MkdirTemp("", "gradle-download-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp directory: %w", err)
	}

	cleanup := func() {
		fmt.Println("  Cleaning up temporary Gradle download.")
		os.RemoveAll(tmpDir)
	}

	zipPath := filepath.Join(tmpDir, "gradle.zip")
	if err := downloadFile(gradleDistURL, zipPath); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("downloading Gradle: %w", err)
	}

	// Extract the distribution
	fmt.Println("  Extracting Gradle distribution...")
	if err := extractZip(zipPath, tmpDir, ""); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extracting Gradle: %w", err)
	}

	// The zip extracts to gradle-{version}/
	gradleBinName := "gradle"
	if runtime.GOOS == "windows" {
		gradleBinName = "gradle.bat"
	}
	gradleBin := filepath.Join(tmpDir, fmt.Sprintf("gradle-%s", gradleVersion), "bin", gradleBinName)

	if _, err := os.Stat(gradleBin); os.IsNotExist(err) {
		cleanup()
		return "", nil, fmt.Errorf("gradle binary not found at expected path: %s", gradleBin)
	}

	return gradleBin, cleanup, nil
}

// runGradleWrapper executes "gradle wrapper --gradle-version=8.7" in the android directory.
func runGradleWrapper(gradleBin, androidDir string, verbose bool) error {
	fmt.Printf("  Running: gradle wrapper --gradle-version=%s\n", gradleVersion)

	cmd := exec.Command(gradleBin, "wrapper", "--gradle-version="+gradleVersion)
	cmd.Dir = androidDir

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running gradle wrapper: %w", err)
	}

	fmt.Println("  Gradle wrapper generated.")
	return nil
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("  Downloaded %.1f MB\n", float64(written)/(1024*1024))
	return nil
}
