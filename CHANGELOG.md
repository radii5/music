# Changelog

All notable changes to radii5 are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
radii5 uses [Semantic Versioning](https://semver.org/).

---

## [0.3.2] — 2026-03-17

### Added
- Playlist support with concurrent workers
- `--workers` flag for playlist concurrency control
- Visual sliding animation for track transitions

---

## [0.2.0] — 2026-03-17

### Changed
- Renamed Go module to match repo (`github.com/radii5/music`)
- Updated README badges for latest release + platforms
- Improved Windows PowerShell 5 installer (DefaultConnectionLimit, QuickEdit disable, PS5 compatibility)
- Fixed import paths and revert cleanup

### Added
- PS5-compatible installer routing

## [0.1.0] — 2024-01-01

### Added
- Initial release
- Parallel chunked downloading with configurable thread count (default: 8)
- Support for MP3, FLAC, WAV, M4A, Opus, and any format yt-dlp supports
- Automatic ID3v2 tag writing for MP3 files (title, artist, album, cover art)
- Live progress bar with bytes downloaded / total
- Default output to `~/Music/radii5 downloads`
- `--format`, `--output`, `--threads` flags
- Supports 1000+ sites via yt-dlp (YouTube, SoundCloud, Bandcamp, and more)
- Pre-built binaries for Linux (x64/ARM64), macOS (x64/Apple Silicon), Windows (x64)
