package progress

import (
	"strings"
	"testing"
)

func TestBar_Write(t *testing.T) {
	bar := NewBar(1000)
	
	// Test writing data
	data := []byte("hello world")
	n, err := bar.Write(data)
	
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	
	current := bar.current.Load()
	if current != int64(len(data)) {
		t.Errorf("Expected current %d, got %d", len(data), current)
	}
}

func TestBar_Set(t *testing.T) {
	bar := NewBar(1000)
	
	bar.Set(500)
	current := bar.current.Load()
	if current != 500 {
		t.Errorf("Expected current 500, got %d", current)
	}
	
	// Test setting to same value (should not panic)
	bar.Set(500)
}

func TestBar_Finish(t *testing.T) {
	bar := NewBar(1000)
	
	// Test finishing
	bar.Finish()
	
	// Test double finish (should not panic)
	bar.Finish()
	
	// Test that further writes don't render after finish
	data := []byte("test")
	bar.Write(data)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}
	
	for _, test := range tests {
		result := formatBytes(test.input)
		if result != test.expected {
			t.Errorf("formatBytes(%d) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestBar_RenderThrottling(t *testing.T) {
	bar := NewBar(1000000) // 1MB
	
	// Capture stdout to test throttling
	// This is a basic test - in real usage, throttling prevents excessive redraws
	
	// Write small amounts that should be throttled
	for i := 0; i < 10; i++ {
		bar.Write([]byte("x")) // 1 byte each
	}
	
	// The current should be 10, but rendering should be throttled
	current := bar.current.Load()
	if current != 10 {
		t.Errorf("Expected current 10, got %d", current)
	}
}

func TestBar_UnknownSize(t *testing.T) {
	bar := NewBar(0) // Unknown size
	
	data := []byte("test data")
	bar.Write(data)
	
	current := bar.current.Load()
	if current != int64(len(data)) {
		t.Errorf("Expected current %d, got %d", len(data), current)
	}
	
	// Should handle unknown size gracefully
	bar.Finish()
}

func TestBar_ConcurrentWrites(t *testing.T) {
	bar := NewBar(1000)
	done := make(chan bool, 10)
	
	// Test concurrent writes
	for i := 0; i < 10; i++ {
		go func() {
			bar.Write([]byte("test"))
			done <- true
		}()
	}
	
	// Wait for all writes to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Should have written 40 bytes total (10 * 4)
	current := bar.current.Load()
	if current != 40 {
		t.Errorf("Expected current 40, got %d", current)
	}
}

func TestBar_RenderOutput(t *testing.T) {
	bar := NewBar(100)
	
	// This test is basic - it just ensures render doesn't panic
	// In a real test environment, you'd capture stdout and verify the format
	bar.Set(50) // 50% progress
	bar.Set(100) // 100% progress
	bar.Finish()
	
	// Should not panic
}

// BenchmarkWrite benchmarks the Write operation
func BenchmarkWrite(b *testing.B) {
	bar := NewBar(1000000)
	data := []byte(strings.Repeat("x", 1024)) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bar.Write(data)
	}
}
