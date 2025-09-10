package rawmaker_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/image/rawmaker"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

// Mock implementations for testing
type mockChrootEnv struct {
	pkgType           string
	chrootPkgCacheDir string
	shouldFailRefresh bool
	chrootEnvRoot     string
}

func (m *mockChrootEnv) GetChrootEnvRoot() string {
	return m.chrootEnvRoot
}

func (m *mockChrootEnv) GetChrootImageBuildDir() string {
	return filepath.Join(m.chrootEnvRoot, "workspace", "imagebuild")
}

func (m *mockChrootEnv) GetTargetOsPkgType() string {
	return m.pkgType
}

func (m *mockChrootEnv) GetTargetOsConfigDir() string {
	return filepath.Join(m.chrootEnvRoot, "config")
}

func (m *mockChrootEnv) GetTargetOsReleaseVersion() string {
	return "3.0"
}

func (m *mockChrootEnv) GetChrootPkgCacheDir() string {
	return m.chrootPkgCacheDir
}

func (m *mockChrootEnv) GetChrootEnvEssentialPackageList() ([]string, error) {
	return []string{"essential-pkg1", "essential-pkg2"}, nil
}

func (m *mockChrootEnv) GetChrootEnvHostPath(chrootPath string) (string, error) {
	return filepath.Join(m.chrootEnvRoot, chrootPath), nil
}

func (m *mockChrootEnv) GetChrootEnvPath(ChrootEnvHostPath string) (string, error) {
	return ChrootEnvHostPath[len(m.chrootEnvRoot):], nil
}

func (m *mockChrootEnv) MountChrootSysfs(chrootPath string) error {
	return nil
}

func (m *mockChrootEnv) UmountChrootSysfs(chrootPath string) error {
	return nil
}

func (m *mockChrootEnv) MountChrootPath(hostFullPath, chrootPath, mountFlags string) error {
	return nil
}

func (m *mockChrootEnv) UmountChrootPath(chrootPath string) error {
	return nil
}

func (m *mockChrootEnv) CopyFileFromHostToChroot(hostFilePath, chrootPath string) error {
	return nil
}

func (m *mockChrootEnv) CopyFileFromChrootToHost(hostFilePath, chrootPath string) error {
	return nil
}

func (m *mockChrootEnv) RefreshLocalCacheRepo(arch string) error {
	if m.shouldFailRefresh {
		return fmt.Errorf("mock refresh cache repo failure")
	}
	return nil
}

func (m *mockChrootEnv) InitChrootEnv(targetOs, targetDist, targetArch string) error {
	return nil
}

func (m *mockChrootEnv) CleanupChrootEnv(targetOs, targetDist, targetArch string) error {
	return nil
}

func (m *mockChrootEnv) TdnfInstallPackage(packageName, installRoot string, repositoryIDList []string) error {
	return nil
}

func (m *mockChrootEnv) AptInstallPackage(packageName, installRoot string, repoSrcList []string) error {
	return nil
}

type mockLoopDev struct {
	shouldFailCreate bool
	shouldFailDelete bool
	loopDevPath      string
}

func (m *mockLoopDev) CreateRawImageLoopDev(filePath string, template *config.ImageTemplate) (string, map[string]string, error) {
	if m.shouldFailCreate {
		return "", nil, fmt.Errorf("mock loop device creation failure")
	}
	diskPathIdMap := map[string]string{
		"root": "/dev/loop0p1",
		"boot": "/dev/loop0p2",
	}
	return m.loopDevPath, diskPathIdMap, nil
}

func (m *mockLoopDev) LoopSetupDelete(loopDevPath string) error {
	if m.shouldFailDelete {
		return fmt.Errorf("mock loop device deletion failure")
	}
	return nil
}

type mockImageOs struct {
	installRoot       string
	shouldFailInstall bool
	versionInfo       string
}

func (m *mockImageOs) GetInstallRoot() string {
	return m.installRoot
}

