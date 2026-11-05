$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $ProjectRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Go is not installed or is not available in PATH." -ForegroundColor Red
    Write-Host "Install Go, reopen PowerShell, and run this script again."
    exit 1
}

if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
    Write-Host "Created .env from .env.example." -ForegroundColor Yellow
    Write-Host "Paste your Gnani API key into .env. Typed commands can still run without it." -ForegroundColor Yellow
}

Write-Host "Starting Vyapari Voice Copilot..." -ForegroundColor Green
Write-Host "Open http://localhost:8080" -ForegroundColor Cyan
go run .\cmd\server
