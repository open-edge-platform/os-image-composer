package system

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

// SetupHostEnv sets up the host environment for cross-architecture builds
// It handles x86_64 and aarch64 host/target combinations
func SetupHostEnv(os, dist, arch string) error {
	hostOsInfo, err := GetHostOsInfo()
	if err != nil {
		return fmt.Errorf("failed to get host OS info: %w", err)
	}

	hostArch := hostOsInfo["arch"]

	// No setup needed when host and target architectures are the same
	if arch == hostArch {
		log.Infof("Host Architecture %s matches Target Arch %s - no cross-architecture setup needed", hostArch, arch)
		return nil
	}

	log.Infof("Host Architecture: %s, Target Arch: %s - setting up cross-architecture support", hostArch, arch)
	log.Infof("Target OS: %s, Target Dist: %s, Target Arch: %s", os, dist, arch)

	switch hostArch {
	case "x86_64", "amd64":
		return setupFromX86_64(arch)
	case "aarch64", "arm64":
		return setupFromAarch64(arch)
	case "armv7l", "armv7", "arm":
		return setupFromArmv7(arch)
	case "riscv64":
		return setupFromRiscv64(arch)
	case "ppc64le":
		return setupFromPpc64le(arch)
	case "s390x":
		return setupFromS390x(arch)
	case "mips64":
		return setupFromMips64(arch)
	default:
		return fmt.Errorf("unsupported host architecture: %s", hostArch)
	}
}

// setupFromX86_64 handles cross-architecture setup from x86_64 host
func setupFromX86_64(targetArch string) error {
	switch targetArch {
	case "aarch64", "arm64":
		log.Infof("Setting up x86_64 host for aarch64/arm64 target")
		return setupQemuUserStatic("aarch64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up x86_64 host for armv7 target")
		return setupQemuUserStatic("arm")
	case "riscv64":
		log.Infof("Setting up x86_64 host for riscv64 target")
		return setupQemuUserStatic("riscv64")
	case "ppc64le":
		log.Infof("Setting up x86_64 host for ppc64le target")
		return setupQemuUserStatic("ppc64le")
	case "s390x":
		log.Infof("Setting up x86_64 host for s390x target")
		return setupQemuUserStatic("s390x")
	case "mips64":
		log.Infof("Setting up x86_64 host for mips64 target")
		return setupQemuUserStatic("mips64")
	case "i386", "i686":
		log.Infof("Setting up x86_64 host for i386/i686 target")
		// x86_64 can natively run i386/i686 binaries, but may need multilib support
		return setupMultilib()
	default:
		return fmt.Errorf("unsupported target architecture %s for x86_64 host", targetArch)
	}
}

// setupFromAarch64 handles cross-architecture setup from aarch64 host
func setupFromAarch64(targetArch string) error {
	switch targetArch {
	case "x86_64", "amd64":
		log.Infof("Setting up aarch64 host for x86_64/amd64 target")
		return setupQemuUserStatic("x86_64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up aarch64 host for armv7 target")
		// aarch64 can often run armv7 natively with compatibility mode
		return setupArmv7Compat()
	case "riscv64":
		log.Infof("Setting up aarch64 host for riscv64 target")
		return setupQemuUserStatic("riscv64")
	case "ppc64le":
		log.Infof("Setting up aarch64 host for ppc64le target")
		return setupQemuUserStatic("ppc64le")
	case "s390x":
		log.Infof("Setting up aarch64 host for s390x target")
		return setupQemuUserStatic("s390x")
	case "mips64":
		log.Infof("Setting up aarch64 host for mips64 target")
		return setupQemuUserStatic("mips64")
	case "i386", "i686":
		log.Infof("Setting up aarch64 host for i386/i686 target")
		return setupQemuUserStatic("i386")
	default:
		return fmt.Errorf("unsupported target architecture %s for aarch64 host", targetArch)
	}
}

// setupFromArmv7 handles cross-architecture setup from armv7 host
func setupFromArmv7(targetArch string) error {
	switch targetArch {
	case "aarch64", "arm64":
		log.Infof("Setting up armv7 host for aarch64/arm64 target")
		return setupQemuUserStatic("aarch64")
	case "x86_64", "amd64":
		log.Infof("Setting up armv7 host for x86_64 target")
		return setupQemuUserStatic("x86_64")
	case "i386", "i686":
		log.Infof("Setting up armv7 host for i386 target")
		return setupQemuUserStatic("i386")
	case "riscv64":
		log.Infof("Setting up armv7 host for riscv64 target")
		return setupQemuUserStatic("riscv64")
	default:
		return fmt.Errorf("unsupported target architecture %s for armv7 host", targetArch)
	}
}