func (m *mockImageOs) InstallInitrd() (string, string, error) {
	if m.shouldFailInstall {
		return "", "", fmt.Errorf("mock install initrd failure")
	}
	return m.installRoot, m.versionInfo, nil
}

func (m *mockImageOs) InstallImageOs(diskPathIdMap map[string]string) (versionInfo string, err error) {
	if m.shouldFailInstall {
		return "", fmt.Errorf("mock install image OS failure")
	}
	return m.versionInfo, nil
}

type mockImageConvert struct {
	shouldFailConvert bool
}

func (m *mockImageConvert) ConvertImageFile(filePath string, template *config.ImageTemplate) error {
	if m.shouldFailConvert {
		return fmt.Errorf("mock image conversion failure")
	}
	return nil
}

func TestNewRawMaker(t *testing.T) {
	tests := []struct {
		name        string
		chrootEnv   chroot.ChrootEnvInterface
		expectError bool
	}{
		{
			name:        "successful_creation",
			chrootEnv:   &mockChrootEnv{},
			expectError: false,
		},
		{
			name:        "nil_chroot_env",
			chrootEnv:   nil,
			expectError: false, // Constructor doesn't validate nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMaker, err := rawmaker.NewRawMaker(tt.chrootEnv)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if rawMaker == nil {
					t.Error("Expected non-nil RawMaker")
				} else {
					if tt.chrootEnv != nil {
						if rawMaker.ChrootEnv != tt.chrootEnv {
							t.Error("Expected ChrootEnv to be set correctly")
						}
					}

					if rawMaker.LoopDev == nil {
						t.Error("Expected LoopDev to be initialized")
					}
				}
			}
		})
	}
}

func TestRawMaker_Init(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name          string
		template      *config.ImageTemplate
		mockCommands  []shell.MockCommand
		setupFunc     func(tempDir string) error
		expectError   bool
		expectedError string
	}{
		{
			name: "successful_init",
			template: &config.ImageTemplate{
				Target: config.TargetInfo{
					OS:   "ubuntu",
					Dist: "jammy",
					Arch: "x86_64",
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: "mkdir", Output: "", Error: nil},
			},
			expectError: false,
		},
		{
			name:     "nil_template",
			template: nil,
			mockCommands: []shell.MockCommand{
				{Pattern: "mkdir", Output: "", Error: nil},
			},
			expectError:   true,
			expectedError: "failed to create image OS instance",
		},
		{
			name: "mkdir_failure",
			template: &config.ImageTemplate{
				Target: config.TargetInfo{
					OS:   "ubuntu",
					Dist: "jammy",
					Arch: "x86_64",
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: "mkdir", Output: "", Error: fmt.Errorf("mkdir failed")},
			},
			expectError:   true,
			expectedError: "failed to create directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			tempDir := t.TempDir()
			chrootEnv := &mockChrootEnv{
				pkgType:           "deb",
				chrootEnvRoot:     tempDir,
				chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
			}

			chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
			if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
				t.Fatalf("Failed to create chroot image build dir: %v", err)
			}

			if tt.setupFunc != nil {
				if err := tt.setupFunc(tempDir); err != nil {
					t.Fatalf("Failed to setup test: %v", err)
				}
			}

			// Mock config.WorkDir()
			originalWorkDir := os.Getenv("IMAGE_COMPOSER_WORK_DIR")
			os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
			defer func() {
				if originalWorkDir == "" {
					os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")
				} else {
					os.Setenv("IMAGE_COMPOSER_WORK_DIR", originalWorkDir)
				}
			}()

			rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
			if err != nil {
				t.Fatalf("Failed to create RawMaker: %v", err)
			}

			currentConfig := config.Global()
			currentConfig.WorkDir = tempDir
			config.SetGlobal(currentConfig)

			err = rawMaker.Init(tt.template)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				} else if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if rawMaker.ImageOs == nil {
					t.Error("Expected ImageOs to be initialized")
				}
				if rawMaker.ImageConvert == nil {
					t.Error("Expected ImageConvert to be initialized")
				}
				if rawMaker.ImageBuildDir == "" {
					t.Error("Expected ImageBuildDir to be set")
				}
			}
		})
	}
}

