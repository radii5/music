package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChunkDownloader_ProbeSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("Expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cd := NewChunkDownloader(4, nil)
	size, err := cd.probeSize(server.URL)

	if err != nil {
		t.Fatalf("probeSize failed: %v", err)
	}
	if size != 1024 {
		t.Errorf("Expected size 1024, got %d", size)
	}
}

func TestChunkDownloader_FetchRange(t *testing.T) {
	content := "0123456789"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			t.Error("Missing Range header")
		}
		
		if rangeHeader == "bytes=2-5" {
			w.Header().Set("Content-Range", "bytes 2-5/10")
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte(content[2:6]))
		} else {
			t.Errorf("Unexpected range: %s", rangeHeader)
		}
	}))
	defer server.Close()

	cd := NewChunkDownloader(4, nil)
	data, err := cd.fetchRange(server.URL, 2, 5)

	if err != nil {
		t.Fatalf("fetchRange failed: %v", err)
	}
	if string(data) != "2345" {
		t.Errorf("Expected '2345', got '%s'", string(data))
	}
}

func TestAtomicProgress(t *testing.T) {
	var progressCalls int
	var lastDownloaded, lastTotal int64

	progress := func(downloaded, total int64) {
		progressCalls++
		lastDownloaded = downloaded
		lastTotal = total
	}

	ap := NewAtomicProgress(100, progress)

	ap.Add(10)
	if progressCalls != 1 {
		t.Errorf("Expected 1 progress call, got %d", progressCalls)
	}
	if lastDownloaded != 10 || lastTotal != 100 {
		t.Errorf("Expected (10, 100), got (%d, %d)", lastDownloaded, lastTotal)
	}

	ap.Add(5)
	if ap.Get() != 15 {
		t.Errorf("Expected total 15, got %d", ap.Get())
	}
}

func TestChunkDownloader_StreamingFallback(t *testing.T) {
	content := strings.Repeat("x", 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK) // No Content-Length header
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	cd := NewChunkDownloader(8, nil)
	
	// This should fallback to streaming download
	err := cd.Download(server.URL, "test_output")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	
	// Clean up
	defer os.Remove("test_output")
}
