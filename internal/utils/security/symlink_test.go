package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSymlink_RegularFile(t *testing.T) {
	// Create a temporary regular file
	tmpFile, err := os.CreateTemp("", "test-regular-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "test content"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test with all policies - regular files should work with all policies
	policies := []SymlinkPolicy{RejectSymlinks, ResolveSymlinks, AllowSymlinks}
	for _, policy := range policies {
		safeInfo, err := CheckSymlink(tmpFile.Name(), policy)
		if err != nil {
			t.Errorf("CheckSymlink failed for regular file with policy %d: %v", policy, err)
			continue
		}

		if safeInfo.IsSymlink {
			t.Errorf("regular file incorrectly identified as symlink")
		}

		if safeInfo.OriginalPath != tmpFile.Name() {
			t.Errorf("original path mismatch: expected %s, got %s", tmpFile.Name(), safeInfo.OriginalPath)
		}

		if safeInfo.ResolvedPath != tmpFile.Name() {
			t.Errorf("resolved path should equal original for regular file: expected %s, got %s", tmpFile.Name(), safeInfo.ResolvedPath)
		}
	}
}

func TestCheckSymlink_SymlinkReject(t *testing.T) {
	// Create a temporary file and symlink
	tmpFile, err := os.CreateTemp("", "test-target-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	symlinkPath := tmpFile.Name() + ".symlink"
	if err := os.Symlink(tmpFile.Name(), symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Test RejectSymlinks policy
	_, err = CheckSymlink(symlinkPath, RejectSymlinks)
	if err == nil {
		t.Errorf("expected error when rejecting symlinks, got nil")
	}
	if !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Errorf("expected 'symlinks are not allowed' error, got: %v", err)
	}
}

func TestCheckSymlink_SymlinkResolve(t *testing.T) {
	// Create a temporary file and symlink
	tmpFile, err := os.CreateTemp("", "test-target-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "target content"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	symlinkPath := tmpFile.Name() + ".symlink"
	if err := os.Symlink(tmpFile.Name(), symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Test ResolveSymlinks policy
	safeInfo, err := CheckSymlink(symlinkPath, ResolveSymlinks)
	if err != nil {
		t.Errorf("unexpected error when resolving symlinks: %v", err)
	}

	if !safeInfo.IsSymlink {
		t.Errorf("symlink not correctly identified")
	}

	if safeInfo.OriginalPath != symlinkPath {
		t.Errorf("original path mismatch: expected %s, got %s", symlinkPath, safeInfo.OriginalPath)
	}

	// Resolved path should be the absolute path of the target
	expectedResolvedPath, _ := filepath.Abs(tmpFile.Name())
	actualResolvedPath, _ := filepath.Abs(safeInfo.ResolvedPath)
	if actualResolvedPath != expectedResolvedPath {
		t.Errorf("resolved path mismatch: expected %s, got %s", expectedResolvedPath, actualResolvedPath)
	}
}

func TestCheckSymlink_SymlinkAllow(t *testing.T) {
	// Create a temporary file and symlink
	tmpFile, err := os.CreateTemp("", "test-target-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	symlinkPath := tmpFile.Name() + ".symlink"
	if err := os.Symlink(tmpFile.Name(), symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Test AllowSymlinks policy
	safeInfo, err := CheckSymlink(symlinkPath, AllowSymlinks)
	if err != nil {
		t.Errorf("unexpected error when allowing symlinks: %v", err)
	}

	if !safeInfo.IsSymlink {
		t.Errorf("symlink not correctly identified")
	}

	if safeInfo.OriginalPath != symlinkPath {
		t.Errorf("original path mismatch: expected %s, got %s", symlinkPath, safeInfo.OriginalPath)
	}

	// With AllowSymlinks, resolved path should equal original path
	if safeInfo.ResolvedPath != symlinkPath {
		t.Errorf("resolved path should equal original with AllowSymlinks: expected %s, got %s", symlinkPath, safeInfo.ResolvedPath)
	}
}

func TestCheckSymlink_BrokenSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a symlink pointing to a non-existent target
	nonExistentTarget := filepath.Join(tmpDir, "nonexistent.txt")
	symlinkPath := filepath.Join(tmpDir, "broken.symlink")

	if err := os.Symlink(nonExistentTarget, symlinkPath); err != nil {
		t.Fatalf("failed to create broken symlink: %v", err)
	}

	// Test ResolveSymlinks with broken symlink
	_, err := CheckSymlink(symlinkPath, ResolveSymlinks)
	if err == nil {
		t.Errorf("expected error for broken symlink with ResolveSymlinks policy")
		return
	}

	// Accept either error message - both are valid depending on where failure occurs
	errMsg := err.Error()
	hasExpectedError := strings.Contains(errMsg, "failed to resolve symlink") ||
		strings.Contains(errMsg, "failed to access symlink target")

	if !hasExpectedError {
		t.Errorf("expected symlink resolution error, got: %v", err)
	} else {
		t.Logf("Got expected error for broken symlink: %v", err)
	}
}

func TestCheckSymlink_NonExistentFile(t *testing.T) {
	nonExistentPath := "/definitely/does/not/exist.txt"

	_, err := CheckSymlink(nonExistentPath, RejectSymlinks)
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to get file info") {
		t.Errorf("expected 'failed to get file info' error, got: %v", err)
	}
}

func TestSafeReadFile_RegularFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	expectedContent := "test file content"
	if _, err := tmpFile.WriteString(expectedContent); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test SafeReadFile
	content, err := SafeReadFile(tmpFile.Name(), RejectSymlinks)
	if err != nil {
		t.Errorf("SafeReadFile failed: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("content mismatch: expected %s, got %s", expectedContent, string(content))
	}
}

func TestSafeReadFile_SymlinkRejected(t *testing.T) {
	// Create a temporary file and symlink
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	symlinkPath := tmpFile.Name() + ".symlink"
	if err := os.Symlink(tmpFile.Name(), symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Test SafeReadFile with RejectSymlinks
	_, err = SafeReadFile(symlinkPath, RejectSymlinks)
	if err == nil {
		t.Errorf("expected error when reading symlink with RejectSymlinks policy")
	}
}

func TestSafeWriteFile_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")

	err := SafeWriteFile(filePath, content, 0600, RejectSymlinks)
	if err != nil {
		t.Errorf("SafeWriteFile failed: %v", err)
	}

	// Verify content was written
	readContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("failed to read written file: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("content mismatch: expected %s, got %s", content, readContent)
	}
}

func TestSafeWriteFile_SymlinkDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real directory and a symlink to it
	realDir := filepath.Join(tmpDir, "real")
	symlinkDir := filepath.Join(tmpDir, "symlink")

	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatalf("failed to create real directory: %v", err)
	}

	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatalf("failed to create directory symlink: %v", err)
	}

	// Try to write to a file in the symlinked directory
	filePath := filepath.Join(symlinkDir, "test.txt")
	content := []byte("test content")

	err := SafeWriteFile(filePath, content, 0600, RejectSymlinks)
	if err == nil {
		t.Errorf("expected error when writing to file in symlinked directory with RejectSymlinks policy")
	}

	// Should work with ResolveSymlinks policy
	err = SafeWriteFile(filePath, content, 0600, ResolveSymlinks)
	if err != nil {
		t.Errorf("SafeWriteFile should work with ResolveSymlinks policy: %v", err)
	}
}

func TestSafeOpenFile_RegularFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test SafeOpenFile
	file, err := SafeOpenFile(tmpFile.Name(), os.O_RDWR, 0600, RejectSymlinks)
	if err != nil {
		t.Errorf("SafeOpenFile failed: %v", err)
	}
	defer file.Close()

	// Write and read to verify it works
	testContent := []byte("test content")
	if _, err := file.Write(testContent); err != nil {
		t.Errorf("failed to write to file: %v", err)
	}

	if _, err := file.Seek(0, 0); err != nil {
		t.Errorf("failed to seek: %v", err)
	}

	readContent := make([]byte, len(testContent))
	if _, err := file.Read(readContent); err != nil {
		t.Errorf("failed to read from file: %v", err)
	}

	if string(readContent) != string(testContent) {
		t.Errorf("content mismatch: expected %s, got %s", testContent, readContent)
	}
}

