<div align="center">

# radii5

<p>Fast music downloader built on <a href="https://github.com/yt-dlp/yt-dlp">yt-dlp</a></p>

[![Release](https://img.shields.io/github/v/release/radii5/music?style=flat&color=326ce5&label=latest)](https://github.com/radii5/music/releases)
[![License](https://img.shields.io/github/license/radii5/music?style=flat&color=40c463)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)

![demo](assets/demo.gif)

</div>

---

## Install

**Windows** — run in **PowerShell**
```powershell
irm https://radii5.github.io/music/install.ps1 | iex
```
Installs radii5, yt-dlp, ffmpeg, and deno automatically. No manual setup needed.

**Linux / macOS**
```sh
curl -fsSL https://raw.githubusercontent.com/radii5/music/main/scripts/install.sh | sh
```
Installs radii5 and yt-dlp. ffmpeg is required separately:
```sh
brew install ffmpeg          # macOS
sudo apt install ffmpeg      # Debian/Ubuntu
```

**Alternative: Prebuilt releases**
```sh
# Download from GitHub Releases and verify SHA256
wget https://github.com/radii5/music/releases/latest/download/radii5-windows-amd64.exe
# Verify hash from release notes
sha256sum radii5-windows-amd64.exe
```

**Alternative: Go install**
```sh
go install github.com/radii5/music/cmd/music@latest
```

<details>
<summary>Manual install / Build from source</summary>

**Prebuilt binaries** — [Releases](https://github.com/radii5/music/releases)

| Platform | File |
|---|---|
| Linux x64 | `radii5-linux-amd64` |
| Linux ARM64 | `radii5-linux-arm64` |
| macOS x64 | `radii5-macos-amd64` |
| macOS Apple Silicon | `radii5-macos-arm64` |
| Windows x64 | `radii5-windows-amd64.exe` |

```sh
chmod +x radii5-linux-amd64
sudo mv radii5-linux-amd64 /usr/local/bin/radii5
```

**Build from source** — requires [Go 1.22+](https://go.dev/dl/)
```sh
git clone https://github.com/radii5/music.git
cd music
go build -o radii5 ./cmd/music
sudo mv radii5 /usr/local/bin/   # Linux/macOS
```
</details>

---

## Usage

```sh
radii5 <url>                          # download as MP3 (default)
radii5 <url> --format flac            # choose format
radii5 <url> --output ~/Music         # custom output directory
radii5 <url> --threads 16             # more parallel chunks
```

Files are saved to `~/Music/radii5 downloads` by default.

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--format` | `-f` | `mp3` | Output format (`mp3` `flac` `m4a` `opus` `aac`) |
| `--output` | `-o` | `~/Music/radii5 downloads` | Output directory |
| `--threads` | `-t` | `8` | Parallel download chunks |
| `--version` | `-v` | | Print version |
| `--help` | `-h` | | Show usage |

> [!TIP]
> **Windows / PowerShell:** URLs with `&` must be quoted or trimmed — PowerShell treats `&` as a command separator.
> ```powershell
> radii5 "https://music.youtube.com/watch?v=abc123&list=xyz"  # quoted
> radii5 https://music.youtube.com/watch?v=abc123              # trimmed
> ```

---

## Features

- **Parallel chunk downloading** — splits files into concurrent range requests for faster downloads
- **Automatic ID3 tags** — embeds title, artist, album, and cover art into MP3s
- **1000+ supported sites** — YouTube, YouTube Music, SoundCloud, Bandcamp, and anything else yt-dlp supports
- **Zero config** — sensible defaults, works out of the box
- **Single binary** — one executable, no runtime to manage

---

## Requirements

| Dependency | Purpose | Windows installer | Linux / macOS installer |
|---|---|---|---|
| yt-dlp | URL resolving, stream extraction | auto | auto |
| ffmpeg | Audio conversion | auto | manual |
| deno | YouTube JS runtime | auto | not required |

---

## License

[MIT](LICENSE)
