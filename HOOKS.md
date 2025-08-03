# Git Hooks Setup

This repository includes shared Git hooks to maintain code quality and workflow standards.

## 🔧 Installation

Run this command **once** after cloning the repository:

```bash
# On Unix/Linux/macOS:
./install-hooks.sh

# On Windows (PowerShell):
.\install-hooks.ps1
```

## 🛡️ Installed Hooks

### Pre-commit Hook
- **Purpose**: Prevents direct commits to the `main` branch
- **Action**: Blocks commits and suggests creating a feature branch
- **Bypass**: Use `git commit --no-verify` (not recommended)

## 📋 Team Workflow

1. **Always work on feature branches**: `git checkout -b feature/your-feature`
2. **Create pull requests** for all changes to main branch
3. **Let the hooks guide you** - they're there to help maintain quality

## 🔄 Updating Hooks

When hooks are updated in the repository:
1. Pull the latest changes: `git pull`  
2. Re-run the installation: `./install-hooks.sh`

## 🚫 Main Branch Protection

The pre-commit hook prevents these common mistakes:
- Accidental commits directly to `main`
- Bypassing the pull request workflow
- Breaking the protected branch workflow

This works in combination with GitHub's branch protection rules for complete protection.
