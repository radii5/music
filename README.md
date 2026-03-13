# radii5

**Fast music downloader — built for speed, not bloat.**

Downloads audio from YouTube, SoundCloud, Bandcamp, and 1000+ other sites. Uses parallel chunk downloading for maximum speed and automatically writes proper ID3 tags to your files.

---

## Install

### Pre-built binaries (recommended)

Download the latest release for your platform from the [Releases](https://github.com/radii5/radii5/releases) page.

| Platform       | File                          |
|----------------|-------------------------------|
| Linux x64      | `radii5-linux-amd64`          |
| Linux ARM64    | `radii5-linux-arm64`          |
| macOS x64      | `radii5-macos-amd64`          |
| macOS Apple Silicon | `radii5-macos-arm64`     |
| Windows x64    | `radii5-windows-amd64.exe`    |

**Linux / macOS** — make it executable and move it to your PATH:

```sh
chmod +x radii5-linux-amd64
sudo mv radii5-linux-amd64 /usr/local/bin/radii5
```

**Windows** — rename to `radii5.exe` and add to a folder in your `PATH`.

### Build from source

```sh
git clone https://github.com/radii5/radii5.git
cd radii5
go build -o radii5 .
```

### Prerequisites

radii5 requires **[yt-dlp](https://github.com/yt-dlp/yt-dlp#installation)** to be installed and on your PATH. It handles site extraction and format conversion.

```sh
# macOS
brew install yt-dlp

# Linux
sudo curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp

# Windows (via winget)
winget install yt-dlp
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

### Flags

| Flag                  | Short | Default                       | Description                        |
|-----------------------|-------|-------------------------------|------------------------------------|
| `--format <fmt>`      | `-f`  | `mp3`                         | Output format (`mp3`, `flac`, `wav`, `m4a`, `opus`) |
| `--output <dir>`      | `-o`  | `~/Music/radii5 downloads`    | Output directory                   |
| `--threads <n>`       | `-t`  | `8`                           | Parallel download chunks           |
| `--version`           | `-v`  |                               | Print version and exit             |
| `--help`              | `-h`  |                               | Show usage                         |

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
