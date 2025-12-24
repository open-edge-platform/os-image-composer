package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

func TestHookPostRootfs(t *testing.T) {
	tests := []struct {
		name         string
		installRoot  string
		template     *config.ImageTemplate
		mockCommands []shell.MockCommand
		expectError  bool
		errorMsg     string
		setupFunc    func(*testing.T, string) // Function to set up test files
	}{
		{
			name:        "successful_single_hook",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: "/opt/scripts/post-setup.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*post-setup.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*post-setup.sh.*", Output: "", Error: nil},
				{Pattern: ".*sh.*post-setup.sh.*", Output: "Hook executed successfully\n", Error: nil},
			},
			expectError: false,
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script file
				err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'Hook executed'\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test hook script: %v", err)
				}
			},
		},
		{
			name:        "successful_multiple_hooks",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook1.sh",
							TargetPostRootfs: "/opt/scripts/hook1.sh",
						},
						{
							LocalPostRootfs:  "/tmp/test-hook2.sh",
							TargetPostRootfs: "/usr/local/bin/hook2.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook1.sh.*hook1.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*hook1.sh.*", Output: "", Error: nil},
				{Pattern: ".*sh.*hook1.sh.*", Output: "Hook1 executed\n", Error: nil},
				{Pattern: ".*cp.*test-hook2.sh.*hook2.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*hook2.sh.*", Output: "", Error: nil},
				{Pattern: ".*sh.*hook2.sh.*", Output: "Hook2 executed\n", Error: nil},
			},
			expectError: false,
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script files
				err1 := os.WriteFile("/tmp/test-hook1.sh", []byte("#!/bin/bash\necho 'Hook1 executed'\n"), 0644)
				if err1 != nil {
					t.Fatalf("Failed to create test hook1 script: %v", err1)
				}
				err2 := os.WriteFile("/tmp/test-hook2.sh", []byte("#!/bin/bash\necho 'Hook2 executed'\n"), 0644)
				if err2 != nil {
					t.Fatalf("Failed to create test hook2 script: %v", err2)
				}
			},
		},
		{
			name:        "no_hooks_configured",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{},
				},
			},
			mockCommands: []shell.MockCommand{},
			expectError:  false,
		},
		{
			name:        "copy_command_failure",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: "/opt/scripts/post-setup.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: fmt.Errorf("cp: cannot stat '/tmp/test-hook.sh': No such file or directory")},
			},
			expectError: true,
			errorMsg:    "failed to copy hook script to target rootfs",
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script file
				err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'Hook executed'\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test hook script: %v", err)
				}
			},
		},
		{
			name:        "chmod_command_failure",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: "/opt/scripts/post-setup.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*", Output: "", Error: fmt.Errorf("chmod: cannot access 'post-setup.sh': No such file or directory")},
			},
			expectError: true,
			errorMsg:    "failed to make hook script executable",
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script file
				err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'Hook executed'\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test hook script: %v", err)
				}
			},
		},
		{
			name:        "hook_execution_failure",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: "/opt/scripts/post-setup.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
				{Pattern: ".*sh.*post-setup.sh.*", Output: "", Error: fmt.Errorf("script execution failed with exit code 1")},
			},
			expectError: true,
			errorMsg:    "failed to execute hook script",
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script file
				err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\nexit 1\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test hook script: %v", err)
				}
			},
		},
		{
			name:        "deep_target_directory_structure",
			installRoot: "",
			template: &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: "/opt/custom/very/deep/path/hook.sh",
						},
					},
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*hook.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
				{Pattern: ".*sh.*hook.sh.*", Output: "Deep path hook executed\n", Error: nil},
			},
			expectError: false,
			setupFunc: func(t *testing.T, installRoot string) {
				// Create the hook script file
				err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'Deep path hook executed'\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test hook script: %v", err)
				}
			},
		},
	}

	// Clean up test files before running tests
	defer func() {
		os.Remove("/tmp/test-hook.sh")
		os.Remove("/tmp/test-hook1.sh")
		os.Remove("/tmp/test-hook2.sh")
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the install root
			tempDir := t.TempDir()
			if tt.installRoot == "" {
				tt.installRoot = tempDir
			}

			// Set up shell mocking
			originalExecutor := shell.Default
			defer func() { shell.Default = originalExecutor }()
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			// Set up test files if needed
			if tt.setupFunc != nil {
				tt.setupFunc(t, tt.installRoot)
			}

			// Execute the function under test
			err := HookPostRootfs(tt.installRoot, tt.template)

			// Verify results
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for test '%s', but got none", tt.name)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

func TestHookPostRootfsDirectoryCreation(t *testing.T) {
	// Test that target directories are created correctly
	tempDir := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			HookScripts: []config.HookScriptInfo{
				{
					LocalPostRootfs:  "/tmp/test-hook.sh",
					TargetPostRootfs: "/opt/custom/scripts/hook.sh",
				},
			},
		},
	}

	// Create the test hook script
	err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'test'\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test hook script: %v", err)
	}
	defer os.Remove("/tmp/test-hook.sh")

	// Set up shell mocking
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	
	mockCommands := []shell.MockCommand{
		{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
		{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
		{Pattern: ".*sh.*hook.sh.*", Output: "test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	// Execute the function
	err = HookPostRootfs(tempDir, template)
	if err != nil {
		t.Fatalf("HookPostRootfs failed: %v", err)
	}

	// Verify that the target directory was created
	expectedTargetDir := filepath.Join(tempDir, "/opt/custom/scripts")
	if _, err := os.Stat(expectedTargetDir); os.IsNotExist(err) {
		t.Errorf("expected target directory %s to be created", expectedTargetDir)
	}
}

func TestHookPostRootfsWithNilTemplate(t *testing.T) {
	tempDir := t.TempDir()

	// Set up shell mocking to avoid any real commands
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	shell.Default = shell.NewMockExecutor([]shell.MockCommand{})

	// Test with nil template - this should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HookPostRootfs panicked with nil template: %v", r)
		}
	}()

	err := HookPostRootfs(tempDir, nil)
	if err == nil {
		t.Error("expected error when template is nil, but got none")
	}
}

func TestHookPostRootfsEnvironmentVariable(t *testing.T) {
	tempDir := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			HookScripts: []config.HookScriptInfo{
				{
					LocalPostRootfs:  "/tmp/test-hook.sh",
					TargetPostRootfs: "/opt/scripts/hook.sh",
				},
			},
		},
	}

	// Create the test hook script
	err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho \"TARGET_ROOTFS: $TARGET_ROOTFS\"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test hook script: %v", err)
	}
	defer os.Remove("/tmp/test-hook.sh")

	// Set up shell mocking - we need to capture the environment variable
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	
	// Create a custom mock that verifies the environment variable is set
	mockCommands := []shell.MockCommand{
		{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
		{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
		{Pattern: ".*TARGET_ROOTFS.*sh.*hook.sh.*", Output: fmt.Sprintf("TARGET_ROOTFS: %s\n", tempDir), Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	// Execute the function
	err = HookPostRootfs(tempDir, template)
	if err != nil {
		t.Errorf("HookPostRootfs failed: %v", err)
	}
}

func TestHookPostRootfsEmptyInstallRoot(t *testing.T) {
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			HookScripts: []config.HookScriptInfo{
				{
					LocalPostRootfs:  "/tmp/test-hook.sh",
					TargetPostRootfs: "/opt/scripts/hook.sh",
				},
			},
		},
	}

	// Create the test hook script
	err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'test'\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test hook script: %v", err)
	}
	defer os.Remove("/tmp/test-hook.sh")

	// Set up shell mocking
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	
	mockCommands := []shell.MockCommand{
		{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
		{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
		{Pattern: ".*sh.*hook.sh.*", Output: "test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	// Test with empty install root
	err = HookPostRootfs("", template)
	if err != nil {
		t.Errorf("HookPostRootfs with empty install root failed: %v", err)
	}
}

func TestHookPostRootfsPathNormalization(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name             string
		targetPostRootfs string
		expectedPath     string
	}{
		{
			name:             "absolute_path",
			targetPostRootfs: "/opt/scripts/hook.sh",
			expectedPath:     filepath.Join(tempDir, "/opt/scripts/hook.sh"),
		},
		{
			name:             "path_with_extra_slashes",
			targetPostRootfs: "//opt///scripts//hook.sh",
			expectedPath:     filepath.Join(tempDir, "//opt///scripts//hook.sh"),
		},
		{
			name:             "root_path",
			targetPostRootfs: "/hook.sh",
			expectedPath:     filepath.Join(tempDir, "/hook.sh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &config.ImageTemplate{
				SystemConfig: config.SystemConfig{
					HookScripts: []config.HookScriptInfo{
						{
							LocalPostRootfs:  "/tmp/test-hook.sh",
							TargetPostRootfs: tt.targetPostRootfs,
						},
					},
				},
			}

			// Create the test hook script
			err := os.WriteFile("/tmp/test-hook.sh", []byte("#!/bin/bash\necho 'test'\n"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test hook script: %v", err)
			}
			defer os.Remove("/tmp/test-hook.sh")

			// Set up shell mocking
			originalExecutor := shell.Default
			defer func() { shell.Default = originalExecutor }()
			
			mockCommands := []shell.MockCommand{
				{Pattern: ".*cp.*test-hook.sh.*", Output: "", Error: nil},
				{Pattern: ".*chmod.*\\+x.*", Output: "", Error: nil},
				{Pattern: ".*sh.*hook.sh.*", Output: "test\n", Error: nil},
			}
			shell.Default = shell.NewMockExecutor(mockCommands)

			// Execute the function
			err = HookPostRootfs(tempDir, template)
			if err != nil {
				t.Errorf("HookPostRootfs failed: %v", err)
			}

			// The function should have attempted to create the target directory
			expectedDir := filepath.Join(tempDir, filepath.Dir(tt.targetPostRootfs))
			if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
				t.Errorf("expected directory %s to be created", expectedDir)
			}
		})
	}
}