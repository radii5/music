package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/radii5/music/internal/metadata"
	"github.com/radii5/music/internal/progress"
)

type VideoInfo struct {
	Title          string  `json:"title"`
	Artist         string  `json:"artist"`
	Uploader       string  `json:"uploader"`
	Album          string  `json:"album"`
	Duration       float64 `json:"duration"`
	URL            string  `json:"url"`
	Thumbnail      string  `json:"thumbnail"`
	Ext            string  `json:"ext"`
	AudioCodec     string  `json:"acodec"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
}

func (v *VideoInfo) DisplayArtist() string {
	if v.Artist != "" {
		return v.Artist
	}
	return v.Uploader
}

var httpClient = NewOptimizedHTTPClient()

const maxRetries = 5

// findBin finds the path to a binary, can be overridden for testing
var findBin = func(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	if dir := selfDir(); dir != "" {
		if candidate := filepath.Join(dir, name); fileExists(candidate) {
			return candidate
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if candidate := filepath.Join(home, ".radii5", "bin", name); fileExists(candidate) {
			return candidate
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return name
}

func selfDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Download(url, format, outputDir string, threads int) (err error) {
	bold := color.New(color.FgWhite, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	cyan.Print("  → ")
	bold.Println("Resolving track...")

	info, err := resolve(url)
	if err != nil {
		return fmt.Errorf("could not resolve URL: %w", err)
	}

	fmt.Println()
	color.New(color.FgHiWhite, color.Bold).Printf("  %s\n", info.Title)
	if artist := info.DisplayArtist(); artist != "" {
		color.New(color.FgHiBlack).Printf("  %s\n", artist)
	}
	if info.Duration > 0 {
		color.New(color.FgHiBlack).Printf("  %s\n", formatDuration(info.Duration))
	}
	fmt.Println()

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output dir: %w", err)
	}

	safeTitle := sanitizeFilename(info.Title)
	tmpFile := filepath.Join(outputDir, safeTitle+".tmp")
	outFile := filepath.Join(outputDir, safeTitle+"."+format)

	// Ensure temp file cleanup on panic or error
	defer func() {
		if r := recover(); r != nil {
			os.Remove(tmpFile)
			panic(r) // Re-panic after cleanup
		}
		if err != nil {
			os.Remove(tmpFile)
		}
	}()

	// If direct URL available, download ourselves (fast parallel chunks).
	// Otherwise fall back to yt-dlp for extraction + conversion.
	if info.URL != "" {
		size := info.Filesize
		if size == 0 {
			size = info.FilesizeApprox
		}

		_, supportsRange, _ := probeURL(info.URL)
		start := time.Now()

		adaptiveThreads := DetermineThreads(size, threads)
		if supportsRange && size > 0 && adaptiveThreads > 1 {
			cyan.Printf("  → Downloading in %d parallel chunks...\n\n", adaptiveThreads)
			if err := parallelDownload(info.URL, tmpFile, size, adaptiveThreads); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
		} else {
			cyan.Println("  → Downloading...")
			fmt.Println()
			if err := streamDownload(info.URL, tmpFile, size); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
		}

		elapsed := time.Since(start)

		// Print speed summary
		var size64 int64
		if fi, err := os.Stat(tmpFile); err == nil {
			size64 = fi.Size()
		}
		if size64 > 0 && elapsed.Seconds() > 0 {
			mbps := float64(size64) / (1 << 20) / elapsed.Seconds()
			if supportsRange && size > 0 && adaptiveThreads > 1 {
				color.New(color.FgHiBlack).Printf("  %.1f MB/s  (%.1fs,  %d threads)\n", mbps, elapsed.Seconds(), adaptiveThreads)
			} else {
				color.New(color.FgHiBlack).Printf("  %.1f MB/s  (%.1fs)\n", mbps, elapsed.Seconds())
			}
		}

		if format != info.Ext {
			cyan.Print("\n  → Converting to " + strings.ToUpper(format) + "...")
			if err := convertAudio(tmpFile, outFile, format); err != nil {
				os.Remove(tmpFile)
				return fmt.Errorf("conversion failed: %w", err)
			}
			os.Remove(tmpFile)
			cyan.Println(" done")
		} else {
			os.Rename(tmpFile, outFile)
		}

		if format == "mp3" {
			cyan.Print("  → Writing metadata...")
			_ = metadata.WriteMP3Tags(outFile, info.Title, info.DisplayArtist(), info.Album, info.Thumbnail)
			cyan.Println(" done")
		}

		fmt.Println()
		color.New(color.FgGreen, color.Bold).Print("  ✓ ")
		fmt.Printf("Saved to %s", color.New(color.FgCyan).Sprint(outFile))
		color.New(color.FgHiBlack).Printf("  (%s)\n\n", elapsed.Round(time.Millisecond))

	} else {
		// No direct URL — let yt-dlp handle the full download + conversion
		if err := ytDlpFallback(url, format, outFile, threads); err != nil {
			return err
		}
	}

	return nil
}

func sanitizeURL(inputURL string) string {
	inputURL = strings.TrimSpace(inputURL)
	if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
		return ""
	}

	parsed, err := url.Parse(inputURL)
	if err != nil {
		return ""
	}

	if parsed.Host == "" {
		return ""
	}

	parsed.Fragment = ""
	return parsed.String()
}

func resolve(url string) (*VideoInfo, error) {
	url = sanitizeURL(url)
	if url == "" {
		return nil, fmt.Errorf("invalid URL")
	}

	url = cleanURL(url)
	ytdlp := findBin("yt-dlp")

	// Create context with timeout for yt-dlp resolve operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ytdlp,
		"--dump-json",
		"--format", "bestaudio",
		"--no-playlist",
		"--socket-timeout", "25", // yt-dlp socket timeout (25s < 30s context)
		url,
	)

	// Capture both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	err = cmd.Wait()

	// Check for context errors first
	if ctx.Err() == context.Canceled {
		return nil, fmt.Errorf("yt-dlp resolve canceled: %w", ctx.Err())
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("yt-dlp resolve timeout after 30 seconds")
	}

	if err != nil {
		// Check if yt-dlp binary exists
		if _, e := exec.LookPath(ytdlp); e != nil {
			return nil, fmt.Errorf("yt-dlp not found — run the installer")
		}

		// Return structured error with stderr
		stderr := stderrBuf.String()
		if stderr != "" {
			return nil, fmt.Errorf("yt-dlp resolve failed: %w\nstderr: %s", err, stderr)
		}
		return nil, fmt.Errorf("yt-dlp resolve failed: %w", err)
	}

	var info VideoInfo
	if err := json.Unmarshal(stdoutBuf.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse track info: %w", err)
	}
	return &info, nil
}

func ytDlpFallback(url, format, outFile string, threads int) error {
	cyan := color.New(color.FgCyan)
	adaptiveThreads := DetermineThreads(0, threads) // 0 size = unknown, use adaptive logic
	cyan.Printf("  → Downloading via yt-dlp (%d fragments)...\n\n", adaptiveThreads)

	// Create context with timeout for yt-dlp operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	ytdlp := findBin("yt-dlp")
	args := []string{
		"--no-playlist",
		"-x",
		"--audio-format", format,
		"--audio-quality", "0",
		"--concurrent-fragments", fmt.Sprintf("%d", adaptiveThreads),
		"--no-colors",
		"--progress", "--newline",
		"-o", outFile,
		url,
	}

	cmd := exec.CommandContext(ctx, ytdlp, args...)

	// Capture both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	var bar *progress.Bar
	var mu sync.Mutex
	var once sync.Once

	scan := func(r io.Reader, isErr bool) {
		scanner := newLineScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			pct, total, current, ok := parseYtDlpProgress(line)
			if ok {
				mu.Lock()
				if bar == nil {
					bar = progress.NewBar(total)
				}
				bar.Set(current)
				if pct >= 100 {
					once.Do(func() { bar.Finish() })
				}
				mu.Unlock()
			} else if isErr && (strings.Contains(line, "ERROR")) {
				fmt.Fprintf(os.Stderr, "  %s\n", color.RedString(line))
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); scan(bytes.NewReader(stdoutBuf.Bytes()), false) }()
	go func() { defer wg.Done(); scan(bytes.NewReader(stderrBuf.Bytes()), true) }()

	// Start scanning in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				// Continue scanning new output
			}
		}
	}()

	err := cmd.Wait()

	mu.Lock()
	if bar != nil {
		once.Do(func() { bar.Finish() })
	}
	mu.Unlock()

	if err != nil {
		// Check if context was canceled
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("yt-dlp canceled: %w", ctx.Err())
		}
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("yt-dlp timeout after 30 minutes: %w", err)
		}

		// Check if yt-dlp binary exists
		if _, e := exec.LookPath(ytdlp); e != nil {
			return fmt.Errorf("yt-dlp not found — run the installer: %w", e)
		}

		// Return structured error with stderr
		stderr := stderrBuf.String()
		if stderr != "" {
			return fmt.Errorf("yt-dlp failed: %w\nstderr: %s", err, stderr)
		}
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	color.New(color.FgGreen, color.Bold).Printf("  ✓ Saved to %s\n\n", outFile)
	return nil
}

func probeURL(url string) (int64, bool, error) {
	resp, err := httpClient.Head(url)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	size := resp.ContentLength
	supportsRange := resp.Header.Get("Accept-Ranges") == "bytes"
	return size, supportsRange, nil
}

func parallelDownload(url, dest string, size int64, threads int) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if err := f.Truncate(size); err != nil {
		f.Close()
		return err
	}
	f.Close()

	chunkSize := size / int64(threads)
	var wg sync.WaitGroup
	errCh := make(chan error, threads)
	bar := progress.NewBar(size)

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := int64(i) * chunkSize
			end := start + chunkSize - 1
			if i == threads-1 {
				end = size - 1
			}
			if err := fetchWithRetry(url, dest, start, end, bar); err != nil {
				errCh <- fmt.Errorf("chunk %d: %w", i, err)
			}
		}(i)
	}

	wg.Wait()
	bar.Finish()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func fetchWithRetry(url, dest string, start, end int64, bar *progress.Bar) error {
	current := start
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}
		written, err := fetchRangeToDisk(url, dest, current, end, bar)
		current += written
		if err == nil {
			return nil
		}
		if current > end {
			return nil
		}
	}
	return fmt.Errorf("failed after %d retries", maxRetries)
}

func fetchRangeToDisk(url, dest string, start, end int64, bar *progress.Bar) (int64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; radii5/0.1)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(dest, os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}

	n, err := io.Copy(f, io.TeeReader(resp.Body, bar))
	return n, err
}

func streamDownload(url, dest string, size int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; radii5/0.1)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	bar := progress.NewBar(size)
	_, err = io.Copy(f, io.TeeReader(resp.Body, bar))
	bar.Finish()
	return err
}

func convertAudio(input, output, format string) error {
	ffmpeg := findBin("ffmpeg")
	var args []string
	switch format {
	case "mp3":
		args = []string{"-i", input, "-codec:a", "libmp3lame", "-qscale:a", "0", "-y", output}
	case "flac":
		args = []string{"-i", input, "-codec:a", "flac", "-compression_level", "8", "-y", output}
	case "m4a":
		args = []string{"-i", input, "-codec:a", "aac", "-b:a", "256k", "-y", output}
	default:
		args = []string{"-i", input, "-y", output}
	}
	cmd := exec.Command(ffmpeg, args...)
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func cleanURL(raw string) string {
	for _, param := range []string{"?si=", "&si="} {
		if idx := strings.Index(raw, param); idx != -1 {
			raw = raw[:idx]
		}
	}
	return raw
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "",
		"?", "", "\"", "", "<", "", ">", "", "|", "",
	)
	return strings.TrimSpace(replacer.Replace(name))
}

func formatDuration(secs float64) string {
	m := int(secs) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func newLineScanner(r io.Reader) *lineScanner {
	return &lineScanner{r: r, buf: make([]byte, 0, 4096)}
}

type lineScanner struct {
	r    io.Reader
	buf  []byte
	line string
	done bool
}

func (s *lineScanner) Scan() bool {
	if s.done {
		return false
	}
	tmp := make([]byte, 512)
	for {
		if idx := indexByte(s.buf, '\n'); idx >= 0 {
			s.line = strings.TrimRight(string(s.buf[:idx]), "\r")
			s.buf = s.buf[idx+1:]
			return true
		}
		n, err := s.r.Read(tmp)
		if n > 0 {
			s.buf = append(s.buf, tmp[:n]...)
		}
		if err != nil {
			if len(s.buf) > 0 {
				s.line = strings.TrimRight(string(s.buf), "\r\n")
				s.buf = nil
				s.done = true
				return s.line != ""
			}
			return false
		}
	}
}

func (s *lineScanner) Text() string { return s.line }

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func parseYtDlpProgress(line string) (pct float64, total, current int64, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[download]") {
		return
	}
	line = strings.TrimPrefix(line, "[download]")
	line = strings.TrimSpace(line)

	ofIdx := strings.Index(line, "% of")
	if ofIdx < 0 {
		return
	}

	pctStr := strings.TrimSpace(line[:ofIdx])
	if _, err := fmt.Sscanf(pctStr, "%f", &pct); err != nil {
		return
	}

	rest := strings.TrimSpace(line[ofIdx+4:])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return
	}
	total = parseSizeStr(fields[0])
	if total <= 0 {
		return
	}
	current = int64(float64(total) * pct / 100)
	ok = true
	return
}

func parseSizeStr(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	unit = strings.ToLower(unit)
	switch {
	case strings.HasPrefix(unit, "gib") || strings.HasPrefix(unit, "gb"):
		return int64(val * 1073741824)
	case strings.HasPrefix(unit, "mib") || strings.HasPrefix(unit, "mb"):
		return int64(val * 1048576)
	case strings.HasPrefix(unit, "kib") || strings.HasPrefix(unit, "kb"):
		return int64(val * 1024)
	default:
		return int64(val)
	}
}
