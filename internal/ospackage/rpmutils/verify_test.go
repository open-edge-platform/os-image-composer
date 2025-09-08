package rpmutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResult(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected Result
	}{
		{
			name: "successful verification",
			result: Result{
				Path:     "/path/to/test.rpm",
				OK:       true,
				Duration: time.Millisecond * 100,
				Error:    nil,
			},
			expected: Result{
				Path:     "/path/to/test.rpm",
				OK:       true,
				Duration: time.Millisecond * 100,
				Error:    nil,
			},
		},
		{
			name: "failed verification",
			result: Result{
				Path:     "/path/to/bad.rpm",
				OK:       false,
				Duration: time.Millisecond * 50,
				Error:    fmt.Errorf("verification failed"),
			},
			expected: Result{
				Path:     "/path/to/bad.rpm",
				OK:       false,
				Duration: time.Millisecond * 50,
				Error:    fmt.Errorf("verification failed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Path != tt.expected.Path {
				t.Errorf("Path = %v, want %v", tt.result.Path, tt.expected.Path)
			}
			if tt.result.OK != tt.expected.OK {
				t.Errorf("OK = %v, want %v", tt.result.OK, tt.expected.OK)
			}
			if tt.result.Duration != tt.expected.Duration {
				t.Errorf("Duration = %v, want %v", tt.result.Duration, tt.expected.Duration)
			}
			if (tt.result.Error == nil) != (tt.expected.Error == nil) {
				t.Errorf("Error = %v, want %v", tt.result.Error, tt.expected.Error)
			}
		})
	}
}
func TestVerifyAll(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "verify_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	rpmFile1 := filepath.Join(tmpDir, "test1.rpm")
	rpmFile2 := filepath.Join(tmpDir, "test2.rpm")
	invalidPubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")

	if err := os.WriteFile(rpmFile1, []byte("fake rpm content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rpmFile2, []byte("fake rpm content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPubkeyFile, []byte("invalid key content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		paths      []string
		pubkeyPath string
		workers    int
		wantLen    int
	}{
		{
			name:       "empty paths",
			paths:      []string{},
			pubkeyPath: invalidPubkeyFile,
			workers:    1,
			wantLen:    0,
		},
		{
			name:       "single file",
			paths:      []string{rpmFile1},
			pubkeyPath: invalidPubkeyFile,
			workers:    1,
			wantLen:    1,
		},
		{
			name:       "multiple files",
			paths:      []string{rpmFile1, rpmFile2},
			pubkeyPath: invalidPubkeyFile,
			workers:    2,
			wantLen:    2,
		},
		{
			name:       "multiple workers",
			paths:      []string{rpmFile1, rpmFile2},
			pubkeyPath: invalidPubkeyFile,
			workers:    5,
			wantLen:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := VerifyAll(tt.paths, tt.pubkeyPath, tt.workers)

			if len(results) != tt.wantLen {
				t.Errorf("VerifyAll() returned %d results, want %d", len(results), tt.wantLen)
			}

			// Check that all results have the correct paths and expected failures
			for i, result := range results {
				if i < len(tt.paths) && result.Path != tt.paths[i] {
					t.Errorf("Result[%d].Path = %v, want %v", i, result.Path, tt.paths[i])
				}

				// All our test files will fail verification (fake content)
				if result.OK {
					t.Errorf("Result[%d].OK = true, expected false for fake RPM", i)
				}

				if result.Error == nil {
					t.Errorf("Result[%d].Error = nil, expected error for fake RPM", i)
				}

				if result.Duration < 0 {
					t.Errorf("Result[%d].Duration = %v, expected non-negative duration", i, result.Duration)
				}
			}
		})
	}
}

func TestVerifyAllWithNonExistentFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "verify_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	invalidPubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")
	if err := os.WriteFile(invalidPubkeyFile, []byte("invalid key content"), 0644); err != nil {
		t.Fatal(err)
	}

	nonExistentFile := filepath.Join(tmpDir, "nonexistent.rpm")
	results := VerifyAll([]string{nonExistentFile}, invalidPubkeyFile, 1)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.OK {
		t.Error("Expected OK to be false for non-existent file")
	}
	if result.Error == nil {
		t.Error("Expected error for non-existent file")
	}
	// The error could be about opening rpm or loading keyring, both are acceptable for a non-existent file
	if !strings.Contains(result.Error.Error(), "opening rpm") && !strings.Contains(result.Error.Error(), "loading keyring") {
		t.Errorf("Expected error about opening rpm or loading keyring, got: %v", result.Error)
	}
}

func TestVerifyAllWithInvalidPubkey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "verify_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	rpmFile := filepath.Join(tmpDir, "test.rpm")
	invalidPubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")

	if err := os.WriteFile(rpmFile, []byte("fake rpm content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPubkeyFile, []byte("invalid key content"), 0644); err != nil {
		t.Fatal(err)
	}

	results := VerifyAll([]string{rpmFile}, invalidPubkeyFile, 1)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.OK {
		t.Error("Expected OK to be false for invalid pubkey")
	}
	if result.Error == nil {
		t.Error("Expected error for invalid pubkey")
	}
	if !strings.Contains(result.Error.Error(), "loading keyring") {
		t.Errorf("Expected error about loading keyring, got: %v", result.Error)
	}
}

