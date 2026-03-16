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

# ── compile C# downloader ─────────────────────────────────────────────────────
Add-Type -AssemblyName System.Net.Http
if (-not ([System.Management.Automation.PSTypeName]'ChunkDownloader').Type) {
Add-Type -Language CSharp -ReferencedAssemblies @(
    'System.Net.Http',
    'System.Threading.Tasks'
) @"
using System;
using System.IO;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Threading;
using System.Threading.Tasks;
using System.Collections.Concurrent;

public static class ChunkDownloader
{
    static long _downloaded;
    static long _total;
    const  int  BarWidth = 30;

    static string FmtBytes(long n) {
        if (n >= 1 << 20) return string.Format("{0:F1} MB", (double)n / (1 << 20));
        if (n >= 1 << 10) return string.Format("{0:F1} KB", (double)n / (1 << 10));
        return n + " B";
    }

    static void DrawBar(long cur, long tot) {
        int filled = (tot > 0) ? (int)Math.Min((double)cur / tot * BarWidth, BarWidth) : BarWidth / 2;
        int pct    = (tot > 0) ? (int)((double)cur / tot * 100) : 0;
        string bar = new string('\u2588', filled) + new string('\u2591', BarWidth - filled);
        string line = string.Format("  \u001b[36m[{0}]\u001b[0m  {1} / {2}  ({3}%)",
            bar, FmtBytes(cur), FmtBytes(tot), pct);
        Console.Write("\r" + line + "\u001b[K");
    }

    static void DrawBarDone(long tot) {
        string bar  = new string('\u2588', BarWidth);
        string line = string.Format("  \u001b[32m[{0}]\u001b[0m  {1} \u2713", bar, FmtBytes(tot));
        Console.Write("\r" + line + "\u001b[K\n");
    }

    public static void Download(string url, string dest, int numThreads) {
        using (var client = new HttpClient()) {
            client.Timeout = System.TimeSpan.FromMinutes(30);
            client.DefaultRequestHeaders.UserAgent.ParseAdd("radii5-installer");

            long total = 0;
            try {
                var headReq = new HttpRequestMessage(HttpMethod.Head, url);
                var headRes = client.SendAsync(headReq).GetAwaiter().GetResult();
                total = headRes.Content.Headers.ContentLength ?? 0;
            } catch {}

            _downloaded = 0;
            _total      = total;

            if (total <= 0 || numThreads <= 1) {
                using (var rs = client.GetStreamAsync(url).GetAwaiter().GetResult())
                using (var fs = File.OpenWrite(dest)) {
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
            var  tasks     = new Task[numThreads];
            var  errors    = new ConcurrentBag<string>();

            for (int i = 0; i < numThreads; i++) {
                tmpFiles[i] = Path.GetTempFileName();
                long start  = i * chunkSize;
                long end    = (i == numThreads - 1) ? total - 1 : start + chunkSize - 1;
                string tmp  = tmpFiles[i];

                tasks[i] = Task.Run(async () => {
                    const int maxRetries = 3;
                    for (int attempt = 0; attempt < maxRetries; attempt++) {
                        try {
                            var req = new HttpRequestMessage(HttpMethod.Get, url);
                            req.Headers.Range = new RangeHeaderValue(start, end);
                            var res    = await client.SendAsync(req, HttpCompletionOption.ResponseHeadersRead);
                            long chunkDownloaded = 0;
                            using (var rs = await res.Content.ReadAsStreamAsync())
                            using (var fs = File.OpenWrite(tmp)) {
                                var buf = new byte[65536];
                                int n;
                                while ((n = await rs.ReadAsync(buf, 0, buf.Length)) > 0) {
                                    fs.Write(buf, 0, n);
                                    Interlocked.Add(ref _downloaded, (long)n);
                                    chunkDownloaded += n;
                                }
                            }
                            return;
                        } catch (Exception ex) {
                            if (attempt == maxRetries - 1)
                                errors.Add(string.Format("chunk failed after {0} attempts: {1}", maxRetries, ex.Message));
                            else {
                                var fi = new FileInfo(tmp);
                                if (fi.Exists) {
                                    Interlocked.Add(ref _downloaded, -fi.Length);
                                    fi.Delete();
                                }
                                Thread.Sleep(500 * (attempt + 1));
                            }
                        }
                    }
                });
            }

            while (!Task.WhenAll(tasks).Wait(80)) {
                DrawBar(_downloaded, total);
            }
            DrawBar(total, total);
            DrawBarDone(total);

            if (!errors.IsEmpty) {
                string msg;
                errors.TryTake(out msg);
                throw new Exception("Chunk failed: " + msg);
            }

            using (var fs = File.OpenWrite(dest)) {
                foreach (var tmp in tmpFiles) {
                    byte[] bytes = File.ReadAllBytes(tmp);
                    fs.Write(bytes, 0, bytes.Length);
                    File.Delete(tmp);
                }
            }
        }
    }
}
"@
} # end if ChunkDownloader not loaded

# ── helpers ───────────────────────────────────────────────────────────────────
function Get-GHRelease([string]$Repo) {
    Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ "User-Agent" = "radii5-installer"; "Accept" = "application/vnd.github+json" }
}

function Install-Binary([string]$Url, [string]$Dest) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    [ChunkDownloader]::Download($Url, $Dest, $threads)
    $sw.Stop()
    $secs    = $sw.Elapsed.TotalSeconds
    $size    = (Get-Item $Dest).Length
    $mbps    = [math]::Round(($size / 1MB) / $secs, 1)
    $elapsed = [math]::Round($secs, 1)
    Write-Host "  `e[2m${mbps} MB/s  (${elapsed}s,  $threads threads)`e[0m"
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
    if (-not $ytAsset) { Write-Host "  `e[31m✗`e[0m yt-dlp.exe not found" -ForegroundColor Red; exit 1 }

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
        $ffRel   = Get-GHRelease "BtbN/FFmpeg-Builds"
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
        if (-not $ffExe) { throw "ffmpeg.exe not found in archive" }

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

# ── 4. deno (JS runtime for yt-dlp) ──────────────────────────────────────────
$denoDest = Join-Path $installDir "deno.exe"
if (Test-Path $denoDest) {
    Write-Host "  `e[2m✓ deno already installed`e[0m"
    Write-Host ""
} else {
    Write-Host "  `e[36m→`e[0m  deno"
    try {
        $denoRel   = Get-GHRelease "denoland/deno"
        $denoAsset = $denoRel.assets |
            Where-Object { $_.name -eq "deno-x86_64-pc-windows-msvc.zip" } |
            Select-Object -First 1
        if (-not $denoAsset) { throw "deno asset not found" }

        $denoZip = Join-Path $env:TEMP "deno-radii5.zip"
        $denoTmp = Join-Path $env:TEMP "deno-radii5-extract"

        Write-Host "  `e[2mversion`e[0m  $($denoRel.tag_name)"
        Write-Host "  `e[2mdest   `e[0m  $denoDest"
        Write-Host ""

        Install-Binary -Url $denoAsset.browser_download_url -Dest $denoZip

        if (Test-Path $denoTmp) { Remove-Item $denoTmp -Recurse -Force }
        Expand-Archive -Path $denoZip -DestinationPath $denoTmp -Force
        Remove-Item $denoZip -Force

        $denoExe = Get-ChildItem $denoTmp -Filter "deno.exe" | Select-Object -First 1
        if (-not $denoExe) { throw "deno.exe not found in archive" }
        Copy-Item $denoExe.FullName -Destination $denoDest -Force
        Remove-Item $denoTmp -Recurse -Force

        Write-Host "  `e[32m✓`e[0m deno $($denoRel.tag_name)"
        Write-Host ""
    } catch {
        Write-Host "  `e[33m⚠`e[0m deno install failed: $_" -ForegroundColor Yellow
        Write-Host "  Install manually: https://deno.com"
        Write-Host ""
    }
}

# ── 5. PATH ───────────────────────────────────────────────────────────────────
$curPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($curPath -notlike "*$installDir*") {
    [System.Environment]::SetEnvironmentVariable("PATH", "$curPath;$installDir", "User")
    $env:PATH = "$env:PATH;$installDir"
    Write-Host "  `e[32m✓`e[0m Added $installDir to PATH"
} else {
    Write-Host "  `e[2m✓ $installDir already in PATH`e[0m"
}

Write-Host ""
Write-Host "  `e[1m`e[32mAll done!`e[0m  Try: radii5 --version"
Write-Host ""