package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/radii5/radii5/internal/metadata"
	"github.com/radii5/radii5/internal/progress"
)

// Info holds metadata returned by yt-dlp --dump-json
type Info struct {
	Title      string  `json:"title"`
	Uploader   string  `json:"uploader"`
	Album      string  `json:"album"`
	Thumbnail  string  `json:"thumbnail"`
	URL        string  `json:"url"`         // direct audio URL (single format)
	Ext        string  `json:"ext"`
	Filesize   int64   `json:"filesize"`
	FilesizeApprox int64 `json:"filesize_approx"`
	ID         string  `json:"id"`
	WebpageURL string  `json:"webpage_url"`
}

// Download orchestrates the full download pipeline for a given URL.
func Download(url, format, outputDir string, threads int) error {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	// Check yt-dlp is available
	if err := checkYtDlp(); err != nil {
		return err
	}

	color.New(color.FgCyan, color.Bold).Printf("  Fetching info...\n")

	info, err := fetchInfo(url, format)
	if err != nil {
		return fmt.Errorf("fetching info: %w", err)
	}

	filename := sanitizeFilename(info.Title) + "." + format
	outPath := filepath.Join(outputDir, filename)

	color.New(color.FgWhite, color.Bold).Printf("  %s\n", info.Title)
	if info.Uploader != "" {
		color.New(color.FgHiBlack).Printf("  %s\n", info.Uploader)
	}
	fmt.Println()

	// Prefer chunked direct download if we have a direct URL, otherwise fall
	// back to yt-dlp for format conversion (e.g. mp3 from a webm source).
	if info.URL != "" && info.Ext == format {
		size := info.Filesize
		if size == 0 {
			size = info.FilesizeApprox
		}
		if err := chunkedDownload(info.URL, outPath, size, threads); err != nil {
			return fmt.Errorf("downloading: %w", err)
		}
	} else {
		if err := ytDlpDownload(url, format, outPath); err != nil {
			return fmt.Errorf("yt-dlp download: %w", err)
		}
	}

	// Write ID3 tags for MP3
	if format == "mp3" {
		_ = metadata.WriteMP3Tags(outPath, info.Title, info.Uploader, info.Album, info.Thumbnail)
	}

	color.New(color.FgGreen).Printf("  Saved → %s\n", outPath)
	return nil
}

// checkYtDlp verifies yt-dlp is installed and accessible.
func checkYtDlp() error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf(
			"yt-dlp not found — install it first:\n  https://github.com/yt-dlp/yt-dlp#installation",
		)
	}
	return nil
}

// fetchInfo runs yt-dlp --dump-json to get track metadata + direct stream URL.
func fetchInfo(url, format string) (*Info, error) {
	args := []string{
		"--dump-json",
		"--no-playlist",
		"-f", bestAudioFormat(format),
		url,
	}

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		// Surface yt-dlp stderr for better diagnostics
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var info Info
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parsing yt-dlp JSON: %w", err)
	}
	return &info, nil
}

// ytDlpDownload delegates to yt-dlp when format conversion is required.
func ytDlpDownload(url, format, outPath string) error {
	color.New(color.FgHiBlack).Println("  Converting via yt-dlp…")

	args := []string{
		"--no-playlist",
		"-x",
		"--audio-format", format,
		"--audio-quality", "0",
		"-o", outPath,
		url,
	}

	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// chunkedDownload downloads a file using parallel HTTP range requests.
func chunkedDownload(url, outPath string, totalSize int64, threads int) error {
	if threads <= 1 || totalSize == 0 {
		return simpleDownload(url, outPath, totalSize)
	}

	chunkSize := totalSize / int64(threads)
	type result struct {
		index int
		data  []byte
		err   error
	}

	results := make(chan result, threads)

	for i := 0; i < threads; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == threads-1 {
			end = totalSize - 1
		}

		go func(idx int, start, end int64) {
			data, err := fetchRange(url, start, end)
			results <- result{idx, data, err}
		}(i, start, end)
	}

	chunks := make([][]byte, threads)
	bar := progress.NewBar(totalSize)

	for i := 0; i < threads; i++ {
		r := <-results
		if r.err != nil {
			return fmt.Errorf("chunk %d: %w", r.index, r.err)
		}
		chunks[r.index] = r.data
		_, _ = bar.Write(r.data) // update progress
	}
	bar.Finish()

	// Assemble chunks into output file
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, chunk := range chunks {
		if _, err := f.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

// simpleDownload streams a URL to disk with a progress bar.
func simpleDownload(url, outPath string, totalSize int64) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	bar := progress.NewBar(totalSize)
	_, err = io.Copy(f, io.TeeReader(resp.Body, bar))
	bar.Finish()
	return err
}

// fetchRange performs an HTTP Range request and returns the bytes.
func fetchRange(url string, start, end int64) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Minute}
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

// bestAudioFormat returns a yt-dlp format selector for the desired output format.
func bestAudioFormat(format string) string {
	switch format {
	case "flac", "wav", "aiff":
		return "bestaudio[ext=flac]/bestaudio[ext=wav]/bestaudio"
	default:
		return "bestaudio[ext=m4a]/bestaudio[ext=mp3]/bestaudio"
	}
}

// sanitizeFilename removes characters that are illegal in filenames.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "", "\"", "", "<", "", ">", "", "|", "-",
	)
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}
