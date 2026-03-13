# radii5 installer for Windows
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/radii5/music/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$repo       = "radii5/music"
$installDir = "$env:LOCALAPPDATA\radii5"
$threads    = 8

# ── arch ──────────────────────────────────────────────────────────────────────
$arch   = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$suffix = if ($arch -eq "Arm64") { "windows-arm64" } else { "windows-amd64" }

Write-Host ""
Write-Host "  radii5 installer" -ForegroundColor Cyan
Write-Host "  platform: $suffix" -ForegroundColor DarkGray
Write-Host ""

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# ── helpers ───────────────────────────────────────────────────────────────────
function Format-Bytes([long]$n) {
    if     ($n -ge 1MB) { "{0:F1} MB" -f ($n / 1MB) }
    elseif ($n -ge 1KB) { "{0:F1} KB" -f ($n / 1KB) }
    else                { "$n B" }
}

function Show-Bar([long]$current, [long]$total) {
    $width  = 30
    $filled = if ($total -gt 0) { [math]::Min([int](($current / $total) * $width), $width) } else { $width / 2 }
    $pct    = if ($total -gt 0) { [int](($current / $total) * 100) } else { 0 }
    $bar    = ([string][char]0x2588) * $filled + ([string][char]0x2591) * ($width - $filled)
    $cur    = Format-Bytes $current
    $tot    = Format-Bytes $total
    $line   = "  `e[36m[$bar]`e[0m  $cur / $tot  ($pct%)"
    [Console]::Write("`r" + $line.PadRight(72))
}

function Show-BarDone([long]$total) {
    $bar  = ([string][char]0x2588) * 30
    $size = Format-Bytes $total
    [Console]::Write("`r  `e[32m[$bar]`e[0m  $size `u{2713}`n")
}

# ── chunked downloader ────────────────────────────────────────────────────────
# Runs N parallel Range requests via .NET ThreadPool (same process = live bar).
function Get-FileChunked([string]$Url, [string]$Dest, [int]$N = 8) {

    # HEAD to get file size
    $hreq = [System.Net.HttpWebRequest]::Create($Url)
    $hreq.Method = "HEAD"; $hreq.UserAgent = "radii5-installer"
    try { $hr = $hreq.GetResponse(); $total = $hr.ContentLength; $hr.Close() }
    catch { $total = 0 }

    $script:dlBytes = [long]0

    if ($total -le 0 -or $N -le 1) {
        # Streaming fallback
        $r  = [System.Net.HttpWebRequest]::Create($Url)
        $r.UserAgent = "radii5-installer"
        $rs = $r.GetResponse().GetResponseStream()
        $fs = [System.IO.File]::OpenWrite($Dest)
        $b  = New-Object byte[] 65536
        while (($n = $rs.Read($b, 0, $b.Length)) -gt 0) {
            $fs.Write($b, 0, $n)
            $script:dlBytes += $n
            Show-Bar $script:dlBytes $total
        }
        $fs.Close(); $rs.Close()
        Show-BarDone $script:dlBytes
        return
    }

    $chunkSize = [math]::Floor($total / $N)
    $tmpFiles  = [string[]]::new($N)
    $events    = [System.Threading.ManualResetEventSlim[]]::new($N)
    $bag       = [System.Collections.Concurrent.ConcurrentBag[string]]::new()

    for ($i = 0; $i -lt $N; $i++) {
        $tmpFiles[$i] = [System.IO.Path]::GetTempFileName()
        $events[$i]   = [System.Threading.ManualResetEventSlim]::new($false)

        # Capture all loop vars
        $ci = $i
        $cs = [long]($i * $chunkSize)
        $ce = if ($i -eq $N - 1) { $total - 1 } else { $cs + $chunkSize - 1 }
        $ct = $tmpFiles[$i]
        $ce2 = $events[$i]

        [System.Threading.ThreadPool]::QueueUserWorkItem([System.Threading.WaitCallback]{
            param($s)
            try {
                $rq = [System.Net.HttpWebRequest]::Create($s.Url)
                $rq.AddRange("bytes", $s.Start, $s.End)
                $rq.UserAgent = "radii5-installer"
                $rs2 = $rq.GetResponse().GetResponseStream()
                $f2  = [System.IO.File]::OpenWrite($s.Tmp)
                $b2  = New-Object byte[] 65536
                while (($n2 = $rs2.Read($b2, 0, $b2.Length)) -gt 0) {
                    $f2.Write($b2, 0, $n2)
                    [System.Threading.Interlocked]::Add([ref]$script:dlBytes, [long]$n2) | Out-Null
                }
                $f2.Close(); $rs2.Close()
            } catch { $s.Bag.Add($_.Exception.Message) }
            finally { $s.Evt.Set() }
        }, [PSCustomObject]@{ Url=$Url; Start=$cs; End=$ce; Tmp=$ct; Evt=$ce2; Bag=$bag }) | Out-Null
    }

    # Live bar loop — polls until all events are set
    do {
        Show-Bar $script:dlBytes $total
        Start-Sleep -Milliseconds 80
    } while (($events | Where-Object { -not $_.IsSet }).Count -gt 0)

    Show-BarDone $total

    if ($bag.Count -gt 0) {
        Write-Host "  `e[31mX`e[0m chunk error: $($bag | Select-Object -First 1)"
        exit 1
    }

    # Assemble chunks in order
    $fs = [System.IO.File]::OpenWrite($Dest)
    foreach ($tmp in $tmpFiles) {
        $bytes = [System.IO.File]::ReadAllBytes($tmp)
        $fs.Write($bytes, 0, $bytes.Length)
        Remove-Item $tmp -Force
    }
    $fs.Close()
}

function Get-GHRelease([string]$Repo) {
    Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "radii5-installer"; "Accept" = "application/vnd.github+json" }
}

