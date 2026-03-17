//go:build ignore

// radii5 installer
// Usage: go run install.go
//
// Downloads yt-dlp and radii5 (latest releases) using parallel chunk
// downloading — the same technique radii5 uses for music.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	threads        = 8
	ytDlpAPIURL    = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
	radii5APIURL   = "https://api.github.com/repos/radii5/music/releases/latest"
	installDirUnix = "/usr/local/bin"
)

// ── colour helpers (no deps) ──────────────────────────────────────────────────

const (
	clReset  = "\033[0m"
	clCyan   = "\033[36m"
	clGreen  = "\033[32m"
	clRed    = "\033[31m"
	clBold   = "\033[1m"
	clDim    = "\033[2m"
	clYellow = "\033[33m"
)

func cyan(s string) string   { return clCyan + s + clReset }
func green(s string) string  { return clGreen + s + clReset }
func red(s string) string    { return clRed + s + clReset }
func bold(s string) string   { return clBold + s + clReset }
func dim(s string) string    { return clDim + s + clReset }
func yellow(s string) string { return clYellow + s + clReset }

// ── progress bar ──────────────────────────────────────────────────────────────

type bar struct {
	total     int64
	current   atomic.Int64
	lastPrint int64
	mu        sync.Mutex
}

func newBar(total int64) *bar { return &bar{total: total} }

func (b *bar) add(n int64) {
	cur := b.current.Add(n)
	b.mu.Lock()
	defer b.mu.Unlock()

	if cur-b.lastPrint < 51200 && cur < b.total {
		return
	}
	b.lastPrint = cur

	const width = 30
	var filled int
	var pct float64
	if b.total > 0 {
		pct = float64(cur) / float64(b.total)
		filled = int(pct * float64(width))
		if filled > width {
			filled = width
		}
	} else {
		filled = width / 2
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	speed := ""
	if b.total > 0 {
		speed = fmt.Sprintf("  %s / %s  (%.0f%%)", fmtBytes(cur), fmtBytes(b.total), pct*100)
	} else {
		speed = fmt.Sprintf("  %s", fmtBytes(cur))
	}
	fmt.Printf("\r  %s[%s]%s%s", clCyan, bar, clReset, speed)
}

func (b *bar) finish() {
	const width = 30
	bar := strings.Repeat("█", width)
	if b.total > 0 {
		fmt.Printf("\r  %s[%s]%s  %s ✓\n", clGreen, bar, clReset, fmtBytes(b.total))
	} else {
		fmt.Println()
	}
}

func fmtBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// ── GitHub release resolution ─────────────────────────────────────────────────

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func latestRelease(apiURL string) (*ghRelease, error) {
	client := createOptimizedClient()
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "radii5-installer")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, apiURL)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// createOptimizedClient creates an HTTP client with optimized transport settings
func createOptimizedClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     20,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableCompression:  false,
		},
		Timeout: 15 * time.Second,
	}
}

// ── platform detection ────────────────────────────────────────────────────────

type platform struct {
	goos   string
	goarch string
}

func currentPlatform() platform {
	return platform{goos: runtime.GOOS, goarch: runtime.GOARCH}
}

// ytDlpAssetName returns the yt-dlp binary name for the current platform.
func ytDlpAssetName(p platform) string {
	switch p.goos {
	case "windows":
		return "yt-dlp.exe"
	case "darwin":
		switch p.goarch {
		case "arm64":
			return "yt-dlp_macos"
		default:
			return "yt-dlp_macos_legacy"
		}
	default: // linux and others
		switch p.goarch {
		case "arm64":
			return "yt-dlp_linux_aarch64"
		case "arm":
			return "yt-dlp_linux_armv7l"
		default:
			return "yt-dlp_linux"
		}
	}
}

// radii5AssetName returns the radii5 binary name for the current platform.
func radii5AssetName(p platform) string {
	var suffix string
	switch p.goos {
	case "windows":
		switch p.goarch {
		case "arm64":
			suffix = "windows-arm64.exe"
		default:
			suffix = "windows-amd64.exe"
		}
	case "darwin":
		switch p.goarch {
		case "arm64":
			suffix = "macos-arm64"
		default:
			suffix = "macos-amd64"
		}
	default:
		switch p.goarch {
		case "arm64":
			suffix = "linux-arm64"
		default:
			suffix = "linux-amd64"
		}
	}
	return "radii5-" + suffix
}

