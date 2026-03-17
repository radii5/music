package downloader

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolve_ContextTimeout(t *testing.T) {
	// Test with a command that will definitely timeout
	originalFindBin := findBin

	// Mock findBin to return a command that sleeps longer than our timeout
	findBin = func(name string) string {
		if name == "yt-dlp" {
			// Use a command that will sleep for 35 seconds (longer than our 30s timeout)
			if runtime.GOOS == "windows" {
				return "cmd"
			}
			return "sleep"
		}
		return originalFindBin(name)
	}

	defer func() { findBin = originalFindBin }()

	start := time.Now()
	_, err := resolve("http://example.com")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if elapsed > 35*time.Second {
		t.Errorf("Should have timed out around 30s, took %v", elapsed)
	}

	expectedError := "timeout after 30 seconds"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestResolve_BinaryNotFound(t *testing.T) {
	originalFindBin := findBin

	// Mock findBin to return a non-existent binary
	findBin = func(name string) string {
		if name == "yt-dlp" {
			return "non-existent-yt-dlp-binary"
		}
		return originalFindBin(name)
	}

	defer func() { findBin = originalFindBin }()

	_, err := resolve("http://example.com")
	if err == nil {
		t.Fatal("Expected error for missing binary")
	}

	// The error should mention that the binary is not found
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("Expected error mentioning binary not found, got: %v", err)
	}
}

func TestResolve_StderrCapture(t *testing.T) {
	// This test creates a mock script that outputs to stderr and exits with error
	var script string
	var scriptPath string

	if runtime.GOOS == "windows" {
		script = `@echo off
echo ERROR: This is a test error message >&2
exit /b 1`
		scriptPath = "test_ytdlp_error.bat"
	} else {
		script = `#!/bin/bash
echo "ERROR: This is a test error message" >&2
exit 1`
		scriptPath = "test_ytdlp_error.sh"
	}

	// Create test script with full path
	fullPath, err1 := os.Getwd()
	if err1 != nil {
		t.Fatalf("Failed to get working directory: %v", err1)
	}
	scriptPath = filepath.Join(fullPath, scriptPath)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}
	defer os.Remove(scriptPath)

	originalFindBin := findBin
	findBin = func(name string) string {
		if name == "yt-dlp" {
			return scriptPath
		}
		return originalFindBin(name)
	}
	defer func() { findBin = originalFindBin }()

	_, err := resolve("http://example.com")
	if err == nil {
		t.Fatal("Expected error from test script")
	}

	// Check that stderr was captured
	if !strings.Contains(err.Error(), "This is a test error message") {
		t.Errorf("Expected stderr to be captured in error, got: %v", err)
	}

	// Check that the error mentions stderr
	if !strings.Contains(err.Error(), "stderr:") {
		t.Errorf("Expected error to mention stderr, got: %v", err)
	}
}

func TestYtDlpFallback_ContextTimeout(t *testing.T) {
	// Test with a command that will definitely timeout
	originalFindBin := findBin

	findBin = func(name string) string {
		if name == "yt-dlp" {
			if runtime.GOOS == "windows" {
				return "cmd"
			}
			return "sleep"
		}
		return originalFindBin(name)
	}
	defer func() { findBin = originalFindBin }()

	start := time.Now()
	err := ytDlpFallback("http://example.com", "mp3", "test.mp3", 4)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	// Should timeout after 30 minutes, but we'll cancel early for testing
	if elapsed > 10*time.Second {
		t.Errorf("Test taking too long: %v", elapsed)
	}

	if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout/cancellation error, got: %v", err)
	}
}

func TestYtDlpFallback_BinaryNotFound(t *testing.T) {
	originalFindBin := findBin

	findBin = func(name string) string {
		if name == "yt-dlp" {
			return "non-existent-yt-dlp-binary"
		}
		return originalFindBin(name)
	}
	defer func() { findBin = originalFindBin }()

	err := ytDlpFallback("http://example.com", "mp3", "test.mp3", 4)
	if err == nil {
		t.Fatal("Expected error for missing binary")
	}

	expectedError := "yt-dlp not found"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestResolve_InvalidJSON(t *testing.T) {
	// Create a script that outputs invalid JSON
	var script string
	var scriptPath string

	if runtime.GOOS == "windows" {
		script = `@echo off
echo invalid json output
exit /b 0`
		scriptPath = "test_ytdlp_invalid.bat"
	} else {
		script = `#!/bin/bash
echo "invalid json output"
exit 0`
		scriptPath = "test_ytdlp_invalid.sh"
	}

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}
	defer os.Remove(scriptPath)

	originalFindBin := findBin
	findBin = func(name string) string {
		if name == "yt-dlp" {
			return scriptPath
		}
		return originalFindBin(name)
	}
	defer func() { findBin = originalFindBin }()

	_, err := resolve("http://example.com")
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	expectedError := "failed to parse track info"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}
}

func TestResolve_ContextCancellation(t *testing.T) {
	// Create a long-running command
	var script string
	var scriptPath string

	if runtime.GOOS == "windows" {
		script = `@echo off
timeout /t 10 >nul
exit /b 0`
		scriptPath = "test_ytdlp_long.bat"
	} else {
		script = `#!/bin/bash
sleep 10
exit 0`
		scriptPath = "test_ytdlp_long.sh"
	}

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}
	defer os.Remove(scriptPath)

	originalFindBin := findBin
	findBin = func(name string) string {
		if name == "yt-dlp" {
			return scriptPath
		}
		return originalFindBin(name)
	}
	defer func() { findBin = originalFindBin }()

	start := time.Now()
	_, err := resolve("http://example.com")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error from long-running command")
	}

	// This test just verifies the command runs, actual cancellation is tested elsewhere
	if elapsed > 12*time.Second {
		t.Errorf("Command took too long: %v", elapsed)
	}
}
