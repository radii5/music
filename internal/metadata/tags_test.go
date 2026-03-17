package metadata

import (
	"os"
	"testing"
)

func TestWriteMP3Tags(t *testing.T) {
	// Create a temporary MP3 file with proper ID3v2 header
	tmpFile, err := os.CreateTemp("", "test_*.mp3")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write minimal ID3v2 header (10 bytes) + MP3 frame
	header := []byte("ID3\x04\x00\x00\x00\x00\x00\x00") // ID3v2.4 header, no tags
	tmpFile.Write(header)
	tmpFile.Write([]byte{0xFF, 0xFB, 0x90, 0x00}) // Basic MP3 frame header
	tmpFile.Close()

	// Test tag writing
	err = WriteMP3Tags(tmpFile.Name(), "Test Title", "Test Artist", "Test Album", "")
	if err != nil {
		t.Fatalf("WriteMP3Tags failed: %v", err)
	}

	// Verify tags were written (reopen file to check)
	tag, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to reopen file: %v", err)
	}
	defer tag.Close()

	// Basic check: file should be larger than just the headers
	info, err := tag.Stat()
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Size() <= 14 { // 10 byte ID3 header + 4 byte MP3 frame
		t.Error("File size suggests tags were not written")
	}
}

func TestFetchImage(t *testing.T) {
	// This test would require a mock HTTP server
	// For now, just test error handling with invalid URL
	_, err := fetchImage("http://invalid-url-that-does-not-exist.test")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestWriteMP3TagsWithEmptyFields(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_empty_*.mp3")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write minimal ID3v2 header + MP3 frame
	header := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
	tmpFile.Write(header)
	tmpFile.Write([]byte{0xFF, 0xFB, 0x90, 0x00})
	tmpFile.Close()

	// Test with empty fields - should not fail
	err = WriteMP3Tags(tmpFile.Name(), "", "", "", "")
	if err != nil {
		t.Fatalf("WriteMP3Tags with empty fields failed: %v", err)
	}
}

// BenchmarkWriteMP3Tags benchmarks the tag writing operation
func BenchmarkWriteMP3Tags(b *testing.B) {
	tmpFile, err := os.CreateTemp("", "bench_*.mp3")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write minimal ID3v2 header + MP3 frame
	header := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
	tmpFile.Write(header)
	tmpFile.Write([]byte{0xFF, 0xFB, 0x90, 0x00})
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = WriteMP3Tags(tmpFile.Name(), "Benchmark Title", "Benchmark Artist", "Benchmark Album", "")
		if err != nil {
			b.Fatalf("WriteMP3Tags failed: %v", err)
		}
	}
}
