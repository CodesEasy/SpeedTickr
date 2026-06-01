# SpeedTickr installer for Windows (PowerShell).
#
#   irm https://speedtickr.com/install.ps1 | iex
#
# From cmd.exe:
#   powershell -ExecutionPolicy Bypass -Command "irm https://speedtickr.com/install.ps1 | iex"
#
# Optional environment overrides:
#   $env:SPEEDTICKR_VERSION   release tag to install (default: latest)
#   $env:SPEEDTICKR_DIR       install directory (default: %LOCALAPPDATA%\Programs\SpeedTickr)
#
# SpeedTickr is a single self-contained .exe - this script only downloads it,
# verifies its checksum, and launches it. It installs per-user; no admin needed.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # faster Invoke-WebRequest

$repo    = 'codeseasy/speedtickr'
$version = if ($env:SPEEDTICKR_VERSION) { $env:SPEEDTICKR_VERSION } else { 'latest' }
$dir     = if ($env:SPEEDTICKR_DIR) { $env:SPEEDTICKR_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\SpeedTickr' }

function Info($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Warn($m) { Write-Host "warning: $m" -ForegroundColor Yellow }

# Detect architecture. PROCESSOR_ARCHITECTURE is the *process* arch, so a 32-bit
# or x64-emulated PowerShell on ARM64 Windows reports x86/AMD64; PROCESSOR_ARCHITEW6432
# (set only under WOW64 emulation) holds the real OS arch, so prefer it when present.
$procArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($procArch) {
  'ARM64' { 'arm64' }
  default { 'amd64' }
}
$asset = "speedtickr-windows-$arch.exe"

$base = if ($version -eq 'latest') {
  "https://github.com/$repo/releases/latest/download"
} else {
  "https://github.com/$repo/releases/download/$version"
}

Info "Downloading $asset ($version)..."
$tmp = Join-Path $env:TEMP $asset
try {
  Invoke-WebRequest -Uri "$base/$asset" -OutFile $tmp -UseBasicParsing
} catch {
  throw "download failed - is the $version release published yet? ($base/$asset)"
}

# Verify the download against the published SHA256SUMS before trusting it.
Info 'Verifying checksum...'
$sums = $null
try { $sums = (Invoke-WebRequest -Uri "$base/SHA256SUMS" -UseBasicParsing).Content } catch { }
# Windows PowerShell 5.1 (what `irm | iex` runs by default) returns .Content as a
# byte[] for non-text responses, and GitHub serves release assets as
# application/octet-stream. Decode to text before parsing, or every "line" is a byte
# number, nothing matches the asset name, and verification is silently skipped.
if ($sums -is [byte[]]) { $sums = [System.Text.Encoding]::UTF8.GetString($sums) }
if (-not $sums) {
  Warn 'could not download SHA256SUMS - skipping verification'
} else {
  $line = ($sums -split "`n") | Where-Object { $_ -match ('\s' + [regex]::Escape($asset) + '\s*$') } | Select-Object -First 1
  if (-not $line) {
    Warn "no checksum listed for $asset - skipping verification"
  } else {
    $want = ($line -split '\s+')[0].ToLower()
    $got  = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLower()
    if ($want -ne $got) {
      Remove-Item -Force $tmp -ErrorAction SilentlyContinue
      throw "checksum mismatch for $asset! expected $want, got $got. The download may be corrupt or tampered with - aborting."
    }
    Info "Checksum OK - $want"
  }
}

# Stop any running instance so its .exe isn't locked. This also makes re-running
# the installer act as an in-place update.
$running = Get-Process -Name 'speedtickr' -ErrorAction SilentlyContinue
if ($running) {
  $running | Stop-Process -Force -ErrorAction SilentlyContinue
  $running | Wait-Process -Timeout 5 -ErrorAction SilentlyContinue
}

New-Item -ItemType Directory -Force -Path $dir | Out-Null
$out = Join-Path $dir 'speedtickr.exe'

# Overwrite any existing copy. Copy-Item -Force reliably replaces the file (unlike
# Move-Item, which can fail when the destination already exists); retry briefly in
# case the OS hasn't released the lock from the just-stopped instance yet.
for ($i = 1; $i -le 15; $i++) {
  try { Copy-Item -Force -LiteralPath $tmp -Destination $out; break }
  catch {
    if ($i -eq 15) { throw "could not write ${out} (is SpeedTickr still running?): $($_.Exception.Message)" }
    Start-Sleep -Milliseconds 200
  }
}
Remove-Item -Force -LiteralPath $tmp -ErrorAction SilentlyContinue
Info "Installed $out"

# Launch it - SpeedTickr puts the meter on the taskbar and enables start-at-login itself.
Start-Process $out
Write-Host "==> SpeedTickr is running on your taskbar and will start at login." -ForegroundColor Green