func findAsset(rel *ghRelease, name string) *ghAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

// ── chunked downloader ────────────────────────────────────────────────────────

// probeSize does a HEAD request to get Content-Length.
func probeSize(url string) int64 {
	client := createOptimizedClient()
	resp, err := client.Head(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.ContentLength
}

// fetchRange fetches bytes [start, end] via HTTP Range.
func fetchRange(url string, start, end int64) ([]byte, error) {
	client := createOptimizedClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// chunkedDownload downloads url to dest using n parallel Range requests.
// Includes resume support by checking for existing temp files.
func chunkedDownload(url, dest string, n int) error {
	size := probeSize(url)

	b := newBar(size)

	// Check for existing temp file to resume download
	var existingSize int64
	if fi, err := os.Stat(dest); err == nil {
		existingSize = fi.Size()
		b.add(existingSize) // Update progress bar with existing data
		fmt.Printf("  → Resuming download from %s\n", fmtBytes(existingSize))
	}

	if size == 0 || n <= 1 {
		// Fallback: simple streaming download
		return streamingDownload(url, dest, size, existingSize)
	}

	type result struct {
		idx  int
		data []byte
		err  error
	}

	chunkSize := size / int64(n)
	results := make(chan result, n)

	for i := 0; i < n; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == n-1 {
			end = size - 1
		}

		// Skip chunks that are already downloaded
		if existingSize > 0 && start < existingSize {
			if end < existingSize {
				// Chunk is fully downloaded, skip it
				continue
			} else {
				// Partial chunk, adjust start position
				start = existingSize
			}
		}

		go func(idx int, s, e int64) {
			data, err := fetchRange(url, s, e)
			if err == nil {
				b.add(int64(len(data)))
			}
			results <- result{idx, data, err}
		}(i, start, end)
	}

	chunks := make([][]byte, n)
	received := 0
	expected := n
	if existingSize > 0 {
		expected = 0
		for i := 0; i < n; i++ {
			start := int64(i) * chunkSize
			if start >= existingSize {
				expected++
			}
		}
	}

	for i := 0; i < expected; i++ {
		r := <-results
		if r.err != nil {
			return fmt.Errorf("chunk %d failed: %w", r.idx, r.err)
		}
		chunks[r.idx] = r.data
		received++
	}

	if received != expected {
		return fmt.Errorf("expected %d chunks, received %d", expected, received)
	}

	b.finish()

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, chunk := range chunks {
		if chunk != nil { // Only write chunks that were downloaded
			if _, err := f.Write(chunk); err != nil {
				return err
			}
		}
	}
	return nil
}

// streamingDownload handles simple streaming download with resume support
func streamingDownload(url, dest string, totalSize, existingSize int64) error {
	client := createOptimizedClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Add Range header for resume support
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
		nr, rerr := resp.Body.Read(buf)
		if nr > 0 {
			if _, werr := f.Write(buf[:nr]); werr != nil {
				return werr
			}
			downloaded += int64(nr)
			// Progress already shown by chunked downloader
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// ── install helpers ───────────────────────────────────────────────────────────

// installPath returns where the binary should be placed and its final name.
func installPath(binaryName string) (string, error) {
	if runtime.GOOS == "windows" {
		// Install next to the installer on Windows (or LocalAppData\radii5)
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = "."
		}
		dir := filepath.Join(appData, "radii5")
		return filepath.Join(dir, binaryName), nil
	}
	return filepath.Join(installDirUnix, binaryName), nil
}

func makeExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil // not needed
	}
	return os.Chmod(path, 0755)
}

