package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Hook templates - these are the actual git hooks that will be installed
const preCommitHook = `#!/bin/sh
#
# Git pre-commit hook for Cheapskate Finance Tracker
# Runs tests before allowing commits
#
# This hook is installed by: hooks-cli setup-hooks
#

set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"

echo "Running pre-commit tests..."

cd "$REPO_ROOT"
if ! go test ./... -v; then
    echo ""
    echo "=========================================="
    echo "COMMIT REJECTED: Tests failed!"
    echo "=========================================="
    echo ""
    echo "Please fix the failing tests before committing."
    echo "Run 'go test ./... -v' to see detailed output."
    exit 1
fi

echo ""
echo "All tests passed. Proceeding with commit..."
`

const commitMsgHook = `#!/bin/sh
#
# Git commit-msg hook for Cheapskate Finance Tracker
# Enforces conventional commits format
#
# This hook is installed by: hooks-cli setup-hooks
#

set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
COMMIT_MSG_FILE="$1"

# Try to use the hooks-cli binary if it exists
if [ -x "$REPO_ROOT/bin/hooks-cli" ]; then
    exec "$REPO_ROOT/bin/hooks-cli" validate-commit-file "$COMMIT_MSG_FILE"
fi

# Fallback: run via go run if binary doesn't exist
cd "$REPO_ROOT"
exec go run ./scripts/hooks-cli validate-commit-file "$COMMIT_MSG_FILE"
`

// SetupHooks installs git hooks for the repository
func SetupHooks() error {
	// Find git directory
	gitDir, err := findGitDir()
	if err != nil {
		return err
	}

	hooksDir := filepath.Join(gitDir, "hooks")

	// Create hooks directory if it doesn't exist
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	fmt.Println("Installing git hooks...")

	// Install pre-commit hook
	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	if err := writeHook(preCommitPath, preCommitHook); err != nil {
		return fmt.Errorf("failed to install pre-commit hook: %w", err)
	}
	fmt.Println("  Installed: pre-commit")

	// Install commit-msg hook
	commitMsgPath := filepath.Join(hooksDir, "commit-msg")
	if err := writeHook(commitMsgPath, commitMsgHook); err != nil {
		return fmt.Errorf("failed to install commit-msg hook: %w", err)
	}
	fmt.Println("  Installed: commit-msg")

	fmt.Println("")
	fmt.Println("Git hooks installed successfully!")
	fmt.Println("")
	fmt.Println("Hooks installed:")
	fmt.Println("  - pre-commit:  Runs 'go test ./...' before each commit")
	fmt.Println("  - commit-msg:  Enforces conventional commits format")

	return nil
}

// writeHook writes a hook script to the specified path and makes it executable
func writeHook(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return err
	}
	return nil
}

// findGitDir finds the .git directory for the current repository
func findGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or git not installed): %w", err)
	}

	gitDir := filepath.Clean(string(output[:len(output)-1])) // Remove trailing newline

	// Make absolute if relative
	if !filepath.IsAbs(gitDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		gitDir = filepath.Join(cwd, gitDir)
	}

	return gitDir, nil
}

// RunTests runs the test suite
func RunTests() error {
	fmt.Println("Running pre-commit tests...")

	// Find repository root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	repoRoot := string(output[:len(output)-1])

	// Run tests
	testCmd := exec.Command("go", "test", "./...", "-v")
	testCmd.Dir = repoRoot
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "==========================================")
		fmt.Fprintln(os.Stderr, "COMMIT REJECTED: Tests failed!")
		fmt.Fprintln(os.Stderr, "==========================================")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please fix the failing tests before committing.")
		fmt.Fprintln(os.Stderr, "Run 'go test ./... -v' to see detailed output.")
		return fmt.Errorf("tests failed")
	}

	fmt.Println("")
	fmt.Println("All tests passed. Proceeding with commit...")
	return nil
}

// GetBinaryName returns the appropriate binary name for the current OS
func GetBinaryName() string {
	if runtime.GOOS == "windows" {
		return "hooks-cli.exe"
	}
	return "hooks-cli"
}
