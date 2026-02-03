package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteHook(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "hooks-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hookPath := filepath.Join(tmpDir, "test-hook")
	hookContent := "#!/bin/sh\necho 'test'\n"

	err = writeHook(hookPath, hookContent)
	if err != nil {
		t.Fatalf("writeHook() error = %v", err)
	}

	// Verify file was created
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Hook file was not created: %v", err)
	}

	// Verify file is executable (on Unix systems)
	if runtime.GOOS != "windows" {
		mode := info.Mode()
		if mode&0111 == 0 {
			t.Error("Hook file should be executable")
		}
	}

	// Verify content
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook file: %v", err)
	}
	if string(content) != hookContent {
		t.Errorf("Hook content = %q, want %q", string(content), hookContent)
	}
}

func TestGetBinaryName(t *testing.T) {
	name := GetBinaryName()

	if runtime.GOOS == "windows" {
		if name != "hooks-cli.exe" {
			t.Errorf("GetBinaryName() = %q, want 'hooks-cli.exe' on Windows", name)
		}
	} else {
		if name != "hooks-cli" {
			t.Errorf("GetBinaryName() = %q, want 'hooks-cli' on Unix", name)
		}
	}
}

func TestPreCommitHookContent(t *testing.T) {
	// Verify pre-commit hook contains expected content
	if !containsHelper(preCommitHook, "#!/bin/sh") {
		t.Error("pre-commit hook should have shebang")
	}
	if !containsHelper(preCommitHook, "go test") {
		t.Error("pre-commit hook should run go test")
	}
	if !containsHelper(preCommitHook, "COMMIT REJECTED") {
		t.Error("pre-commit hook should have rejection message")
	}
}

func TestCommitMsgHookContent(t *testing.T) {
	// Verify commit-msg hook contains expected content
	if !containsHelper(commitMsgHook, "#!/bin/sh") {
		t.Error("commit-msg hook should have shebang")
	}
	if !containsHelper(commitMsgHook, "hooks-cli") {
		t.Error("commit-msg hook should reference hooks-cli")
	}
	if !containsHelper(commitMsgHook, "validate-commit-file") {
		t.Error("commit-msg hook should call validate-commit-file")
	}
}