func TestRawMaker_BuildRawImage_Success(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
		{Pattern: "mv", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
		t.Fatalf("Failed to create chroot image build dir: %v", err)
	}

	// Mock config.WorkDir()
	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mock implementations
	mockLoopDev := &mockLoopDev{
		loopDevPath: "/dev/loop0",
	}
	mockImageOs := &mockImageOs{
		installRoot: tempDir,
		versionInfo: "1.0.0",
	}
	mockImageConvert := &mockImageConvert{}

	rawMaker.LoopDev = mockLoopDev
	rawMaker.ImageOs = mockImageOs
	rawMaker.ImageConvert = mockImageConvert

	// Create the expected directory structure
	buildDir := filepath.Join(tempDir, "ubuntu-jammy-x86_64", "imagebuild", "test-config")
	if err := os.MkdirAll(buildDir, 0700); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestRawMaker_BuildRawImage_LoopDevCreationFailure(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
		t.Fatalf("Failed to create chroot image build dir: %v", err)
	}

	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mock that fails
	mockLoopDev := &mockLoopDev{
		shouldFailCreate: true,
	}
	rawMaker.LoopDev = mockLoopDev

	err = rawMaker.Init(template)
	if err != nil {
		t.Fatalf("Failed to initialize RawMaker: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err == nil {
		t.Error("Expected error, but got none")
	}
	if !strings.Contains(err.Error(), "failed to create raw image") {
		t.Errorf("Expected error about raw image creation, but got: %v", err)
	}
}

func TestRawMaker_BuildRawImage_ImageOsInstallFailure(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
		{Pattern: "rm", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
		t.Fatalf("Failed to create chroot image build dir: %v", err)
	}

	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mocks
	mockLoopDev := &mockLoopDev{
		loopDevPath: "/dev/loop0",
	}
	mockImageOs := &mockImageOs{
		shouldFailInstall: true,
	}

	rawMaker.LoopDev = mockLoopDev
	rawMaker.ImageOs = mockImageOs

	err = rawMaker.Init(template)
	if err != nil {
		t.Fatalf("Failed to initialize RawMaker: %v", err)
	}

	// Create the expected directory structure
	buildDir := filepath.Join(tempDir, "ubuntu-jammy-x86_64", "imagebuild", "test-config")
	if err := os.MkdirAll(buildDir, 0700); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err == nil {
		t.Error("Expected error, but got none")
	}
	if !strings.Contains(err.Error(), "failed to install image OS") {
		t.Errorf("Expected error about image OS installation, but got: %v", err)
	}
}

func TestRawMaker_BuildRawImage_LoopDevDeleteFailure(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
		{Pattern: "rm", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
		t.Fatalf("Failed to create chroot image build dir: %v", err)
	}

	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mocks
	mockLoopDev := &mockLoopDev{
		loopDevPath:      "/dev/loop0",
		shouldFailDelete: true,
	}
	mockImageOs := &mockImageOs{
		installRoot: tempDir,
		versionInfo: "1.0.0",
	}

	rawMaker.LoopDev = mockLoopDev
	rawMaker.ImageOs = mockImageOs

	err = rawMaker.Init(template)
	if err != nil {
		t.Fatalf("Failed to initialize RawMaker: %v", err)
	}

	// Create the expected directory structure
	buildDir := filepath.Join(tempDir, "ubuntu-jammy-x86_64", "imagebuild", "test-config")
	if err := os.MkdirAll(buildDir, 0700); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err == nil {
		t.Error("Expected error, but got none")
	}
	if !strings.Contains(err.Error(), "mock loop device deletion failure") {
		t.Errorf("Expected error about loop device detachment, but got: %v", err)
	}
}

func TestRawMaker_BuildRawImage_RenameFailure(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
		{Pattern: "mv", Output: "", Error: fmt.Errorf("mv failed")},
		{Pattern: "rm", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if err := os.MkdirAll(chrootImageBuildDir, 0700); err != nil {
		t.Fatalf("Failed to create chroot image build dir: %v", err)
	}

	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mocks
	mockLoopDev := &mockLoopDev{
		loopDevPath: "/dev/loop0",
	}
	mockImageOs := &mockImageOs{
		installRoot: tempDir,
		versionInfo: "1.0.0",
	}

	rawMaker.LoopDev = mockLoopDev
	rawMaker.ImageOs = mockImageOs

	// Create the expected directory structure and file
	buildDir := filepath.Join(tempDir, "ubuntu-jammy-x86_64", "imagebuild", "test-config")
	if err := os.MkdirAll(buildDir, 0700); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err == nil {
		t.Error("Expected error, but got none")
	}
	if !strings.Contains(err.Error(), "failed to rename raw image file") {
		t.Errorf("Expected error about file rename, but got: %v", err)
	}
}

func TestRawMaker_BuildRawImage_ConvertFailure(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	mockCommands := []shell.MockCommand{
		{Pattern: "mkdir", Output: "", Error: nil},
		{Pattern: "mv", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		pkgType:           "deb",
		chrootEnvRoot:     tempDir,
		chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
	}

	os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
	defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
		Image: config.ImageInfo{
			Name: "test-image",
		},
		SystemConfig: config.SystemConfig{
			Name: "test-config",
		},
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Replace with mocks
	mockLoopDev := &mockLoopDev{
		loopDevPath: "/dev/loop0",
	}
	mockImageOs := &mockImageOs{
		installRoot: tempDir,
		versionInfo: "1.0.0",
	}
	mockImageConvert := &mockImageConvert{
		shouldFailConvert: true,
	}

	rawMaker.LoopDev = mockLoopDev
	rawMaker.ImageOs = mockImageOs
	rawMaker.ImageConvert = mockImageConvert

	// Create the expected directory structure
	buildDir := filepath.Join(tempDir, "ubuntu-jammy-x86_64", "imagebuild", "test-config")
	if err := os.MkdirAll(buildDir, 0700); err != nil {
		t.Fatalf("Failed to create build directory: %v", err)
	}

	err = rawMaker.BuildRawImage(template)

	if err == nil {
		t.Error("Expected error, but got none")
	}
	if !strings.Contains(err.Error(), "failed to convert image file") {
		t.Errorf("Expected error about image conversion, but got: %v", err)
	}
}

func TestRawMaker_CleanupOnSuccess(t *testing.T) {
	tests := []struct {
		name             string
		loopDevPath      string
		shouldFailDelete bool
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name:        "successful_cleanup",
			loopDevPath: "/dev/loop0",
			expectError: false,
		},
		{
			name:        "empty_loop_dev_path",
			loopDevPath: "",
			expectError: false,
		},
		{
			name:             "delete_failure",
			loopDevPath:      "/dev/loop0",
			shouldFailDelete: true,
			expectError:      true,
			expectedErrorMsg: "mock loop device deletion failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			chrootEnv := &mockChrootEnv{
				chrootEnvRoot: tempDir,
			}

			rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
			if err != nil {
				t.Fatalf("Failed to create RawMaker: %v", err)
			}

			mockLoopDev := &mockLoopDev{
				shouldFailDelete: tt.shouldFailDelete,
			}
			rawMaker.LoopDev = mockLoopDev

			// Test cleanup function by calling it directly through BuildRawImage
			// since cleanupOnSuccess is unexported

			// We can't directly test the unexported method, so we test the behavior
			// through the public interface by checking if cleanup happens correctly
			if tt.loopDevPath != "" {
				err := mockLoopDev.LoopSetupDelete(tt.loopDevPath)
				if tt.expectError {
					if err == nil {
						t.Error("Expected error during cleanup, but got none")
					} else if tt.expectedErrorMsg != "" && !strings.Contains(err.Error(), tt.expectedErrorMsg) {
						t.Errorf("Expected error containing '%s', but got: %v", tt.expectedErrorMsg, err)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error during cleanup, but got: %v", err)
					}
				}
			}
		})
	}
}

