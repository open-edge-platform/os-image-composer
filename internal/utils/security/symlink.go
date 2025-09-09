// internal/utils/security/symlink.go
package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// SymlinkPolicy defines how to handle symlinks
type SymlinkPolicy int

const (
	// RejectSymlinks - reject any symlinks and return an error
	RejectSymlinks SymlinkPolicy = iota
	// ResolveSymlinks - resolve symlinks and use the target path
	ResolveSymlinks
	// AllowSymlinks - allow symlinks without any checks (unsafe)
	AllowSymlinks
)

// SafeFileInfo contains information about a file after symlink checks
type SafeFileInfo struct {
	OriginalPath string
	ResolvedPath string
	IsSymlink    bool
	FileInfo     os.FileInfo
}

// CheckSymlink validates a file path according to the specified policy
func CheckSymlink(path string, policy SymlinkPolicy) (*SafeFileInfo, error) {
	// CRITICAL: Validate policy FIRST to prevent panic
	if policy < RejectSymlinks || policy > AllowSymlinks {
		return nil, fmt.Errorf("invalid symlink policy: %d", policy)
	}

	// Get file info using Lstat to detect symlinks
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	isSymlink := fileInfo.Mode()&os.ModeSymlink != 0

	result := &SafeFileInfo{
		OriginalPath: path,
		ResolvedPath: path,
		IsSymlink:    isSymlink,
		FileInfo:     fileInfo,
	}

	if !isSymlink {
		// Not a symlink, safe to proceed
		return result, nil
	}

	switch policy {
	case RejectSymlinks:
		return nil, fmt.Errorf("symlinks are not allowed: %s", path)

	case ResolveSymlinks:
		// Resolve the symlink to its target
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve symlink %s: %w", path, err)
		}

		// Get info about the resolved target
		targetInfo, err := os.Stat(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to access symlink target %s: %w", resolvedPath, err)
		}

		result.ResolvedPath = resolvedPath
		result.FileInfo = targetInfo
		return result, nil

	case AllowSymlinks:
		// Allow symlinks without checks (not recommended for security-sensitive operations)
		return result, nil

	default:
		// This should never happen due to validation above, but keep as safety net
		return nil, fmt.Errorf("invalid symlink policy: %d", policy)
	}
}

// SafeReadFile reads a file after performing symlink checks
func SafeReadFile(path string, policy SymlinkPolicy) ([]byte, error) {
	safeInfo, err := CheckSymlink(path, policy)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(safeInfo.ResolvedPath)
}

// SafeWriteFile writes to a file after performing symlink checks on the directory
func SafeWriteFile(path string, data []byte, perm os.FileMode, policy SymlinkPolicy) error {
	// Check if file already exists and is a symlink
	if _, err := os.Lstat(path); err == nil {
		// File exists, check if it's a symlink
		safeInfo, err := CheckSymlink(path, policy)
		if err != nil {
			return fmt.Errorf("existing file symlink check failed: %w", err)
		}
		path = safeInfo.ResolvedPath
	}

	// Check parent directory for symlinks
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		safeInfo, err := CheckSymlink(dir, policy)
		if err != nil {
			return fmt.Errorf("parent directory symlink check failed: %w", err)
		}

		// Reconstruct path with resolved directory
		if safeInfo.ResolvedPath != dir {
			path = filepath.Join(safeInfo.ResolvedPath, filepath.Base(path))
		}
	}

	return os.WriteFile(path, data, perm)
}

// SafeOpenFile opens a file after performing symlink checks
func SafeOpenFile(path string, flag int, perm os.FileMode, policy SymlinkPolicy) (*os.File, error) {
	// If we're creating a file and it doesn't exist, just check the parent directory
	if flag&os.O_CREATE != 0 {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			// File doesn't exist, check parent directory for symlinks
			dir := filepath.Dir(path)
			if dir != "." && dir != "/" && dir != path {
				if _, err := os.Stat(dir); err == nil { // Only check if directory exists
					safeInfo, err := CheckSymlink(dir, policy)
					if err != nil {
						return nil, fmt.Errorf("parent directory symlink check failed: %w", err)
					}

					// Reconstruct path with resolved directory
					if safeInfo.ResolvedPath != dir {
						path = filepath.Join(safeInfo.ResolvedPath, filepath.Base(path))
					}
				}
			}

			// File doesn't exist, safe to create
			return os.OpenFile(path, flag, perm)
		}
	}

	// File exists or we're not creating, perform normal symlink check
	safeInfo, err := CheckSymlink(path, policy)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(safeInfo.ResolvedPath, flag, perm)
}
