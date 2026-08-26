# claude-notify installer (Windows)
#   irm https://raw.githubusercontent.com/CoderGangW/claude-notify/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "CoderGangW/claude-notify"
$dir = Join-Path $env:LOCALAPPDATA "claude-notify"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$exe = Join-Path $dir "claude-notify.exe"

$url = "https://github.com/$repo/releases/latest/download/claude-notify-windows-amd64.exe"
Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile $exe

& $exe install
Write-Host "installed: $exe"