func TestSafeOpenFile_SymlinkRejected(t *testing.T) {
	// Create a temporary file and symlink
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	symlinkPath := tmpFile.Name() + ".symlink"
	if err := os.Symlink(tmpFile.Name(), symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

	// Test SafeOpenFile with RejectSymlinks
	_, err = SafeOpenFile(symlinkPath, os.O_RDONLY, 0600, RejectSymlinks)
	if err == nil {
		t.Errorf("expected error when opening symlink with RejectSymlinks policy")
	}
}

func TestCheckSymlink_InvalidPolicy(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test with invalid policy value
	_, err = CheckSymlink(tmpFile.Name(), SymlinkPolicy(999))
	if err == nil {
		t.Errorf("expected error for invalid policy")
	}
	if !strings.Contains(err.Error(), "invalid symlink policy") {
		t.Errorf("expected 'invalid symlink policy' error, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkCheckSymlink_RegularFile(b *testing.B) {
	tmpFile, err := os.CreateTemp("", "benchmark-*.txt")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CheckSymlink(tmpFile.Name(), RejectSymlinks)
		if err != nil {
			b.Fatalf("CheckSymlink failed: %v", err)
		}
	}
}

func BenchmarkSafeReadFile_RegularFile(b *testing.B) {
	tmpFile, err := os.CreateTemp("", "benchmark-*.txt")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "benchmark test content"
	if _, err := tmpFile.WriteString(content); err != nil {
		b.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SafeReadFile(tmpFile.Name(), RejectSymlinks)
		if err != nil {
			b.Fatalf("SafeReadFile failed: %v", err)
		}
	}
}
