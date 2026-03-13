# radii5 installer for Windows
# Usage: irm https://raw.githubusercontent.com/radii5/music/main/scripts/install.ps1 | iex

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

# ── compile pure C# chunked downloader ───────────────────────────────────────
# PowerShell script blocks cannot run on ThreadPool threads (no Runspace).
# We compile a small C# class that does everything in pure .NET.
Add-Type -Language CSharp @"
using System;
using System.IO;
using System.Net;
using System.Threading;
using System.Collections.Generic;

public static class ChunkDownloader
{
    static long _downloaded;
    static long _total;
    static int  _barWidth = 30;

    static string FmtBytes(long n) {
        if (n >= 1 << 20) return string.Format("{0:F1} MB", (double)n / (1 << 20));
        if (n >= 1 << 10) return string.Format("{0:F1} KB", (double)n / (1 << 10));
        return n + " B";
    }

    static void DrawBar(long cur, long tot) {
        int filled = (tot > 0) ? (int)Math.Min((double)cur / tot * _barWidth, _barWidth) : _barWidth / 2;
        int pct    = (tot > 0) ? (int)((double)cur / tot * 100) : 0;
        string bar = new string('\u2588', filled) + new string('\u2591', _barWidth - filled);
        string line = string.Format("  \u001b[36m[{0}]\u001b[0m  {1} / {2}  ({3}%)",
            bar, FmtBytes(cur), FmtBytes(tot), pct);
        Console.Write("\r" + line.PadRight(72));
    }

    static void DrawBarDone(long tot) {
        string bar  = new string('\u2588', _barWidth);
        string line = string.Format("  \u001b[32m[{0}]\u001b[0m  {1} \u2713", bar, FmtBytes(tot));
        Console.Write("\r" + line.PadRight(72) + "\n");
    }

    public static void Download(string url, string dest, int numThreads) {
        // HEAD to get size
        var hreq = (HttpWebRequest)WebRequest.Create(url);
        hreq.Method    = "HEAD";
        hreq.UserAgent = "radii5-installer";
        long total = 0;
        try {
            using (var hr = (HttpWebResponse)hreq.GetResponse())
                total = hr.ContentLength;
        } catch {}

        _downloaded = 0;
        _total      = total;

        if (total <= 0 || numThreads <= 1) {
            // Simple streaming download
            var req  = (HttpWebRequest)WebRequest.Create(url);
            req.UserAgent = "radii5-installer";
            using (var rs  = req.GetResponse().GetResponseStream())
            using (var fs  = File.OpenWrite(dest)) {
                var buf = new byte[65536];
                int n;
                while ((n = rs.Read(buf, 0, buf.Length)) > 0) {
                    fs.Write(buf, 0, n);
                    Interlocked.Add(ref _downloaded, n);
                    DrawBar(_downloaded, total);
                }
            }
            DrawBarDone(_downloaded);
            return;
        }

        long chunkSize = total / numThreads;
        var  tmpFiles  = new string[numThreads];
        var  events    = new ManualResetEventSlim[numThreads];
        var  errors    = new System.Collections.Concurrent.ConcurrentBag<string>();

        for (int i = 0; i < numThreads; i++) {
            tmpFiles[i] = Path.GetTempFileName();
            events[i]   = new ManualResetEventSlim(false);

            long start = i * chunkSize;
            long end   = (i == numThreads - 1) ? total - 1 : start + chunkSize - 1;

            // Capture for closure
            var state = new { Url=url, Start=start, End=end, Tmp=tmpFiles[i], Evt=events[i], Bag=errors };

            ThreadPool.QueueUserWorkItem(_ => {
                try {
                    var rq = (HttpWebRequest)WebRequest.Create(state.Url);
                    rq.AddRange(state.Start, state.End);
                    rq.UserAgent = "radii5-installer";
                    using (var rs = rq.GetResponse().GetResponseStream())
                    using (var fs = File.OpenWrite(state.Tmp)) {
                        var buf = new byte[65536];
                        int n;
                        while ((n = rs.Read(buf, 0, buf.Length)) > 0) {
                            fs.Write(buf, 0, n);
                            Interlocked.Add(ref _downloaded, (long)n);
                        }
                    }
                } catch (Exception ex) {
                    state.Bag.Add(ex.Message);
                } finally {
                    state.Evt.Set();
                }
            });
        }

        // Poll and redraw bar until all chunks done
        bool allDone = false;
        while (!allDone) {
            DrawBar(_downloaded, total);
            Thread.Sleep(80);
            allDone = true;
            foreach (var ev in events)
                if (!ev.IsSet) { allDone = false; break; }
        }
        DrawBarDone(total);

        if (errors.Count > 0) {
            string msg;
            errors.TryTake(out msg);
            throw new Exception("Chunk download failed: " + msg);
        }

        // Assemble
        using (var fs = File.OpenWrite(dest)) {
            foreach (var tmp in tmpFiles) {
                var bytes = File.ReadAllBytes(tmp);
                fs.Write(bytes, 0, bytes.Length);
                File.Delete(tmp);
            }
        }
    }
}
"@