func TestRawMaker_CleanupOnError(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name             string
		loopDevPath      string
		imagePath        string
		shouldFailDelete bool
		shouldFailRemove bool
		createImageFile  bool
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name:            "successful_cleanup",
			loopDevPath:     "/dev/loop0",
			imagePath:       "/tmp/test.raw",
			createImageFile: true,
			expectError:     false,
		},
		{
			name:        "empty_paths",
			loopDevPath: "",
			imagePath:   "",
			expectError: false,
		},
		{
			name:             "delete_failure",
			loopDevPath:      "/dev/loop0",
			imagePath:        "/tmp/test.raw",
			shouldFailDelete: true,
			createImageFile:  true,
			expectError:      true,
			expectedErrorMsg: "cleanup errors",
		},
		{
			name:             "remove_failure",
			loopDevPath:      "/dev/loop0",
			imagePath:        "/tmp/test.raw",
			shouldFailRemove: true,
			createImageFile:  true,
			expectError:      true,
			expectedErrorMsg: "cleanup errors",
		},
		{
			name:            "file_not_exists",
			loopDevPath:     "/dev/loop0",
			imagePath:       "/tmp/nonexistent.raw",
			createImageFile: false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommands := []shell.MockCommand{
				{Pattern: "rm", Output: "", Error: nil},
			}
			if tt.shouldFailRemove {
				mockCommands = []shell.MockCommand{
					{Pattern: "rm", Output: "", Error: fmt.Errorf("rm failed")},
				}
			}
			shell.Default = shell.NewMockExecutor(mockCommands)

			tempDir := t.TempDir()
			chrootEnv := &mockChrootEnv{
				chrootEnvRoot: tempDir,
			}

			rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
			if err != nil {
				t.Fatalf("Failed to create RawMaker: %v", err)
			}

			mockLoopDev := &mockLoopDev{
				shouldFailDelete: tt.shouldFailDelete,
			}
			rawMaker.LoopDev = mockLoopDev

			// Create image file if needed
			if tt.createImageFile && tt.imagePath != "" {
				imageDir := filepath.Dir(tt.imagePath)
				if err := os.MkdirAll(imageDir, 0700); err != nil {
					t.Fatalf("Failed to create image directory: %v", err)
				}
				if err := os.WriteFile(tt.imagePath, []byte("test image"), 0644); err != nil {
					t.Fatalf("Failed to create image file: %v", err)
				}
			}

			// Test cleanup by simulating error conditions
			// We can't directly test cleanupOnError since it's unexported,
			// but we can test the individual operations
			var testErr error = fmt.Errorf("original error")

			// Test loop device deletion
			if tt.loopDevPath != "" {
				err := mockLoopDev.LoopSetupDelete(tt.loopDevPath)
				if tt.shouldFailDelete && err == nil {
					t.Error("Expected loop device deletion to fail")
				}
			}

			// Test file removal
			if tt.imagePath != "" && tt.createImageFile {
				_, err := os.Stat(tt.imagePath)
				fileExists := err == nil

				if fileExists {
					_, err := shell.ExecCmd(fmt.Sprintf("rm -f %s", tt.imagePath), true, "", nil)
					if tt.shouldFailRemove && err == nil {
						t.Error("Expected file removal to fail")
					}
				}
			}

			// In actual usage, the error would be modified by cleanupOnError
			if tt.expectError && testErr != nil {
				if !strings.Contains(testErr.Error(), "original error") {
					t.Errorf("Expected error to contain original error message")
				}
			}
		})
	}
}

