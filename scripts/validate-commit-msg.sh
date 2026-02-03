#!/bin/bash
#
# Validates a commit message against conventional commits format
# Used by Claude Code hooks to enforce conventional commits
#
# Usage: ./scripts/validate-commit-msg.sh "<commit message>"
#
# Exit codes:
#   0 - Valid conventional commit message
#   1 - Invalid format
#

set -e

COMMIT_MSG="$1"

if [ -z "$COMMIT_MSG" ]; then
    echo "Error: No commit message provided"
    exit 1
fi

# Skip merge commits
if echo "$COMMIT_MSG" | grep -qE "^Merge "; then
    exit 0
fi

# Skip revert commits
if echo "$COMMIT_MSG" | grep -qE "^Revert \""; then
    exit 0
fi

# Conventional commit regex pattern
PATTERN="^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9_-]+\))?: .+"

# Get first line of commit message
FIRST_LINE=$(echo "$COMMIT_MSG" | head -n1)

if ! echo "$FIRST_LINE" | grep -qE "$PATTERN"; then
    echo ""
    echo "=========================================="
    echo "INVALID COMMIT MESSAGE FORMAT"
    echo "=========================================="
    echo ""
    echo "Your message: \"$FIRST_LINE\""
    echo ""
    echo "Required format: <type>[scope]: <description>"
    echo ""
    echo "Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert"
    echo ""
    echo "Examples:"
    echo "  feat: add user authentication"
    echo "  fix(parser): handle edge case in amount parsing"
    echo "  docs: update API documentation"
    echo ""
    exit 1
fi

echo "Commit message format is valid (conventional commit)"
exit 0