// setupFromRiscv64 handles cross-architecture setup from riscv64 host
func setupFromRiscv64(targetArch string) error {
	switch targetArch {
	case "x86_64", "amd64":
		log.Infof("Setting up riscv64 host for x86_64 target")
		return setupQemuUserStatic("x86_64")
	case "aarch64", "arm64":
		log.Infof("Setting up riscv64 host for aarch64 target")
		return setupQemuUserStatic("aarch64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up riscv64 host for armv7 target")
		return setupQemuUserStatic("arm")
	default:
		return fmt.Errorf("unsupported target architecture %s for riscv64 host", targetArch)
	}
}

// setupFromPpc64le handles cross-architecture setup from ppc64le host
func setupFromPpc64le(targetArch string) error {
	switch targetArch {
	case "x86_64", "amd64":
		log.Infof("Setting up ppc64le host for x86_64 target")
		return setupQemuUserStatic("x86_64")
	case "aarch64", "arm64":
		log.Infof("Setting up ppc64le host for aarch64 target")
		return setupQemuUserStatic("aarch64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up ppc64le host for armv7 target")
		return setupQemuUserStatic("arm")
	default:
		return fmt.Errorf("unsupported target architecture %s for ppc64le host", targetArch)
	}
}

// setupFromS390x handles cross-architecture setup from s390x host
func setupFromS390x(targetArch string) error {
	switch targetArch {
	case "x86_64", "amd64":
		log.Infof("Setting up s390x host for x86_64 target")
		return setupQemuUserStatic("x86_64")
	case "aarch64", "arm64":
		log.Infof("Setting up s390x host for aarch64 target")
		return setupQemuUserStatic("aarch64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up s390x host for armv7 target")
		return setupQemuUserStatic("arm")
	default:
		return fmt.Errorf("unsupported target architecture %s for s390x host", targetArch)
	}
}

// setupFromMips64 handles cross-architecture setup from mips64 host
func setupFromMips64(targetArch string) error {
	switch targetArch {
	case "x86_64", "amd64":
		log.Infof("Setting up mips64 host for x86_64 target")
		return setupQemuUserStatic("x86_64")
	case "aarch64", "arm64":
		log.Infof("Setting up mips64 host for aarch64 target")
		return setupQemuUserStatic("aarch64")
	case "armv7l", "armv7", "arm":
		log.Infof("Setting up mips64 host for armv7 target")
		return setupQemuUserStatic("arm")
	default:
		return fmt.Errorf("unsupported target architecture %s for mips64 host", targetArch)
	}
}

// setupQemuUserStatic installs QEMU user-mode emulation and sets up binfmt_misc
func setupQemuUserStatic(targetArch string) error {
	// Install qemu-user-static package
	if err := InstallQemuUserStatic(); err != nil {
		return fmt.Errorf("failed to install qemu-user-static: %w", err)
	}

	// Verify the specific QEMU binary exists
	qemuBinary := fmt.Sprintf("qemu-%s-static", targetArch)
	exists, err := shell.IsCommandExist(qemuBinary, shell.HostPath)
	if err != nil || !exists {
		log.Warnf("QEMU binary %s not found, but qemu-user-static is installed", qemuBinary)
		// Continue anyway as the package might have different naming
	} else {
		log.Infof("QEMU binary %s is available", qemuBinary)
	}

	// Check if binfmt_misc is mounted
	if err := ensureBinfmtMisc(); err != nil {
		return fmt.Errorf("failed to setup binfmt_misc: %w", err)
	}

	// Register binfmt for the target architecture if needed
	if err := registerBinfmt(targetArch); err != nil {
		log.Warnf("Failed to register binfmt for %s: %v (may already be registered)", targetArch, err)
		// Don't fail here as it might already be registered
	}

	log.Infof("Cross-architecture support for %s configured successfully", targetArch)
	return nil
}

