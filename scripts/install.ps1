#
# Install script for notion-sync (Windows PowerShell)
# Usage: irm https://raw.githubusercontent.com/ran-codes/notion-sync/main/scripts/install.ps1 | iex
#

$ErrorActionPreference = "Stop"

$repo = "ran-codes/notion-sync"
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$binaryName = "notion-sync-windows-${arch}.exe"

Write-Host "Detected: windows/$arch"
Write-Host "Downloading $binaryName..."

# Get latest release info
$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$asset = $release.assets | Where-Object { $_.name -eq $binaryName }

if (-not $asset) {
    Write-Error "Could not find release for $binaryName. Check https://github.com/$repo/releases"
    exit 1
}

# Download to temp file
$tmpFile = Join-Path $env:TEMP "notion-sync-download.exe"
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tmpFile

# Install to user's local bin
$installDir = Join-Path $env:LOCALAPPDATA "notion-sync"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir | Out-Null
}

$installPath = Join-Path $installDir "notion-sync.exe"
Move-Item -Force $tmpFile $installPath

# Add to PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    $env:Path = "$env:Path;$installDir"
    Write-Host "Added $installDir to your PATH."
}

Write-Host "Installed notion-sync to $installPath"
Write-Host ""
Write-Host "Get started:"
Write-Host "  notion-sync config set apiKey <your-notion-api-key>"
Write-Host "  notion-sync import <database-id> --output ./my-notes"
Write-Host ""
Write-Host "Run 'notion-sync --help' for more information."