# ── helpers ───────────────────────────────────────────────────────────────────
function Get-GHRelease([string]$Repo) {
    Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "radii5-installer"; "Accept" = "application/vnd.github+json" }
}

function Install-Binary([string]$Url, [string]$Dest) {
    [ChunkDownloader]::Download($Url, $Dest, $threads)
}

# ── 1. radii5 ─────────────────────────────────────────────────────────────────
Write-Host "  `e[36m→`e[0m  radii5"
try   { $rel = Get-GHRelease $repo }
catch { Write-Host "  `e[31m✗`e[0m Could not fetch radii5 release. Is the repo published and tagged?" -ForegroundColor Red; exit 1 }

$assetName = "radii5-$suffix.exe"
$asset = $rel.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) { Write-Host "  `e[31m✗`e[0m No asset '$assetName' found in release $($rel.tag_name)" -ForegroundColor Red; exit 1 }

$r5Dest = Join-Path $installDir "radii5.exe"
Write-Host "  `e[2mversion`e[0m  $($rel.tag_name)"
Write-Host "  `e[2mdest   `e[0m  $r5Dest"
Write-Host ""
Install-Binary -Url $asset.browser_download_url -Dest $r5Dest
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
    if (-not $ytAsset) { Write-Host "  `e[31m✗`e[0m yt-dlp.exe not found in release" -ForegroundColor Red; exit 1 }

    $ytDest = Join-Path $installDir "yt-dlp.exe"
    Write-Host "  `e[2mversion`e[0m  $($ytRel.tag_name)"
    Write-Host "  `e[2mdest   `e[0m  $ytDest"
    Write-Host ""
    Install-Binary -Url $ytAsset.browser_download_url -Dest $ytDest
    Write-Host "  `e[32m✓`e[0m yt-dlp $($ytRel.tag_name)"
    Write-Host ""
}

# ── 3. ffmpeg ─────────────────────────────────────────────────────────────────
$ffDest = Join-Path $installDir "ffmpeg.exe"
if (Test-Path $ffDest) {
    Write-Host "  `e[2m✓ ffmpeg already installed`e[0m"
    Write-Host ""
} else {
    Write-Host "  `e[36m→`e[0m  ffmpeg"
    try {
        $ffRel = Get-GHRelease "BtbN/FFmpeg-Builds"

        $ffAsset = $ffRel.assets |
            Where-Object { $_.name -eq "ffmpeg-master-latest-win64-gpl.zip" } |
            Select-Object -First 1
        if (-not $ffAsset) {
            $ffAsset = $ffRel.assets |
                Where-Object { $_.name -like "*win64*gpl*.zip" -and $_.name -notlike "*shared*" } |
                Select-Object -First 1
        }

        if (-not $ffAsset) { throw "No matching asset found" }

        $ffZip = Join-Path $env:TEMP "ffmpeg-radii5.zip"
        $ffTmp = Join-Path $env:TEMP "ffmpeg-radii5-extract"

        Write-Host "  `e[2msize   `e[0m  $([math]::Round($ffAsset.size / 1MB, 1)) MB (zip)"
        Write-Host "  `e[2mdest   `e[0m  $installDir"
        Write-Host ""

        Install-Binary -Url $ffAsset.browser_download_url -Dest $ffZip

        Write-Host "  `e[2mextracting...`e[0m"
        if (Test-Path $ffTmp) { Remove-Item $ffTmp -Recurse -Force }
        Expand-Archive -Path $ffZip -DestinationPath $ffTmp -Force
        Remove-Item $ffZip -Force

        $ffExe = Get-ChildItem $ffTmp -Recurse -Filter "ffmpeg.exe" | Select-Object -First 1
        if (-not $ffExe) { throw "ffmpeg.exe not found inside archive" }

        foreach ($exe in @("ffmpeg.exe", "ffprobe.exe", "ffplay.exe")) {
            $src = Join-Path $ffExe.DirectoryName $exe
            if (Test-Path $src) { Copy-Item $src -Destination $installDir -Force }
        }
        Remove-Item $ffTmp -Recurse -Force

        Write-Host "  `e[32m✓`e[0m ffmpeg installed"
        Write-Host ""
    } catch {
        Write-Host "  `e[33m⚠`e[0m ffmpeg install failed: $_" -ForegroundColor Yellow
        Write-Host "  Install manually: https://ffmpeg.org/download.html"
        Write-Host ""
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
