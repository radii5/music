# radii5

**Fast music downloader — built for speed, not bloat.**

Downloads audio from YouTube, SoundCloud, Bandcamp, and 1000+ other sites. Uses parallel chunk downloading for maximum speed and automatically writes proper ID3 tags to your files.

---

## Install

### Linux / macOS — one line

```sh
curl -fsSL https://raw.githubusercontent.com/radii5/radii5/main/scripts/install.sh | sh
```

### Windows — PowerShell

```powershell
irm https://raw.githubusercontent.com/radii5/radii5/main/scripts/install.ps1 | iex
```

### Cross-platform — Go runner

If you already have Go installed, `install.go` uses the same chunked parallel downloader as radii5 itself — no shell or PowerShell needed:

```sh
go run https://raw.githubusercontent.com/radii5/radii5/main/install.go
# or after cloning:
go run install.go
```

All three methods automatically download **both** yt-dlp and radii5 at their latest versions. yt-dlp is skipped if it's already on your PATH.

### Manual

Download the binary for your platform from [Releases](https://github.com/radii5/radii5/releases):

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
git clone https://github.com/radii5/radii5.git
cd radii5
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

# Download as FLAC
radii5 https://soundcloud.com/artist/track --format flac

# Save to a custom directory
radii5 <url> --output ~/Music/downloads

# Use more parallel threads for faster downloads
radii5 <url> --threads 16
```

> **Windows / PowerShell tip:** If your URL contains `&` (e.g. `&list=...`), wrap it in quotes — otherwise PowerShell splits it:
> ```powershell
> # ✗ breaks — PowerShell treats & as a command separator
> radii5 https://music.youtube.com/watch?v=abc123&list=xyz
>
> # ✓ works
> radii5 "https://music.youtube.com/watch?v=abc123&list=xyz"
> ```

### Flags

| Flag             | Short | Default                    | Description                                         |
|------------------|-------|----------------------------|-----------------------------------------------------|
| `--format <fmt>` | `-f`  | `mp3`                      | Output format (`mp3`, `flac`, `wav`, `m4a`, `opus`) |
| `--output <dir>` | `-o`  | `~/Music/radii5 downloads` | Output directory                                    |
| `--threads <n>`  | `-t`  | `8`                        | Parallel download chunks                            |
| `--version`      | `-v`  |                            | Print version and exit                              |
| `--help`         | `-h`  |                            | Show usage                                          |

---

## Features

- **Parallel downloading** — splits files into chunks and fetches them concurrently
- **ID3 tag writing** — automatically embeds title, artist, album, and cover art into MP3s
- **1000+ supported sites** — anything yt-dlp supports, radii5 supports
- **Zero config** — sensible defaults out of the box
- **Single binary** — no runtime dependencies beyond yt-dlp

---

## Supported formats

`mp3` · `flac` · `wav` · `m4a` · `opus` · `aac` · `vorbis`

Any format supported by yt-dlp's `--audio-format` flag will work.

---

## License

MIT — see [LICENSE](LICENSE)
