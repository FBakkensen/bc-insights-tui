#!/bin/bash
# Script to install Git hooks for all team members

echo "🔧 Installing Git hooks..."

# Check if we're in a Git repository
if [ ! -d ".git" ]; then
    echo "❌ Error: Not in a Git repository root"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p .git/hooks

# Copy hooks from .githooks to .git/hooks
if [ -d ".githooks" ]; then
    for hook in .githooks/*; do
        if [ -f "$hook" ]; then
            hook_name=$(basename "$hook")
            cp "$hook" ".git/hooks/$hook_name"
            chmod +x ".git/hooks/$hook_name"
            echo "✅ Installed: $hook_name"
        fi
    done
    echo "🎉 Git hooks installation complete!"
    echo ""
    echo "ℹ️  Hooks installed:"
    ls -la .git/hooks/
else
    echo "❌ Error: .githooks directory not found"
    exit 1
fi
