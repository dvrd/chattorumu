#!/bin/bash

# Setup git hooks for the project
# Run this script after cloning or pulling the repository

set -e

echo "📦 Setting up git hooks..."

# Configure git to use .githooks directory
git config core.hooksPath .githooks

echo "✅ Git hooks configured"
echo ""
echo "Installed hooks:"
echo "  • pre-push: Runs linting before push"
echo ""
echo "To bypass hooks (not recommended), use: git push --no-verify"
