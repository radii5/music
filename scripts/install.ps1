# radii5 installer for Windows
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/radii5/radii5/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo      = "radii5/music"
$installDir = "$env:LOCALAPPDATA\radii5"

# ── arch detection ────────────────────────────────────────────────────────────
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$suffix = switch ($arch) {
    "Arm64" { "windows-arm64" }
    default { "windows-amd64" }
}

Write-Host ""
Write-Host "  radii5 installer" -ForegroundColor Cyan
Write-Host "  platform: windows-$($suffix.Split('-')[1])" -ForegroundColor DarkGray
Write-Host ""

# ── ensure install dir ────────────────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# ── helper: chunked download ──────────────────────────────────────────────────
function Get-FileChunked {
    param(
        [string]$Url,
        [string]$Dest,
        [int]$Threads = 8
    )

    # HEAD to get size
    try {
        $head = Invoke-WebRequest -Uri $Url -Method Head -UseBasicParsing
        $total = [long]$head.Headers["Content-Length"]
    } catch {
        $total = 0
    }

    if ($total -le 0 -or $Threads -le 1) {
        # Simple fallback
        $wc = New-Object System.Net.WebClient
        $wc.DownloadFile($Url, $Dest)
        return
    }

    $chunkSize = [math]::Floor($total / $Threads)
    $jobs      = @()
    $tmpFiles  = @()

    for ($i = 0; $i -lt $Threads; $i++) {
        $start = $i * $chunkSize
        $end   = if ($i -eq $Threads - 1) { $total - 1 } else { $start + $chunkSize - 1 }
        $tmp   = [System.IO.Path]::GetTempFileName()
        $tmpFiles += $tmp

        $jobs += Start-Job -ScriptBlock {
            param($url, $start, $end, $tmp)
            $req = [System.Net.HttpWebRequest]::Create($url)
            $req.AddRange("bytes", $start, $end)
            $resp   = $req.GetResponse()
            $stream = $resp.GetResponseStream()
            $out    = [System.IO.File]::OpenWrite($tmp)
            $buf    = New-Object byte[] 65536
            while (($n = $stream.Read($buf, 0, $buf.Length)) -gt 0) {
                $out.Write($buf, 0, $n)
            }
            $out.Close()
            $stream.Close()
        } -ArgumentList $Url, $start, $end, $tmp
    }

    # Show a simple spinner while waiting
    $spin = @("|", "/", "-", "\")
    $si   = 0
    while ($jobs | Where-Object { $_.State -eq "Running" }) {
        Write-Host -NoNewline "`r  $($spin[$si % 4]) downloading ($Threads threads)…  " -ForegroundColor Cyan
        $si++
        Start-Sleep -Milliseconds 120
    }

    $jobs | Wait-Job | Out-Null
    $failed = $jobs | Where-Object { $_.State -ne "Completed" }
    if ($failed) {
        Write-Host "`r  ✗ One or more chunks failed" -ForegroundColor Red
        $jobs | Remove-Job
        exit 1
    }
    $jobs | Remove-Job

    # Assemble
    $out = [System.IO.File]::OpenWrite($Dest)
    foreach ($tmp in $tmpFiles) {
        $bytes = [System.IO.File]::ReadAllBytes($tmp)
        $out.Write($bytes, 0, $bytes.Length)
        Remove-Item $tmp -Force
    }
    $out.Close()

    $mb = [math]::Round($total / 1MB, 1)
    Write-Host "`r  [$('█' * 30)]  ${mb} MB ✓" -ForegroundColor Green
}

# ── resolve latest radii5 release ────────────────────────────────────────────
Write-Host "  → radii5" -ForegroundColor Cyan
try {
    $rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" `
        -Headers @{ "User-Agent" = "radii5-installer"; "Accept" = "application/vnd.github+json" }
} catch {
    Write-Host "  ✗ Could not fetch radii5 release. Is the repo published?" -ForegroundColor Red
    exit 1
}

$assetName = "radii5-$suffix.exe"
$asset     = $rel.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) {
    Write-Host "  ✗ No asset found for $assetName" -ForegroundColor Red
    exit 1
}

$r5Dest = Join-Path $installDir "radii5.exe"
Write-Host "  version  $($rel.tag_name)" -ForegroundColor DarkGray
Write-Host "  dest     $r5Dest" -ForegroundColor DarkGray
Write-Host ""
Get-FileChunked -Url $asset.browser_download_url -Dest $r5Dest -Threads 8
Write-Host "  ✓ radii5 $($rel.tag_name)" -ForegroundColor Green
Write-Host ""

# ── yt-dlp ────────────────────────────────────────────────────────────────────
$ytCmd = Get-Command "yt-dlp.exe" -ErrorAction SilentlyContinue
if ($ytCmd) {
    Write-Host "  ✓ yt-dlp already installed" -ForegroundColor DarkGray
} else {
    Write-Host "  → yt-dlp (latest)" -ForegroundColor Cyan
    $ytRel = Invoke-RestMethod "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest" `
        -Headers @{ "User-Agent" = "radii5-installer"; "Accept" = "application/vnd.github+json" }

    $ytAsset = $ytRel.assets | Where-Object { $_.name -eq "yt-dlp.exe" } | Select-Object -First 1
    if (-not $ytAsset) {
        Write-Host "  ✗ yt-dlp asset not found" -ForegroundColor Red
        exit 1
    }

    $ytDest = Join-Path $installDir "yt-dlp.exe"
    Write-Host "  version  $($ytRel.tag_name)" -ForegroundColor DarkGray
    Write-Host "  dest     $ytDest" -ForegroundColor DarkGray
    Write-Host ""
    Get-FileChunked -Url $ytAsset.browser_download_url -Dest $ytDest -Threads 8
    Write-Host "  ✓ yt-dlp $($ytRel.tag_name)" -ForegroundColor Green
    Write-Host ""
}

# ── PATH hint ─────────────────────────────────────────────────────────────────
$currentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -notlike "*$installDir*") {
    [System.Environment]::SetEnvironmentVariable(
        "PATH", "$currentPath;$installDir", "User"
    )
    Write-Host "  ✓ Added $installDir to your PATH" -ForegroundColor Green
    Write-Host "  (Restart your terminal for changes to take effect)" -ForegroundColor DarkGray
} else {
    Write-Host "  ✓ $installDir already in PATH" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "  All done! Try:  radii5 --version" -ForegroundColor Cyan
Write-Host ""
