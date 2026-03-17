package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// ProgressCallback reports download progress
type ProgressCallback func(downloaded, total int64)

// ChunkDownloader handles parallel chunked downloads
type ChunkDownloader struct {
	Client    *http.Client
	Progress  ProgressCallback
	Threads   int
	ChunkSize int64
}

// NewChunkDownloader creates a new chunk downloader with sensible defaults
func NewChunkDownloader(threads int, progress ProgressCallback) *ChunkDownloader {
	if threads <= 0 {
		threads = 8
	}
	return &ChunkDownloader{
		Client: &http.Client{
			Timeout: 30 * time.Minute,
		},
		Progress:  progress,
		Threads:   threads,
		ChunkSize: 1024 * 1024, // 1MB chunks
	}
}

// Download performs a parallel chunked download
func (cd *ChunkDownloader) Download(url, dest string) error {
	size, err := cd.probeSize(url)
	if err != nil {
		return err
	}

	if cd.Progress != nil {
		cd.Progress(0, size)
	}

	if size == 0 || cd.Threads <= 1 {
		return cd.streamingDownload(url, dest, size)
	}

	return cd.chunkedDownload(url, dest, size)
}

// probeSize gets the total file size via HEAD request
func (cd *ChunkDownloader) probeSize(url string) (int64, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "radii5-downloader")

	resp, err := cd.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD request failed: %d", resp.StatusCode)
	}

	return resp.ContentLength, nil
}

// streamingDownload falls back to simple streaming download
func (cd *ChunkDownloader) streamingDownload(url, dest string, size int64) error {
	resp, err := cd.Client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET request failed: %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
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
			if cd.Progress != nil {
				cd.Progress(downloaded, size)
			}
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

// chunkedDownload downloads using parallel Range requests
func (cd *ChunkDownloader) chunkedDownload(url, dest string, totalSize int64) error {
	type chunkResult struct {
		idx  int
		data []byte
		err  error
	}

	chunkSize := totalSize / int64(cd.Threads)
	results := make(chan chunkResult, cd.Threads)

	// Start chunk downloads
	for i := 0; i < cd.Threads; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == cd.Threads-1 {
			end = totalSize - 1
		}

		go func(idx int, s, e int64) {
			data, err := cd.fetchRange(url, s, e)
			results <- chunkResult{idx, data, err}
		}(i, start, end)
	}

	// Collect results
	chunks := make([][]byte, cd.Threads)
	for i := 0; i < cd.Threads; i++ {
		r := <-results
		if r.err != nil {
			return fmt.Errorf("chunk %d failed: %w", r.idx, r.err)
		}
		chunks[r.idx] = r.data
	}

	// Write file
	f, err := os.Create(dest)
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

// fetchRange downloads a specific byte range
func (cd *ChunkDownloader) fetchRange(url string, start, end int64) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", "radii5-downloader")

	resp, err := cd.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// AtomicProgress provides thread-safe progress tracking
type AtomicProgress struct {
	downloaded int64
	total      int64
	callback   func(downloaded, total int64)
}

// NewAtomicProgress creates a new atomic progress tracker
func NewAtomicProgress(total int64, callback func(downloaded, total int64)) *AtomicProgress {
	return &AtomicProgress{
		total:    total,
		callback: callback,
	}
}

// Add increments the downloaded count atomically
func (ap *AtomicProgress) Add(n int64) {
	new := atomic.AddInt64(&ap.downloaded, n)
	if ap.callback != nil {
		ap.callback(new, ap.total)
	}
}

// Get returns current downloaded count
func (ap *AtomicProgress) Get() int64 {
	return atomic.LoadInt64(&ap.downloaded)
}