func TestVerifyWithGoRpm(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "verify_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name          string
		setupRPM      func() string
		setupPubkey   func() string
		expectedError string
	}{
		{
			name: "non-existent rpm file",
			setupRPM: func() string {
				return filepath.Join(tmpDir, "nonexistent.rpm")
			},
			setupPubkey: func() string {
				pubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")
				_ = os.WriteFile(pubkeyFile, []byte("invalid key content"), 0644)
				return pubkeyFile
			},
			expectedError: "loading keyring", // Will fail at keyring loading first
		},
		{
			name: "non-existent pubkey file",
			setupRPM: func() string {
				rpmFile := filepath.Join(tmpDir, "test.rpm")
				_ = os.WriteFile(rpmFile, []byte("fake rpm"), 0644)
				return rpmFile
			},
			setupPubkey: func() string {
				return filepath.Join(tmpDir, "nonexistent.gpg")
			},
			expectedError: "opening public key",
		},
		{
			name: "invalid pubkey content",
			setupRPM: func() string {
				rpmFile := filepath.Join(tmpDir, "test.rpm")
				_ = os.WriteFile(rpmFile, []byte("fake rpm"), 0644)
				return rpmFile
			},
			setupPubkey: func() string {
				pubkeyFile := filepath.Join(tmpDir, "invalid.gpg")
				_ = os.WriteFile(pubkeyFile, []byte("invalid content"), 0644)
				return pubkeyFile
			},
			expectedError: "loading keyring",
		},
		{
			name: "invalid rpm content",
			setupRPM: func() string {
				rpmFile := filepath.Join(tmpDir, "invalid.rpm")
				_ = os.WriteFile(rpmFile, []byte("not an rpm file"), 0644)
				return rpmFile
			},
			setupPubkey: func() string {
				pubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")
				_ = os.WriteFile(pubkeyFile, []byte("invalid key content"), 0644)
				return pubkeyFile
			},
			expectedError: "loading keyring", // Will fail at keyring loading first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rpmPath := tt.setupRPM()
			pubkeyPath := tt.setupPubkey()

			err := verifyWithGoRpm(rpmPath, pubkeyPath)

			if err == nil {
				t.Error("Expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing %q, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestVerifyAllConcurrency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "verify_concurrent_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple test files
	numFiles := 10
	paths := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		rpmFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.rpm", i))
		if err := os.WriteFile(rpmFile, []byte(fmt.Sprintf("fake rpm content %d", i)), 0644); err != nil {
			t.Fatal(err)
		}
		paths[i] = rpmFile
	}

	invalidPubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")
	if err := os.WriteFile(invalidPubkeyFile, []byte("invalid key content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with different worker counts
	workerCounts := []int{1, 3, 5, 10}

	for _, workers := range workerCounts {
		t.Run(fmt.Sprintf("workers_%d", workers), func(t *testing.T) {
			start := time.Now()
			results := VerifyAll(paths, invalidPubkeyFile, workers)
			duration := time.Since(start)

			if len(results) != numFiles {
				t.Errorf("Expected %d results, got %d", numFiles, len(results))
			}

			// Verify all files were processed
			processedPaths := make(map[string]bool)
			for _, result := range results {
				processedPaths[result.Path] = true

				// All should fail (fake content)
				if result.OK {
					t.Errorf("Expected file %s to fail verification", result.Path)
				}
				if result.Error == nil {
					t.Errorf("Expected error for file %s", result.Path)
				}
			}

			for _, path := range paths {
				if !processedPaths[path] {
					t.Errorf("File %s was not processed", path)
				}
			}

			t.Logf("Processed %d files with %d workers in %v", numFiles, workers, duration)
		})
	}
}

func TestVerifyAllResultsOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "verify_order_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files with specific names
	paths := []string{
		filepath.Join(tmpDir, "a.rpm"),
		filepath.Join(tmpDir, "b.rpm"),
		filepath.Join(tmpDir, "c.rpm"),
	}

	for _, path := range paths {
		if err := os.WriteFile(path, []byte("fake rpm"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	invalidPubkeyFile := filepath.Join(tmpDir, "invalid_pubkey.gpg")
	if err := os.WriteFile(invalidPubkeyFile, []byte("invalid key content"), 0644); err != nil {
		t.Fatal(err)
	}

	results := VerifyAll(paths, invalidPubkeyFile, 2)

	if len(results) != len(paths) {
		t.Fatalf("Expected %d results, got %d", len(paths), len(results))
	}

	// Results should maintain the same order as input paths
	for i, result := range results {
		if result.Path != paths[i] {
			t.Errorf("Result[%d].Path = %s, want %s", i, result.Path, paths[i])
		}
	}
}
