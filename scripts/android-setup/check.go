package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// SystemInfo holds detected system information.
type SystemInfo struct {
	OS          string // "linux" or "darwin"
	Arch        string // "x86_64" or "arm64"
	JavaVersion string // e.g. "21.0.2" or "" if not found
	GoVersion   string // e.g. "1.25.0" or "" if not found
	SDKPath     string // path to existing SDK or "" if not found
}

// CheckPrerequisites detects the system environment and returns any issues found.
func CheckPrerequisites() (*SystemInfo, []string) {
	info := &SystemInfo{}
	var issues []string

	info.OS = detectOS()
	info.Arch = detectArch()

	if info.OS == "" {
		issues = append(issues, fmt.Sprintf("Unsupported OS: %s (need linux or macOS)", runtime.GOOS))
	}

	javaVer, err := checkJava()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Java: %v", err))
	} else {
		info.JavaVersion = javaVer
	}

	goVer, err := checkGo()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Go: %v", err))
	} else {
		info.GoVersion = goVer
	}

	info.SDKPath = findExistingSDK()

	return info, issues
}

// detectOS returns "linux" or "darwin", or "" for unsupported platforms.
func detectOS() string {
	switch runtime.GOOS {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	default:
		return ""
	}
}

// detectArch returns "x86_64" or "arm64" based on the host architecture.
func detectArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// checkJava verifies that Java 17+ is installed and returns the version string.
func checkJava() (string, error) {
	cmd := exec.Command("java", "-version")
	// java -version outputs to stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Java not found — install JDK 17+ (https://adoptium.net)")
	}

	version := parseJavaVersion(string(output))
	if version == "" {
		return "", fmt.Errorf("could not parse Java version from: %s", strings.TrimSpace(string(output)))
	}

	major, err := javaMajorVersion(version)
	if err != nil {
		return "", fmt.Errorf("could not parse Java major version from %q: %v", version, err)
	}

	if major < 17 {
		return "", fmt.Errorf("Java %s found but 17+ required — upgrade at https://adoptium.net", version)
	}

	return version, nil
}

// parseJavaVersion extracts the version number from java -version output.
// Handles formats like:
//
//	openjdk version "21.0.2" 2024-01-16
//	java version "1.8.0_392"
//	openjdk version "17.0.9" 2023-10-17
func parseJavaVersion(output string) string {
	re := regexp.MustCompile(`(?:java|openjdk) version "([^"]+)"`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// javaMajorVersion extracts the major version number.
// "21.0.2" → 21, "1.8.0_392" → 8, "17" → 17
func javaMajorVersion(version string) (int, error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty version string")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	// Legacy format: "1.8.0_xxx" means Java 8
	if major == 1 && len(parts) >= 2 {
		return strconv.Atoi(parts[1])
	}

	return major, nil
}

// checkGo verifies that Go is installed and returns the version string.
func checkGo() (string, error) {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Go not found — install from https://go.dev/dl")
	}

	version := parseGoVersion(string(output))
	if version == "" {
		return "", fmt.Errorf("could not parse Go version from: %s", strings.TrimSpace(string(output)))
	}

	return version, nil
}

// parseGoVersion extracts the version from "go version go1.25.0 linux/amd64".
func parseGoVersion(output string) string {
	re := regexp.MustCompile(`go(\d+\.\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// findExistingSDK checks common locations for an existing Android SDK.
// Returns the path if found, or "" if not.
func findExistingSDK() string {
	// 1. Check ANDROID_HOME env var
	if home := os.Getenv("ANDROID_HOME"); home != "" {
		if isSDKDir(home) {
			return home
		}
	}

	// 2. Check ANDROID_SDK_ROOT (deprecated but still common)
	if root := os.Getenv("ANDROID_SDK_ROOT"); root != "" {
		if isSDKDir(root) {
			return root
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 3. Check common default locations
	candidates := []string{
		filepath.Join(homeDir, "android-sdk"),
		filepath.Join(homeDir, "Android", "Sdk"),                              // Android Studio default (Linux)
		filepath.Join(homeDir, "Library", "Android", "sdk"),                   // Android Studio default (macOS)
		filepath.Join(homeDir, "AppData", "Local", "Android", "Sdk"),          // Windows (just in case)
	}

	for _, candidate := range candidates {
		if isSDKDir(candidate) {
			return candidate
		}
	}

	return ""
}

// isSDKDir checks if a directory looks like an Android SDK installation.
func isSDKDir(path string) bool {
	// An SDK directory should exist and contain at least one of these subdirectories
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	markers := []string{"cmdline-tools", "platform-tools", "platforms", "build-tools"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}

	return false
}
