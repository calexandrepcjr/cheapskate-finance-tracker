#!/bin/bash
#
# Installs git hooks for the Cheapskate Finance Tracker project
#
# Usage: ./scripts/setup-hooks.sh
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"
SOURCE_HOOKS_DIR="$SCRIPT_DIR/hooks"

echo "Installing git hooks..."

# Install pre-commit hook
if [ -f "$SOURCE_HOOKS_DIR/pre-commit" ]; then
    cp "$SOURCE_HOOKS_DIR/pre-commit" "$HOOKS_DIR/pre-commit"
    chmod +x "$HOOKS_DIR/pre-commit"
    echo "  Installed: pre-commit"
fi

# Install commit-msg hook (conventional commits validation)
if [ -f "$SOURCE_HOOKS_DIR/commit-msg" ]; then
    cp "$SOURCE_HOOKS_DIR/commit-msg" "$HOOKS_DIR/commit-msg"
    chmod +x "$HOOKS_DIR/commit-msg"
    echo "  Installed: commit-msg"
fi

echo ""
echo "Git hooks installed successfully!"
echo ""
echo "Hooks installed:"
echo "  - pre-commit:  Runs 'go test ./...' before each commit"
echo "  - commit-msg:  Enforces conventional commits format"
