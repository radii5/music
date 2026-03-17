package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// Helper functions for compatibility with older Go versions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RetryableHTTPClient performs HTTP requests with exponential backoff retry
type RetryableHTTPClient struct {
	Client     *http.Client
	MaxRetries int
	BaseDelay  time.Duration
}

// NewRetryableHTTPClient creates a new retryable HTTP client
func NewRetryableHTTPClient(maxRetries int, baseDelay time.Duration) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		Client:     NewOptimizedHTTPClient(),
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
	}
}

// Do performs an HTTP request with retry logic
func (rhc *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= rhc.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s
			delay := time.Duration(1<<uint(attempt-1)) * rhc.BaseDelay
			if delay > 30*time.Second {
				delay = 30 * time.Second // Cap at 30s
			}
			time.Sleep(delay)
		}

		resp, err := rhc.Client.Do(req)
		if err == nil {
			// Check for successful status codes
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}
			resp.Body.Close() // Close body on error status

			// Retry on 5xx errors, but not 4xx
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				continue
			}

			// Don't retry on 4xx errors
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		lastErr = err

		// Retry on network errors, but not on context cancellation
		if req.Context().Err() == context.Canceled {
			return nil, req.Context().Err()
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", rhc.MaxRetries, lastErr)
}

// Get performs a GET request with retry logic
func (rhc *RetryableHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return rhc.Do(req)
}

// Head performs a HEAD request with retry logic
func (rhc *RetryableHTTPClient) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return rhc.Do(req)
}

// ProgressCallback reports download progress
type ProgressCallback func(downloaded, total int64)

// ChunkDownloader handles parallel chunked downloads
type ChunkDownloader struct {
	Client    *RetryableHTTPClient
	Progress  ProgressCallback
	Threads   int
	ChunkSize int64
}

// NewOptimizedHTTPClient creates an HTTP client with optimized transport settings
func NewOptimizedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     20,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableCompression:  false,
		},
		Timeout: 30 * time.Minute,
	}
}

// DetermineThreads calculates optimal thread count based on file size and user preference
func DetermineThreads(fileSize int64, userThreads int) int {
	if userThreads > 0 {
		return userThreads // respect --threads flag
	}

	// Adaptive logic based on file size
	switch {
	case fileSize < 5*1024*1024: // < 5MB
		return 2
	case fileSize < 20*1024*1024: // < 20MB
		return 4
	case fileSize < 100*1024*1024: // < 100MB
		return 8
	default: // >= 100MB
		return 12
	}
}

// NewChunkDownloader creates a new chunk downloader with optimized defaults
func NewChunkDownloader(threads int, progress ProgressCallback) *ChunkDownloader {
	return &ChunkDownloader{
		Client:    NewRetryableHTTPClient(3, time.Second),
		Progress:  progress,
		Threads:   threads,
		ChunkSize: 1024 * 1024, // 1MB chunks
	}
}

// ProbeBandwidth measures download speed by downloading a small sample
func ProbeBandwidth(url string) (float64, error) {
	client := NewOptimizedHTTPClient()

	// Download first 1MB as a sample
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-1048575") // First 1MB
	req.Header.Set("User-Agent", "radii5-bandwidth-probe")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1048576))
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(start)
	if elapsed.Seconds() == 0 {
		return 0, fmt.Errorf("elapsed time is zero")
	}

	// Return speed in MB/s
	mbps := float64(len(data)) / (1 << 20) / elapsed.Seconds()
	return mbps, nil
}

// OptimalThreadsForBandwidth calculates optimal threads based on bandwidth
func OptimalThreadsForBandwidth(mbps float64, fileSize int64) int {
	// Base logic from file size
	baseThreads := DetermineThreads(fileSize, 0)

	// Adjust based on bandwidth
	switch {
	case mbps < 1: // Slow connection
		return max(2, baseThreads/2)
	case mbps < 5: // Moderate connection
		return baseThreads
	case mbps < 20: // Fast connection
		return min(baseThreads*2, 16)
	default: // Very fast connection
		return min(baseThreads*3, 24)
	}
}

// NewAdaptiveChunkDownloader creates a downloader that determines threads based on file size and bandwidth
func NewAdaptiveChunkDownloader(userThreads int, progress ProgressCallback) *ChunkDownloader {
	return &ChunkDownloader{
		Client:    NewRetryableHTTPClient(3, time.Second),
		Progress:  progress,
		Threads:   userThreads, // Will be updated after probing
		ChunkSize: 1024 * 1024, // 1MB chunks
	}
}

// Download performs a parallel chunked download
func (cd *ChunkDownloader) Download(url, dest string) error {
	size, err := cd.probeSize(url)
	if err != nil {
		return err
	}

	// Update threads based on file size and bandwidth if using adaptive mode
	if cd.Threads == 0 || (cd.Threads == 8 && size > 0) { // 8 is the old default
		// Try bandwidth probing first
		if mbps, err := ProbeBandwidth(url); err == nil {
			cd.Threads = OptimalThreadsForBandwidth(mbps, size)
		} else {
			// Fallback to size-based logic
			cd.Threads = DetermineThreads(size, cd.Threads)
		}
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
