# agent-notify installer (Windows)
#   irm https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "CoderGangW/agent-notify"
$dir = Join-Path $env:LOCALAPPDATA "agent-notify"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$exe = Join-Path $dir "agent-notify.exe"

$url = "https://github.com/$repo/releases/latest/download/agent-notify-windows-amd64.exe"
Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile $exe

& $exe install
Write-Host "installed: $exe"
