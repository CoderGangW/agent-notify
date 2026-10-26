# agent-notify installer (Windows)
#   irm https://raw.githubusercontent.com/CoderGangW/agent-notify/main/install.ps1 | iex
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue" # PS 5.1's progress bar slows downloads to a crawl

$repo = "CoderGangW/agent-notify"
$exe = Join-Path (Join-Path $env:LOCALAPPDATA "agent-notify") "agent-notify.exe"
# Download beside, never onto, the installed exe: a running daemon locks it.
# `install` swaps the copy into place (renaming the running one away).
$tmp = Join-Path $env:TEMP "agent-notify-setup.exe"

$url = "https://github.com/$repo/releases/latest/download/agent-notify-windows-amd64.exe"
Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing

# GUI-subsystem exe: PowerShell neither waits for it nor shows its output unless piped
& $tmp install | Out-Host
Remove-Item $tmp -ErrorAction SilentlyContinue
Write-Host "installed: $exe"
