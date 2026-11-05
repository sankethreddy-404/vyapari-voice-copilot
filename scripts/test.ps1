$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $ProjectRoot

Write-Host "Formatting Go files..." -ForegroundColor Cyan
gofmt -w .\cmd .\internal

Write-Host "Running unit tests..." -ForegroundColor Cyan
go test .\...

Write-Host "Running go vet..." -ForegroundColor Cyan
go vet .\...

if (Get-Command node -ErrorAction SilentlyContinue) {
    Write-Host "Checking frontend JavaScript syntax..." -ForegroundColor Cyan
    node --check .\web\app.js
    node --check .\web\service-worker.js
}

Write-Host "All checks passed." -ForegroundColor Green
