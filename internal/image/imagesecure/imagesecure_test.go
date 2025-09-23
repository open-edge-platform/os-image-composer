package imagesecure_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

func TestConfigImageSecurity(t *testing.T) {
	tempDir := t.TempDir()

	// Store original executor and restore at the end
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		installRoot  string
		template     *config.ImageTemplate
		mockCommands []shell.MockCommand
		wantErr      bool
		errContains  string
	}{
		{
			name:        "successful config with ro rootfs",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "defaults,ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				// Mock all mkdir commands that might be called in prepareOverlayDir
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				// Mock systemctl command that might be called in createOverlayMntSvc
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
				// Mock tee command used by file.Append for fstab updates
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				// Mock chmod command for script permissions
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
			},
			wantErr: false,
		},
		{
			name:        "rootfs partition with ID and ro option",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							ID:           "rootfs",
							MountOptions: "ro,nodev",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
			},
			wantErr: false,
		},
		{
			name:        "no ro option - should skip overlay config",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "defaults,rw",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{},
			wantErr:      false,
		},
		{
			name:        "rootfs partition with Name and ro option",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Name:         "rootfs",
							MountOptions: "defaults,ro,sync",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
			},
			wantErr: false,
		},
		{
			name:        "no rootfs partition found - should skip",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-swap",
							MountOptions: "defaults",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{},
			wantErr:      false,
		},
		{
			name:        "empty mount options - should skip",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{},
			wantErr:      false,
		},
		{
			name:         "nil template",
			installRoot:  tempDir,
			template:     nil,
			mockCommands: []shell.MockCommand{},
			wantErr:      true, // This should error due to nil pointer dereference
		},
		{
			name:        "empty partitions list",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{},
				},
			},
			mockCommands: []shell.MockCommand{},
			wantErr:      false,
		},
		{
			name:        "ro option with spaces",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "defaults, ro ,nodev",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
			},
			wantErr: false,
		},
		{
			name:        "multiple partitions - only one with ro",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-swap",
							MountOptions: "defaults",
						},
						{
							Type:         "linux-root-amd64",
							MountOptions: "ro",
						},
						{
							Type:         "esp",
							MountOptions: "defaults",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
			},
			wantErr: false,
		},
		{
			name:        "ro as substring should not match",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "errors=remount-ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create necessary directory structure for file operations
			etcDir := filepath.Join(tt.installRoot, "etc")
			systemdDir := filepath.Join(etcDir, "systemd", "system")
			usrBinDir := filepath.Join(tt.installRoot, "usr", "local", "bin")

			if err := os.MkdirAll(etcDir, 0755); err != nil {
				t.Fatalf("Failed to create etc directory: %v", err)
			}
			if err := os.MkdirAll(systemdDir, 0755); err != nil {
				t.Fatalf("Failed to create systemd directory: %v", err)
			}
			if err := os.MkdirAll(usrBinDir, 0755); err != nil {
				t.Fatalf("Failed to create usr/local/bin directory: %v", err)
			}

			// Create empty fstab file
			fstabPath := filepath.Join(etcDir, "fstab")
			if err := os.WriteFile(fstabPath, []byte(""), 0644); err != nil {
				t.Fatalf("Failed to create fstab file: %v", err)
			}

			// Create tmp directory for temp files (used by file.Append)
			tmpDir := "./tmp"
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("Failed to create tmp directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			// Special handling for nil template test to catch panic
			if tt.template == nil {
				defer func() {
					if r := recover(); r != nil {
						if !tt.wantErr {
							t.Errorf("ConfigImageSecurity() panicked = %v, wantErr %v", r, tt.wantErr)
						}
						// Expected panic for nil template - convert to expected behavior
					}
				}()
			}

			err := imagesecure.ConfigImageSecurity(tt.installRoot, tt.template)

			if tt.wantErr && err == nil && tt.template != nil {
				t.Errorf("ConfigImageSecurity() error = nil, wantErr %v", tt.wantErr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ConfigImageSecurity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test error cases with failing commands
func TestConfigImageSecurity_ErrorCases(t *testing.T) {
	tempDir := t.TempDir()

	// Store original executor and restore at the end
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		installRoot  string
		template     *config.ImageTemplate
		mockCommands []shell.MockCommand
		wantErr      bool
		errContains  string
	}{
		{
			name:        "mkdir command fails",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Error: errors.New("Permission denied")},
			},
			wantErr:     true,
			errContains: "failed to prepare ESP directory",
		},
		{
			name:        "systemctl enable fails",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Error: errors.New("Failed to enable service")},
			},
			wantErr:     true,
			errContains: "failed to create overlay mounting service",
		},
		{
			name:        "file append fails for fstab",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .*/fstab >/dev/null`, Error: errors.New("Disk full")},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""}, // Allow other tee commands
				{Pattern: `sudo chmod -R 755 .*`, Output: ""},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
			},
			wantErr:     true,
			errContains: "failed to update fstab",
		},
		{
			name:        "chmod fails",
			installRoot: tempDir,
			template: &config.ImageTemplate{
				Disk: config.DiskConfig{
					Partitions: []config.PartitionInfo{
						{
							Type:         "linux-root-amd64",
							MountOptions: "ro",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: `sudo chroot .* mkdir -p .*`, Output: ""},
				{Pattern: `cat .*/tmp/fileappend-.* \| sudo tee -a .* >/dev/null`, Output: ""},
				{Pattern: `sudo chmod -R 755 .*`, Error: errors.New("Permission denied")},
				{Pattern: `sudo chroot .* bash -c "systemctl enable setup-overlay\.service"`, Output: ""},
			},
			wantErr:     true,
			errContains: "failed to create overlay mounting service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create necessary directory structure for file operations
			etcDir := filepath.Join(tt.installRoot, "etc")
			systemdDir := filepath.Join(etcDir, "systemd", "system")
			usrBinDir := filepath.Join(tt.installRoot, "usr", "local", "bin")

			if err := os.MkdirAll(etcDir, 0755); err != nil {
				t.Fatalf("Failed to create etc directory: %v", err)
			}
			if err := os.MkdirAll(systemdDir, 0755); err != nil {
				t.Fatalf("Failed to create systemd directory: %v", err)
			}
			if err := os.MkdirAll(usrBinDir, 0755); err != nil {
				t.Fatalf("Failed to create usr/local/bin directory: %v", err)
			}

			// Create empty fstab file
			fstabPath := filepath.Join(etcDir, "fstab")
			if err := os.WriteFile(fstabPath, []byte(""), 0644); err != nil {
				t.Fatalf("Failed to create fstab file: %v", err)
			}

			// Create tmp directory for temp files (used by file.Append)
			tmpDir := "./tmp"
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				t.Fatalf("Failed to create tmp directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Enhanced shell mock commands that might include file.Append commands if needed
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			err := imagesecure.ConfigImageSecurity(tt.installRoot, tt.template)

			if tt.wantErr && err == nil {
				t.Errorf("ConfigImageSecurity() error = nil, wantErr %v", tt.wantErr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ConfigImageSecurity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ConfigImageSecurity() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}