func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// alreadyInstalled returns true if the binary exists on PATH.
func alreadyInstalled(name string) bool {
	path := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		path = name + ".exe"
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(dir, path)); err == nil {
			return true
		}
	}
	return false
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	p := currentPlatform()

	fmt.Println()
	fmt.Printf("  %s\n", bold(cyan("radii5 installer")))
	fmt.Printf("  %s\n", dim(fmt.Sprintf("platform: %s/%s", p.goos, p.goarch)))
	fmt.Println()

	// ── 1. yt-dlp ────────────────────────────────────────────────────────────
	fmt.Printf("  %s  yt-dlp\n", cyan("→"))

	ytRel, err := latestRelease(ytDlpAPIURL)
	if err != nil {
		fmt.Println(red("  ✗ Could not fetch yt-dlp release: " + err.Error()))
		os.Exit(1)
	}

	assetName := ytDlpAssetName(p)
	asset := findAsset(ytRel, assetName)
	if asset == nil {
		fmt.Printf(red("  ✗ No yt-dlp asset found for this platform (looked for %q)\n"), assetName)
		os.Exit(1)
	}

	ytBinName := "yt-dlp"
	if p.goos == "windows" {
		ytBinName = "yt-dlp.exe"
	}
	ytDest, err := installPath(ytBinName)
	if err != nil {
		fmt.Println(red("  ✗ " + err.Error()))
		os.Exit(1)
	}

	if alreadyInstalled("yt-dlp") {
		fmt.Printf("  %s already installed, skipping\n\n", dim("yt-dlp"))
	} else {
		fmt.Printf("  %s  %s\n", dim("version"), ytRel.TagName)
		fmt.Printf("  %s  %s\n", dim("dest   "), ytDest)
		fmt.Println()

		if err := ensureDir(ytDest); err != nil {
			fmt.Println(red("  ✗ " + err.Error()))
			os.Exit(1)
		}
		if err := chunkedDownload(asset.BrowserDownloadURL, ytDest, threads); err != nil {
			fmt.Println(red("\n  ✗ Download failed: " + err.Error()))
			os.Exit(1)
		}
		if err := makeExecutable(ytDest); err != nil {
			fmt.Println(yellow("  ⚠ Could not chmod (try running with sudo): " + err.Error()))
		}
		fmt.Printf("  %s yt-dlp %s\n\n", green("✓"), ytRel.TagName)
	}

	// ── 2. radii5 ────────────────────────────────────────────────────────────
	fmt.Printf("  %s  radii5\n", cyan("→"))

	r5Rel, err := latestRelease(radii5APIURL)
	if err != nil {
		fmt.Println(red("  ✗ Could not fetch radii5 release: " + err.Error()))
		fmt.Println(dim("  (Has the repo been published and tagged yet?)"))
		os.Exit(1)
	}

	r5AssetName := radii5AssetName(p)
	r5Asset := findAsset(r5Rel, r5AssetName)
	if r5Asset == nil {
		fmt.Printf(red("  ✗ No radii5 asset found for this platform (looked for %q)\n"), r5AssetName)
		os.Exit(1)
	}

	r5BinName := "radii5"
	if p.goos == "windows" {
		r5BinName = "radii5.exe"
	}
	r5Dest, err := installPath(r5BinName)
	if err != nil {
		fmt.Println(red("  ✗ " + err.Error()))
		os.Exit(1)
	}

	fmt.Printf("  %s  %s\n", dim("version"), r5Rel.TagName)
	fmt.Printf("  %s  %s\n", dim("dest   "), r5Dest)
	fmt.Println()

	if err := ensureDir(r5Dest); err != nil {
		fmt.Println(red("  ✗ " + err.Error()))
		os.Exit(1)
	}
	if err := chunkedDownload(r5Asset.BrowserDownloadURL, r5Dest, threads); err != nil {
		fmt.Println(red("\n  ✗ Download failed: " + err.Error()))
		os.Exit(1)
	}
	if err := makeExecutable(r5Dest); err != nil {
		fmt.Println(yellow("  ⚠ Could not chmod (try running with sudo): " + err.Error()))
	}
	fmt.Printf("  %s radii5 %s\n\n", green("✓"), r5Rel.TagName)

	// ── done ─────────────────────────────────────────────────────────────────
	fmt.Printf("  %s\n", bold(green("All done!")))
	fmt.Println()

	if runtime.GOOS == "windows" {
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = "."
		}
		installDir := filepath.Join(appData, "radii5")
		fmt.Printf("  %s\n", dim("Add this to your PATH if not already there:"))
		fmt.Printf("  %s\n", yellow(installDir))
	} else {
		fmt.Printf("  %s\n", dim("Try it:"))
		fmt.Printf("  %s\n", cyan("  radii5 --version"))
	}
	fmt.Println()
}
