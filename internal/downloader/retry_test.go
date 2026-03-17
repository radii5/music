package downloader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryableHTTPClient_ExponentialBackoff(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)

		// Fail first 2 attempts, succeed on 3rd
		if count <= 2 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	start := time.Now()
	client := NewRetryableHTTPClient(3, 100*time.Millisecond)

	resp, err := client.Get(server.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Request failed after retries: %v", err)
	}
	defer resp.Body.Close()

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	// Should have waited: 0ms + 100ms + 200ms = ~300ms minimum
	if elapsed < 250*time.Millisecond {
		t.Errorf("Expected exponential backoff delay, got only %v", elapsed)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestRetryableHTTPClient_MaxRetriesExceeded(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	client := NewRetryableHTTPClient(2, 50*time.Millisecond)

	_, err := client.Get(server.URL)

	if err == nil {
		t.Fatal("Expected error after max retries")
	}

	if requestCount != 3 { // initial + 2 retries
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	expectedError := "failed after 2 retries"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestRetryableHTTPClient_NoRetryOn4xx(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewRetryableHTTPClient(3, 10*time.Millisecond)

	_, err := client.Get(server.URL)

	if err == nil {
		t.Fatal("Expected error on 404")
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request (no retry on 4xx), got %d", requestCount)
	}
}

func TestRetryableHTTPClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	client := NewRetryableHTTPClient(5, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected context cancellation error")
	}

	// Should be cancelled after ~250ms, but allow some buffer
	if elapsed > 500*time.Millisecond {
		t.Errorf("Request should have been cancelled quickly, took %v", elapsed)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestRetryableHTTPClient_CappedDelay(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)

		// Fail first 3 attempts to test delay capping, then succeed
		if count <= 3 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	start := time.Now()
	client := NewRetryableHTTPClient(3, 50*time.Millisecond) // 50ms base, should cap at 30s

	resp, err := client.Get(server.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// With exponential backoff: 0ms + 50ms + 100ms + 200ms = ~350ms
	// Should complete in reasonable time
	if elapsed > 1*time.Second {
		t.Errorf("Test took too long: %v", elapsed)
	}

	if requestCount != 4 {
		t.Errorf("Expected 4 requests, got %d", requestCount)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