# ── 1. radii5 ─────────────────────────────────────────────────────────────────
Write-Host "  `e[36m→`e[0m  radii5"
try   { $rel = Get-GHRelease $repo }
catch { Write-Host "  `e[31m✗`e[0m Could not fetch radii5 release. Is the repo published?" -ForegroundColor Red; exit 1 }

$assetName = "radii5-$suffix.exe"
$asset = $rel.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) { Write-Host "  `e[31m✗`e[0m No asset '$assetName' found" -ForegroundColor Red; exit 1 }

$r5Dest = Join-Path $installDir "radii5.exe"
Write-Host "  `e[2mversion`e[0m  $($rel.tag_name)"
Write-Host "  `e[2mdest   `e[0m  $r5Dest"
Write-Host ""
Get-FileChunked -Url $asset.browser_download_url -Dest $r5Dest -N $threads
Write-Host "  `e[32m✓`e[0m radii5 $($rel.tag_name)"
Write-Host ""

# ── 2. yt-dlp ─────────────────────────────────────────────────────────────────
if (Get-Command "yt-dlp.exe" -ErrorAction SilentlyContinue) {
    Write-Host "  `e[2m✓ yt-dlp already installed`e[0m"
    Write-Host ""
} else {
    Write-Host "  `e[36m→`e[0m  yt-dlp"
    $ytRel   = Get-GHRelease "yt-dlp/yt-dlp"
    $ytAsset = $ytRel.assets | Where-Object { $_.name -eq "yt-dlp.exe" } | Select-Object -First 1
    if (-not $ytAsset) { Write-Host "  `e[31m✗`e[0m yt-dlp.exe not found" -ForegroundColor Red; exit 1 }

    $ytDest = Join-Path $installDir "yt-dlp.exe"
    Write-Host "  `e[2mversion`e[0m  $($ytRel.tag_name)"
    Write-Host "  `e[2mdest   `e[0m  $ytDest"
    Write-Host ""
    Get-FileChunked -Url $ytAsset.browser_download_url -Dest $ytDest -N $threads
    Write-Host "  `e[32m✓`e[0m yt-dlp $($ytRel.tag_name)"
    Write-Host ""
}

# ── 3. ffmpeg ─────────────────────────────────────────────────────────────────
if (Get-Command "ffmpeg.exe" -ErrorAction SilentlyContinue) {
    Write-Host "  `e[2m✓ ffmpeg already installed`e[0m"
    Write-Host ""
} else {
    Write-Host "  `e[36m→`e[0m  ffmpeg"
    try {
        $ffRel = Get-GHRelease "BtbN/FFmpeg-Builds"
    } catch {
        Write-Host "  `e[33m⚠`e[0m Could not fetch ffmpeg release — skipping"
        Write-Host "  Install manually: https://ffmpeg.org/download.html"
        Write-Host ""
        $ffRel = $null
    }

    if ($ffRel) {
        # prefer essentials (smaller), fall back to any win64 gpl zip
        $ffAsset = $ffRel.assets |
            Where-Object { $_.name -eq "ffmpeg-master-latest-win64-gpl.zip" } |
            Select-Object -First 1
        if (-not $ffAsset) {
            $ffAsset = $ffRel.assets |
                Where-Object { $_.name -like "*win64*gpl*.zip" -and $_.name -notlike "*shared*" } |
                Select-Object -First 1
        }

        if (-not $ffAsset) {
            Write-Host "  `e[33m⚠`e[0m No ffmpeg asset found — install manually: https://ffmpeg.org/download.html"
            Write-Host ""
        } else {
            $ffZip = Join-Path $env:TEMP "ffmpeg-radii5.zip"
            $ffTmp = Join-Path $env:TEMP "ffmpeg-radii5-extract"

            Write-Host "  `e[2msize   `e[0m  $([math]::Round($ffAsset.size / 1MB, 1)) MB (zip)"
            Write-Host "  `e[2mdest   `e[0m  $installDir"
            Write-Host ""

            Get-FileChunked -Url $ffAsset.browser_download_url -Dest $ffZip -N $threads

            Write-Host "  `e[2mextracting...`e[0m"
            if (Test-Path $ffTmp) { Remove-Item $ffTmp -Recurse -Force }
            Expand-Archive -Path $ffZip -DestinationPath $ffTmp -Force
            Remove-Item $ffZip -Force

            $ffExe = Get-ChildItem $ffTmp -Recurse -Filter "ffmpeg.exe" | Select-Object -First 1
            if (-not $ffExe) {
                Write-Host "  `e[31m✗`e[0m Could not find ffmpeg.exe inside archive" -ForegroundColor Red
                exit 1
            }

            foreach ($exe in @("ffmpeg.exe", "ffprobe.exe", "ffplay.exe")) {
                $src = Join-Path $ffExe.DirectoryName $exe
                if (Test-Path $src) { Copy-Item $src -Destination $installDir -Force }
            }
            Remove-Item $ffTmp -Recurse -Force

            Write-Host "  `e[32m✓`e[0m ffmpeg installed"
            Write-Host ""
        }
    }
}

# ── 4. PATH ───────────────────────────────────────────────────────────────────
$curPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($curPath -notlike "*$installDir*") {
    [System.Environment]::SetEnvironmentVariable("PATH", "$curPath;$installDir", "User")
    Write-Host "  `e[32m✓`e[0m Added $installDir to PATH"
    Write-Host "  `e[2m(restart your terminal for it to take effect)`e[0m"
} else {
    Write-Host "  `e[2m✓ $installDir already in PATH`e[0m"
}

Write-Host ""
Write-Host "  `e[1m`e[32mAll done!`e[0m  Try: radii5 --version"
Write-Host ""
