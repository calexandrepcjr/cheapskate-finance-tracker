// Package main provides a CLI tool for git hooks management.
// This tool validates commit messages and sets up git hooks.
//
// Usage:
//
//	hooks-cli validate-commit <message>    Validate a commit message
//	hooks-cli validate-commit-file <file>  Validate commit message from file
//	hooks-cli setup-hooks                  Install git hooks
//	hooks-cli run-tests                    Run test suite
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "validate-commit":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: commit message required")
			fmt.Fprintln(os.Stderr, "Usage: hooks-cli validate-commit <message>")
			os.Exit(1)
		}
		message := os.Args[2]
		if err := ValidateCommitMessage(message); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Commit message format validated: conventional commit")

	case "validate-commit-file":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: commit message file required")
			fmt.Fprintln(os.Stderr, "Usage: hooks-cli validate-commit-file <file>")
			os.Exit(1)
		}
		filePath := os.Args[2]
		if err := ValidateCommitMessageFile(filePath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Commit message format validated: conventional commit")

	case "setup-hooks":
		if err := SetupHooks(); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up hooks: %v\n", err)
			os.Exit(1)
		}

	case "run-tests":
		if err := RunTests(); err != nil {
			fmt.Fprintln(os.Stderr, err)
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

func printUsage() {
	fmt.Println(`hooks-cli - Git hooks management tool for Cheapskate Finance Tracker

Usage:
  hooks-cli <command> [arguments]

Commands:
  validate-commit <message>    Validate a commit message against conventional commits format
  validate-commit-file <file>  Validate commit message from a file (used by git hooks)
  setup-hooks                  Install git hooks (pre-commit and commit-msg)
  run-tests                    Run the test suite
  help                         Show this help message

Examples:
  hooks-cli validate-commit "feat: add new feature"
  hooks-cli validate-commit-file .git/COMMIT_EDITMSG
  hooks-cli setup-hooks
  hooks-cli run-tests`)
}
