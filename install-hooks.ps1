Write-Host "Installing Git hooks..." -ForegroundColor Cyan

if (Test-Path ".githooks\pre-commit") {
    Write-Host "Found .githooks\pre-commit" -ForegroundColor Yellow
    
    if (!(Test-Path ".git\hooks")) {
        New-Item -ItemType Directory -Path ".git\hooks" -Force | Out-Null
        Write-Host "Created .git\hooks directory" -ForegroundColor Yellow
    }
    
    Copy-Item ".githooks\pre-commit" ".git\hooks\pre-commit" -Force
    icacls ".git\hooks\pre-commit" /grant Everyone:F | Out-Null
    
    Write-Host "Installed: pre-commit hook" -ForegroundColor Green
    Write-Host "Git hooks installation complete!" -ForegroundColor Green
} else {
    Write-Host "Error: .githooks\pre-commit not found" -ForegroundColor Red
}
