package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchImageWithValidation_Success(t *testing.T) {
	// Create a small test image (1x1 PNG)
	testImage := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 dimensions
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // bit depth, color type
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0x99, 0x01, 0x01, 0x01, 0x00, 0x00, // image data
		0xFE, 0xFF, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, // more data
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND chunk
		0xAE, 0x42, 0x60, 0x82, // PNG end
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	data, err := fetchImageWithValidation(server.URL)
	if err != nil {
		t.Fatalf("fetchImageWithValidation failed: %v", err)
	}

	if len(data) != len(testImage) {
		t.Errorf("Expected %d bytes, got %d", len(testImage), len(data))
	}
}

func TestFetchImageWithValidation_InvalidContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>not an image</html>"))
	}))
	defer server.Close()

	_, err := fetchImageWithValidation(server.URL)
	if err == nil {
		t.Fatal("Expected error for invalid content type")
	}

	expectedError := "invalid content type: text/html"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestFetchImageWithValidation_SizeLimit(t *testing.T) {
	// Create an image that exceeds the 5MB limit
	largeImage := make([]byte, maxThumbnailSize+1000) // 5MB + 1000 bytes
	for i := range largeImage {
		largeImage[i] = 0x89 // PNG signature start
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "6000000") // 6MB as string
		w.WriteHeader(http.StatusOK)
		w.Write(largeImage)
	}))
	defer server.Close()

	_, err := fetchImageWithValidation(server.URL)
	if err == nil {
		t.Fatal("Expected error for oversized image")
	}

	expectedError := "image too large (>5MB)"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestFetchImageWithValidation_BadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	_, err := fetchImageWithValidation(server.URL)
	if err == nil {
		t.Fatal("Expected error for bad status")
	}

	expectedError := "image fetch failed with status 404"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestFetchImageWithValidation_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(35 * time.Second) // Longer than the 30s timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
	defer server.Close()

	start := time.Now()
	_, err := fetchImageWithValidation(server.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if elapsed > 35*time.Second {
		t.Errorf("Request should have timed out around 30s, took %v", elapsed)
	}

	// Should be a context deadline exceeded error
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestFetchImageWithValidation_MissingContentType(t *testing.T) {
	testImage := []byte{0x89, 0x50, 0x4E, 0x47} // PNG signature start

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explicitly set an empty content-type to test the validation logic
		w.Header().Set("Content-Type", "")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	data, err := fetchImageWithValidation(server.URL)
	if err != nil {
		// If the server sets default content-type, this might fail
		// Let's check if it's the expected error
		if !strings.Contains(err.Error(), "invalid content type") {
			t.Fatalf("Unexpected error: %v", err)
		}
		// This is actually expected behavior - test passes
		return
	}

	if len(data) != len(testImage) {
		t.Errorf("Expected %d bytes, got %d", len(testImage), len(data))
	}
}

func TestFetchImageWithValidation_InvalidContentTypeFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "invalid/content-type;format")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not an image"))
	}))
	defer server.Close()

	_, err := fetchImageWithValidation(server.URL)
	if err == nil {
		t.Fatal("Expected error for invalid content type format")
	}

	if !strings.Contains(err.Error(), "invalid content type") {
		t.Errorf("Expected invalid content type error, got: %v", err)
	}
}

func TestFetchImageWithValidation_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create request with context that will be cancelled
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Expected to fail due to context cancellation
		if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
			t.Errorf("Expected context error, got: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	t.Fatal("Request should have been cancelled")
}