// setupMultilib enables 32-bit support on 64-bit x86 systems
func setupMultilib() error {
	log.Infof("Checking for multilib (32-bit) support on x86_64 host")

	// Check if i386 architecture is already enabled (Debian/Ubuntu)
	output, err := shell.ExecCmd("dpkg --print-foreign-architectures", false, shell.HostPath, nil)
	if err == nil && output != "" {
		if contains(output, "i386") {
			log.Infof("i386 architecture already enabled")
			return nil
		}

		// Add i386 architecture
		log.Infof("Adding i386 architecture")
		if _, err := shell.ExecCmd("dpkg --add-architecture i386", true, shell.HostPath, nil); err != nil {
			log.Warnf("Failed to add i386 architecture: %v", err)
		}
		if _, err := shell.ExecCmd("apt-get update", true, shell.HostPath, nil); err != nil {
			log.Warnf("Failed to update package lists: %v", err)
		}
	}

	log.Infof("Multilib support configured")
	return nil
}

// setupArmv7Compat enables ARMv7 compatibility on aarch64 systems
func setupArmv7Compat() error {
	log.Infof("Checking for ARMv7 compatibility on aarch64 host")

	// Most aarch64 systems can run armv7 binaries natively
	// Check if armhf architecture is enabled (for Debian/Ubuntu)
	output, err := shell.ExecCmd("dpkg --print-foreign-architectures", false, shell.HostPath, nil)
	if err == nil && output != "" {
		if contains(output, "armhf") {
			log.Infof("armhf architecture already enabled")
			return nil
		}

		// Add armhf architecture
		log.Infof("Adding armhf architecture")
		if _, err := shell.ExecCmd("dpkg --add-architecture armhf", true, shell.HostPath, nil); err != nil {
			log.Warnf("Failed to add armhf architecture: %v", err)
		}
		if _, err := shell.ExecCmd("apt-get update", true, shell.HostPath, nil); err != nil {
			log.Warnf("Failed to update package lists: %v", err)
		}
	}

	log.Infof("ARMv7 compatibility configured")
	return nil
}

// ensureBinfmtMisc ensures that binfmt_misc filesystem is mounted
func ensureBinfmtMisc() error {
	binfmtPath := "/proc/sys/fs/binfmt_misc"

	// Check if binfmt_misc is already mounted
	if _, err := os.Stat(binfmtPath); os.IsNotExist(err) {
		log.Infof("binfmt_misc is not mounted, attempting to mount")

		// Try to mount binfmt_misc
		cmd := fmt.Sprintf("mount binfmt_misc -t binfmt_misc %s", binfmtPath)
		if _, err := shell.ExecCmd(cmd, true, shell.HostPath, nil); err != nil {
			return fmt.Errorf("failed to mount binfmt_misc: %w", err)
		}
		log.Infof("binfmt_misc mounted successfully")
	} else {
		log.Infof("binfmt_misc is already mounted at %s", binfmtPath)
	}

	// Check if register file exists
	registerFile := filepath.Join(binfmtPath, "register")
	if _, err := os.Stat(registerFile); os.IsNotExist(err) {
		return fmt.Errorf("binfmt_misc register file not found at %s", registerFile)
	}

	return nil
}

// registerBinfmt registers a binfmt handler for the target architecture
func registerBinfmt(targetArch string) error {
	binfmtPath := "/proc/sys/fs/binfmt_misc"

	// Check if already registered
	binfmtFile := filepath.Join(binfmtPath, fmt.Sprintf("qemu-%s", targetArch))
	if _, err := os.Stat(binfmtFile); err == nil {
		log.Infof("binfmt handler for %s already registered", targetArch)
		return nil
	}

	// Note: Modern qemu-user-static packages often auto-register via systemd
	// Check if the qemu-binfmt service is running
	output, err := shell.ExecCmd("systemctl is-active systemd-binfmt.service", false, shell.HostPath, nil)
	if err == nil && contains(output, "active") {
		log.Infof("systemd-binfmt.service is active, binfmt should be auto-registered")
		return nil
	}

	// Try to restart the binfmt service to trigger registration
	log.Infof("Attempting to restart systemd-binfmt.service")
	if _, err := shell.ExecCmd("systemctl restart systemd-binfmt.service", true, shell.HostPath, nil); err != nil {
		log.Warnf("Failed to restart systemd-binfmt.service: %v", err)
	}

	// Alternative: try to restart qemu-binfmt service (some distributions use this)
	if _, err := shell.ExecCmd("systemctl restart qemu-binfmt.service", true, shell.HostPath, nil); err != nil {
		log.Debugf("qemu-binfmt.service not available or failed to restart: %v", err)
	}

	log.Infof("binfmt registration completed for %s", targetArch)
	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[0:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr)))
}

// findSubstring checks if substr is in s
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
