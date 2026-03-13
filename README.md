# radii5

**Fast music downloader — built for speed, not bloat.**

Downloads audio from YouTube, YouTube Music, SoundCloud, Bandcamp, and 1000+ other sites. Parallel chunk downloading, automatic ID3 tagging, and a clean one-line install.

---

## Install

### Windows — PowerShell

```powershell
irm https://radii5.github.io/music/install.ps1 | iex
```

Installs **radii5**, **yt-dlp**, **ffmpeg**, and **deno** automatically. Adds everything to your PATH — no manual setup needed.

### Linux / macOS

```sh
curl -fsSL https://raw.githubusercontent.com/radii5/music/main/scripts/install.sh | sh
```

> **Note:** yt-dlp and ffmpeg must be on your PATH. Install them first:
> ```sh
> # macOS
> brew install yt-dlp ffmpeg
>
> # Linux
> pip install yt-dlp && sudo apt install ffmpeg
> ```

### Manual

Download the binary for your platform from [Releases](https://github.com/radii5/music/releases):

| Platform            | File                          |
|---------------------|-------------------------------|
| Linux x64           | `radii5-linux-amd64`          |
| Linux ARM64         | `radii5-linux-arm64`          |
| macOS x64           | `radii5-macos-amd64`          |
| macOS Apple Silicon | `radii5-macos-arm64`          |
| Windows x64         | `radii5-windows-amd64.exe`    |

```sh
# Linux / macOS
chmod +x radii5-linux-amd64
sudo mv radii5-linux-amd64 /usr/local/bin/radii5
```

### Build from source

```sh
# requires Go 1.22+  →  https://go.dev/dl/
git clone https://github.com/radii5/music.git
cd music
go build -o radii5 .
sudo mv radii5 /usr/local/bin/   # Linux/macOS
```

---

## Usage

```sh
radii5 <url>
```

```sh
# Download a YouTube track as MP3 (default)
radii5 https://www.youtube.com/watch?v=...

# Download from YouTube Music
radii5 https://music.youtube.com/watch?v=...

# Download as a different format
radii5 <url> --format flac

# Save to a custom directory
radii5 <url> --output ~/Music/downloads

# Use more parallel download threads
radii5 <url> --threads 16
```

Files are saved to `~/Music/radii5 downloads` by default.

> **Windows / PowerShell tip:** If your URL contains `&` (e.g. `&list=...`), either wrap it in quotes or remove everything after the video ID:
> ```powershell
> # ✗ breaks — PowerShell treats & as a command separator
> radii5 https://music.youtube.com/watch?v=abc123&list=xyz
>
> # ✓ quoted
> radii5 "https://music.youtube.com/watch?v=abc123&list=xyz"
>
> # ✓ trimmed (simpler)
> radii5 https://music.youtube.com/watch?v=abc123
> ```

### Flags

| Flag             | Short | Default                    | Description              |
|------------------|-------|----------------------------|--------------------------|
| `--format <fmt>` | `-f`  | `mp3`                      | Output format (see below)|
| `--output <dir>` | `-o`  | `~/Music/radii5 downloads` | Output directory         |
| `--threads <n>`  | `-t`  | `8`                        | Parallel download chunks |
| `--version`      | `-v`  |                            | Print version and exit   |
| `--help`         | `-h`  |                            | Show usage               |

---

## Features

- **Parallel downloading** — splits files into 8 concurrent chunks for faster downloads when the source supports range requests
- **ID3 tag writing** — automatically embeds title, artist, album, and cover art into MP3s
- **1000+ supported sites** — anything yt-dlp supports, radii5 supports
- **Zero config** — sensible defaults out of the box
- **Single binary** — one executable, no runtime to manage

---

## Supported formats

`mp3` · `flac` · `m4a` · `opus` · `aac`

Any format supported by yt-dlp's `--audio-format` flag will work.

---

## Requirements

- **yt-dlp** — for resolving URLs and extracting audio streams
- **ffmpeg** — for audio conversion (e.g. webm → mp3)

The Windows installer handles both automatically. On Linux/macOS, install them manually (see above).

---

## License

MIT — see [LICENSE](LICENSE)
