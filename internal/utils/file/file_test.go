package file_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/utils/file"
)

func TestReadFromJSON_FileNotExist(t *testing.T) {
	_, err := file.ReadFromJSON("not_exist.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFromJSON_EmptyFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.json")
	err := os.WriteFile(tmp, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	_, err = file.ReadFromJSON(tmp)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadFromJSON_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.json")
	content := `{"foo": "bar", "num": 42}`
	err := os.WriteFile(tmp, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	m, err := file.ReadFromJSON(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["foo"] != "bar" || m["num"] != float64(42) {
		t.Errorf("unexpected map: %v", m)
	}
}

func TestWriteToJSON_And_ReadBack(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.json")
	data := map[string]interface{}{"a": 1, "b": "x"}
	err := file.WriteToJSON(tmp, data, 2)
	if err != nil {
		t.Fatalf("WriteToJSON failed: %v", err)
	}
	read, err := file.ReadFromJSON(tmp)
	if err != nil {
		t.Fatalf("ReadFromJSON failed: %v", err)
	}
	if !reflect.DeepEqual(read, map[string]interface{}{"a": float64(1), "b": "x"}) {
		t.Errorf("unexpected read: %v", read)
	}
}

func TestReadFromYaml_FileNotExist(t *testing.T) {
	_, err := file.ReadFromYaml("not_exist.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFromYaml_EmptyFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.yaml")
	err := os.WriteFile(tmp, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	_, err = file.ReadFromYaml(tmp)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadFromYaml_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	content := "foo: bar\nnum: 42\n"
	err := os.WriteFile(tmp, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	m, err := file.ReadFromYaml(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["foo"] != "bar" || m["num"] != 42 {
		t.Errorf("unexpected map: %v", m)
	}
}

// setupTestFiles creates test files and directories for testing
func setupTestFiles(t *testing.T) (string, func()) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "file-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0700); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create source sub-directory
	srcSubDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(srcSubDir, 0700); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create source sub-directory: %v", err)
	}

	// Create a test file in the source directory
	srcFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(srcFile, []byte("test content"), 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a test file in the source sub-directory
	srcSubFile := filepath.Join(srcSubDir, "subtest.txt")
	if err := os.WriteFile(srcSubFile, []byte("sub test content"), 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create source sub-file: %v", err)
	}

	// Create destination directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := os.MkdirAll(dstDir, 0700); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create destination directory: %v", err)
	}

	// Return clean-up function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestCopyFile(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Test files
	srcFile := filepath.Join(tempDir, "src", "test.txt")
	dstFile := filepath.Join(tempDir, "dst", "test.txt")

	// Test cases
	testCases := []struct {
		name     string
		src      string
		dst      string
		flags    string
		wantErr  bool
		errMsg   string
		validate func(t *testing.T, dstPath string)
	}{
		{
			name:    "Basic Copy",
			src:     srcFile,
			dst:     dstFile,
			flags:   "",
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Verify the file exists
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Errorf("Expected destination file to exist: %s", dstPath)
				}

				// Verify content
				content, err := os.ReadFile(dstPath)
				if err != nil {
					t.Errorf("Failed to read destination file: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("Expected destination file to have content 'test content', got '%s'", string(content))
				}
			},
		},
		{
			name:    "Copy with preserve flag",
			src:     srcFile,
			dst:     dstFile + ".preserve",
			flags:   "-p", // preserve attributes
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Verify the file exists with content
				content, err := os.ReadFile(dstPath)
				if err != nil {
					t.Errorf("Failed to read destination file: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("Expected destination file to have content 'test content', got '%s'", string(content))
				}

				// Get source and destination file stats to compare
				srcInfo, _ := os.Stat(srcFile)
				dstInfo, _ := os.Stat(dstPath)

				// Check permissions are preserved (may vary by OS)
				// We check just the mode bits, not the full FileMode
				if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
					t.Errorf("Expected permissions to be preserved: src=%v, dst=%v",
						srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
				}
			},
		},
		{
			name:    "Source file doesn't exist",
			src:     filepath.Join(tempDir, "nonexistent.txt"),
			dst:     dstFile + ".nonexistent",
			flags:   "",
			wantErr: true,
			errMsg:  "source file does not exist",
		},
		{
			name:    "Create missing destination directory",
			src:     srcFile,
			dst:     filepath.Join(tempDir, "new_subdir", "test.txt"),
			flags:   "",
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Verify the file exists in newly created directory
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Errorf("Expected destination file to exist: %s", dstPath)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the copy
			err := file.CopyFile(tc.src, tc.dst, tc.flags, false)

			// Check error state
			if (err != nil) != tc.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			// For expected errors, validate the error message contains expected string
			if err != nil && tc.errMsg != "" {
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("CopyFile() error = %v, should contain %v", err, tc.errMsg)
				}
				return
			}

			// Run validation if successful
			if err == nil && tc.validate != nil {
				tc.validate(t, tc.dst)
			}
		})
	}
}

func TestCopyDir(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Test directories
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")

	// Test cases
	testCases := []struct {
		name     string
		src      string
		dst      string
		flags    string
		wantErr  bool
		errMsg   string
		validate func(t *testing.T, dstPath string)
	}{
		{
			name:    "Basic Directory Copy",
			src:     srcDir,
			dst:     dstDir,
			flags:   "",
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Check if main file was copied
				mainFile := filepath.Join(dstPath, "test.txt")
				if _, err := os.Stat(mainFile); os.IsNotExist(err) {
					t.Errorf("Expected main file to be copied: %s", mainFile)
				}

				// Check if subdirectory was copied
				subDir := filepath.Join(dstPath, "subdir")
				if _, err := os.Stat(subDir); os.IsNotExist(err) {
					t.Errorf("Expected subdirectory to be copied: %s", subDir)
				}

				// Check if file in subdirectory was copied
				subFile := filepath.Join(subDir, "subtest.txt")
				if _, err := os.Stat(subFile); os.IsNotExist(err) {
					t.Errorf("Expected file in subdirectory to be copied: %s", subFile)
				}

				// Verify content of the main file
				content, err := os.ReadFile(mainFile)
				if err != nil {
					t.Errorf("Failed to read main file: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("Expected main file to have content 'test content', got '%s'", string(content))
				}

				// Verify content of the subdir file
				subContent, err := os.ReadFile(subFile)
				if err != nil {
					t.Errorf("Failed to read subdir file: %v", err)
				}
				if string(subContent) != "sub test content" {
					t.Errorf("Expected subdir file to have content 'sub test content', got '%s'", string(subContent))
				}
			},
		},
		{
			name:    "Copy with preserve flag",
			src:     srcDir,
			dst:     filepath.Join(tempDir, "dst_preserve"),
			flags:   "-p",
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Just check if main file exists with correct content
				mainFile := filepath.Join(dstPath, "test.txt")
				content, err := os.ReadFile(mainFile)
				if err != nil {
					t.Errorf("Failed to read main file: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("Expected main file to have content 'test content', got '%s'", string(content))
				}
			},
		},
		{
			name:    "Source directory doesn't exist",
			src:     filepath.Join(tempDir, "nonexistent"),
			dst:     filepath.Join(tempDir, "dst_nonexistent"),
			flags:   "",
			wantErr: true,
			errMsg:  "source directory does not exist",
		},
		{
			name:    "Create missing destination directory",
			src:     srcDir,
			dst:     filepath.Join(tempDir, "new_dst_subdir"),
			flags:   "",
			wantErr: false,
			validate: func(t *testing.T, dstPath string) {
				// Check if the directory was created and files copied
				mainFile := filepath.Join(dstPath, "test.txt")
				if _, err := os.Stat(mainFile); os.IsNotExist(err) {
					t.Errorf("Expected main file to be copied to new directory: %s", mainFile)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make sure dst exists, especially for first test
			if !filepath.IsAbs(tc.dst) {
				dstDir = filepath.Join(tempDir, tc.dst)
			} else {
				dstDir = tc.dst
			}

			// If not the first test, clean destination
			if tc.name != "Basic Directory Copy" {
				if err := os.RemoveAll(dstDir); err != nil {
					t.Fatalf("Failed to clean destination directory: %v", err)
				}
			}

			// Run the copy
			err := file.CopyDir(tc.src, tc.dst, tc.flags, false)

			// Check error state
			if (err != nil) != tc.wantErr {
				t.Errorf("CopyDir() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			// For expected errors, validate the error message contains expected string
			if err != nil && tc.errMsg != "" {
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("CopyDir() error = %v, should contain %v", err, tc.errMsg)
				}
				return
			}

			// Run validation if successful
			if err == nil && tc.validate != nil {
				tc.validate(t, tc.dst)
			}
		})
	}
}

// TestCopyFilePermissions tests whether permissions are correctly set
func TestCopyFilePermissions(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Create a source file with specific permissions
	srcFile := filepath.Join(tempDir, "permissions.txt")
	if err := os.WriteFile(srcFile, []byte("permission test"), 0600); err != nil {
		t.Fatalf("Failed to create source file with permissions: %v", err)
	}

	// Ensure the permissions are set correctly
	if err := os.Chmod(srcFile, 0600); err != nil {
		t.Fatalf("Failed to set permissions on source file: %v", err)
	}

	// Destination file
	dstFile := filepath.Join(tempDir, "dst", "permissions.txt")

	// Test copying with permissions preserved
	err := file.CopyFile(srcFile, dstFile, "-p", false)
	if err != nil {
		t.Fatalf("Failed to copy file with preserved permissions: %v", err)
	}

	// Check if permissions are preserved
	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("Failed to stat source file: %v", err)
	}

	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("Failed to stat destination file: %v", err)
	}

	// Compare permission bits
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("Permissions not preserved: source=%v, dest=%v",
			srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}

// TestCopyFileConcurrent tests copying files concurrently
func TestCopyFileConcurrent(t *testing.T) {
	tempDir, cleanup := setupTestFiles(t)
	defer cleanup()

	// Create multiple source files
	numFiles := 10
	for i := 0; i < numFiles; i++ {
		srcFile := filepath.Join(tempDir, "src", fmt.Sprintf("test%d.txt", i))
		if err := os.WriteFile(srcFile, []byte(fmt.Sprintf("content %d", i)), 0644); err != nil {
			t.Fatalf("Failed to create source file %d: %v", i, err)
		}
	}

	// Copy files concurrently
	var wg sync.WaitGroup
	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			srcFile := filepath.Join(tempDir, "src", fmt.Sprintf("test%d.txt", i))
			dstFile := filepath.Join(tempDir, "dst", fmt.Sprintf("test%d.txt", i))
			if err := file.CopyFile(srcFile, dstFile, "", false); err != nil {
				t.Errorf("Failed to copy file %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all files were copied correctly
	for i := 0; i < numFiles; i++ {
		dstFile := filepath.Join(tempDir, "dst", fmt.Sprintf("test%d.txt", i))
		content, err := os.ReadFile(dstFile)
		if err != nil {
			t.Errorf("Failed to read destination file %d: %v", i, err)
			continue
		}
		expected := fmt.Sprintf("content %d", i)
		if string(content) != expected {
			t.Errorf("File %d: expected content '%s', got '%s'", i, expected, string(content))
		}
	}
}
