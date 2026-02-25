package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectOS(t *testing.T) {
	got := detectOS()

	switch runtime.GOOS {
	case "linux":
		if got != "linux" {
			t.Errorf("detectOS() = %q on linux, want \"linux\"", got)
		}
	case "darwin":
		if got != "darwin" {
			t.Errorf("detectOS() = %q on darwin, want \"darwin\"", got)
		}
	default:
		if got != "" {
			t.Errorf("detectOS() = %q on %s, want \"\" (unsupported)", got, runtime.GOOS)
		}
	}
}

func TestDetectArch(t *testing.T) {
	got := detectArch()

	switch runtime.GOARCH {
	case "amd64":
		if got != "x86_64" {
			t.Errorf("detectArch() = %q, want \"x86_64\"", got)
		}
	case "arm64":
		if got != "arm64" {
			t.Errorf("detectArch() = %q, want \"arm64\"", got)
		}
	default:
		// For other architectures, just verify it returns something
		if got == "" {
			t.Error("detectArch() returned empty string")
		}
	}
}

func TestParseJavaVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "OpenJDK 21",
			output: `openjdk version "21.0.2" 2024-01-16`,
			want:   "21.0.2",
		},
		{
			name:   "OpenJDK 17",
			output: `openjdk version "17.0.9" 2023-10-17`,
			want:   "17.0.9",
		},
		{
			name:   "Oracle Java 8",
			output: `java version "1.8.0_392"`,
			want:   "1.8.0_392",
		},
		{
			name: "multiline OpenJDK output",
			output: `openjdk version "21.0.2" 2024-01-16
OpenJDK Runtime Environment (build 21.0.2+13-58)
OpenJDK 64-Bit Server VM (build 21.0.2+13-58, mixed mode, sharing)`,
			want: "21.0.2",
		},
		{
			name:   "no version",
			output: "something else entirely",
			want:   "",
		},
		{
			name:   "empty",
			output: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJavaVersion(tt.output)
			if got != tt.want {
				t.Errorf("parseJavaVersion(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestJavaMajorVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    int
		wantErr bool
	}{
		{name: "Java 21", version: "21.0.2", want: 21},
		{name: "Java 17", version: "17.0.9", want: 17},
		{name: "Java 8 legacy", version: "1.8.0_392", want: 8},
		{name: "Java 11", version: "11.0.20", want: 11},
		{name: "simple major", version: "17", want: 17},
		{name: "empty", version: "", wantErr: true},
		{name: "garbage", version: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := javaMajorVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("javaMajorVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("javaMajorVersion(%q) = %d, want %d", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseGoVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "standard output",
			output: "go version go1.25.0 linux/amd64",
			want:   "1.25.0",
		},
		{
			name:   "two-part version",
			output: "go version go1.25 darwin/arm64",
			want:   "1.25",
		},
		{
			name:   "no match",
			output: "something else",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoVersion(tt.output)
			if got != tt.want {
				t.Errorf("parseGoVersion(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestSelectSystemImage(t *testing.T) {
	tests := []struct {
		arch string
		want string
	}{
		{arch: "arm64", want: "system-images;android-34;google_apis;arm64-v8a"},
		{arch: "x86_64", want: "system-images;android-34;google_apis;x86_64"},
		{arch: "anything_else", want: "system-images;android-34;google_apis;x86_64"},
	}

	for _, tt := range tests {
		t.Run(tt.arch, func(t *testing.T) {
			got := selectSystemImage(tt.arch)
			if got != tt.want {
				t.Errorf("selectSystemImage(%q) = %q, want %q", tt.arch, got, tt.want)
			}
		})
	}
}

func TestFindExistingSDK_WithEnvVar(t *testing.T) {
	// Create a fake SDK directory with a marker subdirectory
	tmpDir, err := os.MkdirTemp("", "android-setup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a marker directory so isSDKDir returns true
	if err := os.MkdirAll(filepath.Join(tmpDir, "platform-tools"), 0755); err != nil {
		t.Fatalf("failed to create marker dir: %v", err)
	}

	// Set ANDROID_HOME to our fake SDK
	original := os.Getenv("ANDROID_HOME")
	os.Setenv("ANDROID_HOME", tmpDir)
	defer os.Setenv("ANDROID_HOME", original)

	got := findExistingSDK()
	if got != tmpDir {
		t.Errorf("findExistingSDK() = %q, want %q", got, tmpDir)
	}
}

func TestFindExistingSDK_Empty(t *testing.T) {
	// Clear env vars
	origHome := os.Getenv("ANDROID_HOME")
	origRoot := os.Getenv("ANDROID_SDK_ROOT")
	os.Unsetenv("ANDROID_HOME")
	os.Unsetenv("ANDROID_SDK_ROOT")
	defer func() {
		os.Setenv("ANDROID_HOME", origHome)
		os.Setenv("ANDROID_SDK_ROOT", origRoot)
	}()

	// findExistingSDK may or may not find something depending on the host,
	// but it shouldn't panic or error
	_ = findExistingSDK()
}

func TestIsSDKDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "android-setup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty directory is not an SDK
	if isSDKDir(tmpDir) {
		t.Error("isSDKDir() should return false for empty directory")
	}

	// Non-existent path
	if isSDKDir("/nonexistent/path") {
		t.Error("isSDKDir() should return false for non-existent path")
	}

	// With a marker directory
	os.MkdirAll(filepath.Join(tmpDir, "platforms"), 0755)
	if !isSDKDir(tmpDir) {
		t.Error("isSDKDir() should return true with platforms/ subdirectory")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "one line", input: "hello\n", want: 1},
		{name: "two lines", input: "hello\nworld\n", want: 2},
		{name: "blank lines filtered", input: "hello\n\n\nworld\n", want: 2},
		{name: "whitespace only lines filtered", input: "hello\n   \nworld\n", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitLines(%q) returned %d lines, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestCheckPrerequisites_ReturnsSystemInfo(t *testing.T) {
	info, _ := CheckPrerequisites()

	if info == nil {
		t.Fatal("CheckPrerequisites() returned nil SystemInfo")
	}

	// OS should always be detected on linux/darwin
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if info.OS == "" {
			t.Error("OS should be detected on linux/darwin")
		}
	}

	// Arch should always be non-empty
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
}