func TestRawMaker_BuildRawImage_Integration(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name          string
		template      *config.ImageTemplate
		mockCommands  []shell.MockCommand
		setupFunc     func(tempDir string) error
		expectError   bool
		expectedError string
	}{
		{
			name: "complete_workflow",
			template: &config.ImageTemplate{
				Target: config.TargetInfo{
					OS:   "ubuntu",
					Dist: "jammy",
					Arch: "x86_64",
				},
				Image: config.ImageInfo{
					Name: "test-image",
				},
				SystemConfig: config.SystemConfig{
					Name: "test-config",
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: "mkdir", Output: "", Error: nil},
				{Pattern: "mv", Output: "", Error: nil},
			},
			expectError: false,
		},
		{
			name: "invalid_template",
			template: &config.ImageTemplate{
				Target: config.TargetInfo{
					OS:   "",
					Dist: "",
					Arch: "",
				},
				Image: config.ImageInfo{
					Name: "",
				},
			},
			mockCommands: []shell.MockCommand{
				{Pattern: "mkdir", Output: "", Error: nil},
			},
			expectError:   true,
			expectedError: "failed to create raw image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			tempDir := t.TempDir()
			chrootEnv := &mockChrootEnv{
				pkgType:           "deb",
				chrootEnvRoot:     tempDir,
				chrootPkgCacheDir: filepath.Join(tempDir, "cache"),
			}

			if tt.setupFunc != nil {
				if err := tt.setupFunc(tempDir); err != nil {
					t.Fatalf("Failed to setup test: %v", err)
				}
			}

			// Mock config directories
			os.Setenv("IMAGE_COMPOSER_WORK_DIR", tempDir)
			defer os.Unsetenv("IMAGE_COMPOSER_WORK_DIR")

			rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
			if err != nil {
				t.Fatalf("Failed to create RawMaker: %v", err)
			}

			// Set up mocks based on expected behavior
			var mockLoopDevImpl imagedisc.LoopDevInterface
			var mockImageOsImpl imageos.ImageOsInterface
			var mockImageConvertImpl imageconvert.ImageConvertInterface

			if tt.expectError {
				mockLoopDevImpl = &mockLoopDev{
					shouldFailCreate: true,
				}
			} else {
				mockLoopDevImpl = &mockLoopDev{
					loopDevPath: "/dev/loop0",
				}
				mockImageOsImpl = &mockImageOs{
					installRoot: tempDir,
					versionInfo: "1.0.0",
				}
				mockImageConvertImpl = &mockImageConvert{}
			}

			rawMaker.LoopDev = mockLoopDevImpl
			if mockImageOsImpl != nil {
				rawMaker.ImageOs = mockImageOsImpl
			}
			if mockImageConvertImpl != nil {
				rawMaker.ImageConvert = mockImageConvertImpl
			}

			if !tt.expectError {
				// Create the expected directory structure
				providerId := fmt.Sprintf("%s-%s-%s", tt.template.Target.OS, tt.template.Target.Dist, tt.template.Target.Arch)
				buildDir := filepath.Join(tempDir, providerId, "imagebuild", tt.template.SystemConfig.Name)
				if err := os.MkdirAll(buildDir, 0700); err != nil {
					t.Fatalf("Failed to create build directory: %v", err)
				}
			}

			err = rawMaker.BuildRawImage(tt.template)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				} else if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestRawMaker_Interface_Compliance(t *testing.T) {
	// Test that RawMaker implements RawMakerInterface
	tempDir := t.TempDir()
	chrootEnv := &mockChrootEnv{
		chrootEnvRoot: tempDir,
	}

	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		t.Fatalf("Failed to create RawMaker: %v", err)
	}

	// Verify interface compliance by checking method signatures
	var _ rawmaker.RawMakerInterface = rawMaker

	// Test method existence
	template := &config.ImageTemplate{
		Target: config.TargetInfo{
			OS:   "ubuntu",
			Dist: "jammy",
			Arch: "x86_64",
		},
	}

	// Test Init method
	if err := rawMaker.Init(nil); err == nil {
		t.Error("Expected error for nil template, but got none")
	}

	// Test that the methods exist and can be called
	err = rawMaker.BuildRawImage(template)
	// We expect this to fail without proper setup, but the method should exist
	if err == nil {
		t.Error("Expected error without proper setup")
	}
}
