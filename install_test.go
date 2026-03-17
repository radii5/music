//go:build ignore

// Test the installer's resume support and error handling

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChunkedDownload_ResumeSupport(t *testing.T) {
	// Create test content (1MB)
	testContent := strings.Repeat("x", 1024*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")

		if rangeHeader != "" {
			// Handle range request for resume
			if strings.HasPrefix(rangeHeader, "bytes=") {
				rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
				if strings.Contains(rangeStr, "-") {
					// Parse range start
					parts := strings.Split(rangeStr, "-")
					if len(parts) > 0 && parts[0] != "" {
						start := int64(0)
						if parts[0] != "" {
							start = int64(len(parts[0])) // Simplified for test
						}

						if start >= int64(len(testContent)) {
							w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
							return
						}

						remaining := testContent[start:]
						w.Header().Set("Content-Range",
							fmt.Sprintf("bytes %d-%d/%d", start, int64(len(testContent))-1, int64(len(testContent))))
						w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remaining)))
						w.WriteHeader(http.StatusPartialContent)
						w.Write([]byte(remaining))
						return
					}
				}
			}
		}

		// Regular request - return full content
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	// Create a partial file to simulate interrupted download
	tempFile := "test_resume.tmp"
	partialContent := testContent[:len(testContent)/2] // Half downloaded

	if err := os.WriteFile(tempFile, []byte(partialContent), 0644); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}
	defer os.Remove(tempFile)

	// Test resume download
	err := chunkedDownload(server.URL, tempFile, 4)
	if err != nil {
		t.Fatalf("Resume download failed: %v", err)
	}

	// Verify the complete file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read completed file: %v", err)
	}

	if len(data) != len(testContent) {
		t.Errorf("Expected %d bytes, got %d", len(testContent), len(data))
	}

	if string(data) != testContent {
		t.Error("Downloaded content doesn't match original")
	}
}

func TestChunkedDownload_TempFileCleanup(t *testing.T) {
	testContent := strings.Repeat("y", 1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tempFile := "test_cleanup.tmp"

	// Test successful download cleans up properly
	err := chunkedDownload(server.URL, tempFile, 2)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// File should exist and contain correct content
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Expected temp file to exist after successful download")
	}

	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(data) != testContent {
		t.Error("File content doesn't match expected")
	}

	os.Remove(tempFile)
}

func TestChunkedDownload_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate server error
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	tempFile := "test_error.tmp"

	err := chunkedDownload(server.URL, tempFile, 2)
	if err == nil {
		t.Fatal("Expected error for server failure")
	}

	if !strings.Contains(err.Error(), "chunk") && !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected chunk or server error, got: %v", err)
	}

	// Clean up
	os.Remove(tempFile)
}

func TestStreamingDownload_ResumeSupport(t *testing.T) {
	testContent := strings.Repeat("z", 512*1024) // 512KB

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")

		if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
			// Handle resume request
			rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
			if strings.HasSuffix(rangeStr, "-") {
				startStr := strings.TrimSuffix(rangeStr, "-")
				if startStr != "" {
					start := int64(len(startStr)) // Simplified
					if start < int64(len(testContent)) {
						remaining := testContent[start:]
						w.WriteHeader(http.StatusPartialContent)
						w.Write([]byte(remaining))
						return
					}
				}
			}
		}

		// Regular request
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tempFile := "test_streaming_resume.tmp"
	partialContent := testContent[:len(testContent)/4] // Quarter downloaded

	if err := os.WriteFile(tempFile, []byte(partialContent), 0644); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}
	defer os.Remove(tempFile)

	// Test streaming resume
	err := streamingDownload(server.URL, tempFile, int64(len(testContent)), int64(len(partialContent)))
	if err != nil {
		t.Fatalf("Streaming resume failed: %v", err)
	}

	// Verify complete file
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read completed file: %v", err)
	}

	if len(data) != len(testContent) {
		t.Errorf("Expected %d bytes, got %d", len(testContent), len(data))
	}
}

func TestProbeSize_ErrorHandling(t *testing.T) {
	// Test with invalid URL
	size := probeSize("http://invalid-url-that-does-not-exist.test")
	if size != 0 {
		t.Errorf("Expected 0 size for invalid URL, got %d", size)
	}

	// Test with server that returns error (no Content-Length header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	size = probeSize(server.URL)
	// When Content-Length is not set, it returns -1, but this is expected behavior
	if size != -1 && size != 0 {
		t.Errorf("Expected -1 or 0 size for error response, got %d", size)
	}
}

func TestChunkedDownload_ConcurrentAccess(t *testing.T) {
	testContent := strings.Repeat("concurrent", 10*1024) // 140KB

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add small delay to simulate real network
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testContent))
	}))
	defer server.Close()

	tempFile := "test_concurrent.tmp"

	// Test with multiple threads
	err := chunkedDownload(server.URL, tempFile, 8)
	if err != nil {
		t.Fatalf("Concurrent download failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != testContent {
		t.Error("Concurrent download content mismatch")
	}

	os.Remove(tempFile)
}
