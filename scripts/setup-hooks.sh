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

echo ""
echo "Git hooks installed successfully!"
echo "The pre-commit hook will run 'go test ./...' before each commit."
