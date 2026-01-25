package system

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

var (
	log           = logger.Logger()
	OsReleaseFile = "/etc/os-release"
)

func GetHostOsInfo() (map[string]string, error) {
	var hostOsInfo = map[string]string{
		"name":    "",
		"version": "",
		"arch":    "",
	}

	// Get architecture using uname command
	output, err := shell.ExecCmd("uname -m", false, shell.HostPath, nil)
	if err != nil {
		log.Errorf("Failed to get host architecture: %v", err)
		return hostOsInfo, fmt.Errorf("failed to get host architecture: %w", err)
	} else {
		hostOsInfo["arch"] = strings.TrimSpace(output)
	}

	// Read from /etc/os-release if it exists
	if _, err := os.Stat(OsReleaseFile); err == nil {
		file, err := os.Open(OsReleaseFile)
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "NAME=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["name"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				} else if strings.HasPrefix(line, "VERSION_ID=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["version"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				}
			}

			log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
				hostOsInfo["version"] + " " + hostOsInfo["arch"])

			return hostOsInfo, nil
		}
	}

	output, err = shell.ExecCmd("lsb_release -si", false, shell.HostPath, nil)
	if err != nil {
		log.Errorf("Failed to get host OS name: %v", err)
		return hostOsInfo, fmt.Errorf("failed to get host OS name: %w", err)
	} else {
		if output != "" {
			hostOsInfo["name"] = strings.TrimSpace(output)
			output, err = shell.ExecCmd("lsb_release -sr", false, shell.HostPath, nil)
			if err != nil {
				log.Errorf("Failed to get host OS version: %v", err)
				return hostOsInfo, fmt.Errorf("failed to get host OS version: %w", err)
			} else {
				if output != "" {
					hostOsInfo["version"] = strings.TrimSpace(output)
					log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
						hostOsInfo["version"] + " " + hostOsInfo["arch"])
					return hostOsInfo, nil
				}
			}
		}
	}

	log.Errorf("Failed to detect host OS info!")
	return hostOsInfo, fmt.Errorf("failed to detect host OS info")
}

func GetHostOsPkgManager() (string, error) {
	hostOsInfo, err := GetHostOsInfo()
	if err != nil {
		return "", err
	}

	switch hostOsInfo["name"] {
	case "Ubuntu", "Debian", "Debian GNU/Linux", "eLxr":
		return "apt", nil
	case "Fedora", "CentOS", "Red Hat Enterprise Linux":
		return "yum", nil
	case "Microsoft Azure Linux", "Edge Microvisor Toolkit":
		return "tdnf", nil
	default:
		log.Errorf("Unsupported host OS: %s", hostOsInfo["name"])
		return "", fmt.Errorf("unsupported host OS: %s", hostOsInfo["name"])
	}
}

func GetProviderId(os, dist, arch string) string {
	return os + "-" + dist + "-" + arch
}

// OsDistribution contains information about the Linux OS distribution
type OsDistribution struct {
	Name            string   // Distribution name (e.g., "Ubuntu", "Fedora", "Azure Linux")
	Version         string   // Version (e.g., "22.04", "38")
	ID              string   // Distribution ID (e.g., "ubuntu", "fedora")
	IDLike          []string // Related distributions (e.g., ["debian"], ["rhel", "fedora"])
	PackageTypes    []string // Supported package types (e.g., ["deb"], ["rpm"])
	PackageManagers []string // Package managers (e.g., ["apt", "dpkg"], ["tdnf", "rpm"])
}

// DetectOsDistribution detects the underlying Linux OS distribution and its supported package types
// by parsing /etc/os-release and checking available package managers
func DetectOsDistribution() (*OsDistribution, error) {
	osInfo := &OsDistribution{}

	// Parse /etc/os-release file
	if _, err := os.Stat(OsReleaseFile); err == nil {
		file, err := os.Open(OsReleaseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", OsReleaseFile, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"")

			switch key {
			case "NAME":
				osInfo.Name = value
			case "VERSION_ID":
				osInfo.Version = value
			case "ID":
				osInfo.ID = strings.ToLower(value)
			case "ID_LIKE":
				// ID_LIKE can contain multiple space-separated values
				osInfo.IDLike = strings.Fields(value)
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading %s: %w", OsReleaseFile, err)
		}
	} else {
		return nil, fmt.Errorf("file %s not found: %w", OsReleaseFile, err)
	}

	// Determine package types and managers based on distribution
	osInfo.PackageTypes, osInfo.PackageManagers = detectPackageSupport(osInfo.ID, osInfo.IDLike)

	if len(osInfo.PackageTypes) == 0 {
		log.Warnf("Could not determine package type for distribution: %s (ID: %s)", osInfo.Name, osInfo.ID)
	}

	log.Infof("Detected OS distribution: %s %s (ID: %s, Package Types: %v, Package Managers: %v)",
		osInfo.Name, osInfo.Version, osInfo.ID, osInfo.PackageTypes, osInfo.PackageManagers)

	return osInfo, nil
}

// detectPackageSupport determines the package types and managers based on distribution ID
func detectPackageSupport(id string, idLike []string) ([]string, []string) {
	var packageTypes []string
	var packageManagers []string

	// Check the primary distribution ID
	pkgTypes, pkgMgrs := getPackageInfoForID(id)
	if len(pkgTypes) > 0 {
		return pkgTypes, pkgMgrs
	}

	// Check ID_LIKE entries
	for _, likeID := range idLike {
		pkgTypes, pkgMgrs := getPackageInfoForID(likeID)
		if len(pkgTypes) > 0 {
			return pkgTypes, pkgMgrs
		}
	}

	return packageTypes, packageManagers
}

// getPackageInfoForID returns package types and managers for a given distribution ID
func getPackageInfoForID(id string) ([]string, []string) {
	id = strings.ToLower(id)

	switch id {
	case "ubuntu", "debian", "linuxmint", "pop", "elementary", "kali", "raspbian":
		return []string{"deb"}, []string{"apt", "dpkg"}
	case "fedora", "rhel", "centos", "rocky", "almalinux", "scientific", "oracle":
		return []string{"rpm"}, []string{"dnf", "yum", "rpm"}
	case "opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles", "sle":
		return []string{"rpm"}, []string{"zypper", "rpm"}
	case "arch", "manjaro", "endeavouros":
		return []string{"pkg.tar.zst", "pkg.tar.xz"}, []string{"pacman"}
	case "alpine":
		return []string{"apk"}, []string{"apk"}
	case "gentoo", "funtoo":
		return []string{"tbz2"}, []string{"emerge", "portage"}
	case "mariner", "azurelinux":
		// Azure Linux (formerly CBL-Mariner)
		return []string{"rpm"}, []string{"tdnf", "rpm"}
	case "elxr":
		// Wind River eLxr - Debian-based
		return []string{"deb"}, []string{"apt", "dpkg"}
	default:
		// Try to determine from common package manager presence
		return detectFromCommands()
	}
}

// detectFromCommands attempts to detect package support by checking for package manager commands
func detectFromCommands() ([]string, []string) {
	// Check for various package managers - order matters for precedence
	checks := []struct {
		cmd          string
		packageTypes []string
		managers     []string
	}{
		{"apt", []string{"deb"}, []string{"apt"}},
		{"dpkg", []string{"deb"}, []string{"dpkg"}},
		{"dnf", []string{"rpm"}, []string{"dnf"}},
		{"tdnf", []string{"rpm"}, []string{"tdnf"}},
		{"yum", []string{"rpm"}, []string{"yum"}},
		{"rpm", []string{"rpm"}, []string{"rpm"}},
		{"zypper", []string{"rpm"}, []string{"zypper"}},
		{"pacman", []string{"pkg.tar.zst"}, []string{"pacman"}},
		{"apk", []string{"apk"}, []string{"apk"}},
	}

	for _, check := range checks {
		exists, err := shell.IsCommandExist(check.cmd, shell.HostPath)
		if err == nil && exists {
			return check.packageTypes, check.managers
		}
	}

	return []string{}, []string{}
}

// InstallQemuUserStatic installs the qemu-user-static package for cross-architecture support
// It detects the OS distribution and uses the appropriate package manager
func InstallQemuUserStatic() error {
	log.Infof("Checking if qemu-user-static is already installed")

	// Check if qemu-user-static is already installed
	exists, err := shell.IsCommandExist("qemu-aarch64-static", shell.HostPath)
	if err == nil && exists {
		log.Infof("qemu-user-static is already installed")
		return nil
	}

	log.Infof("Detecting OS distribution to install qemu-user-static")
	osInfo, err := DetectOsDistribution()
	if err != nil {
		return fmt.Errorf("failed to detect OS distribution: %w", err)
	}

	var installCmd string
	var packageName string

	// Determine package name and install command based on package type
	if len(osInfo.PackageTypes) == 0 {
		return fmt.Errorf("could not determine package type for distribution: %s", osInfo.Name)
	}

	packageType := osInfo.PackageTypes[0]
	switch packageType {
	case "deb":
		// Debian-based distributions
		packageName = "qemu-user-static"
		// Update package list first
		log.Infof("Updating package list with apt-get update")
		if _, err := shell.ExecCmd("apt-get update", true, shell.HostPath, nil); err != nil {
			log.Warnf("Failed to update package list: %v (continuing anyway)", err)
		}
		installCmd = fmt.Sprintf("apt-get install -y %s", packageName)

	case "rpm":
		// RPM-based distributions
		packageName = "qemu-user-static"

		// Determine which package manager to use
		var pkgManager string
		for _, mgr := range osInfo.PackageManagers {
			if mgr == "tdnf" || mgr == "dnf" || mgr == "yum" {
				pkgManager = mgr
				break
			}
		}

		if pkgManager == "" {
			return fmt.Errorf("no suitable package manager found for RPM-based distribution")
		}

		// For Azure Linux and some distributions, the package might be qemu-user-static-aarch64
		// Try the standard name first
		installCmd = fmt.Sprintf("%s install -y %s", pkgManager, packageName)

	default:
		return fmt.Errorf("unsupported package type: %s for distribution: %s", packageType, osInfo.Name)
	}

	log.Infof("Installing %s using command: %s", packageName, installCmd)
	output, err := shell.ExecCmd(installCmd, true, shell.HostPath, nil)
	if err != nil {
		return fmt.Errorf("failed to install %s: %w\nOutput: %s", packageName, err, output)
	}

	// Verify installation
	exists, err = shell.IsCommandExist("qemu-aarch64-static", shell.HostPath)
	if err != nil || !exists {
		return fmt.Errorf("installation completed but qemu-user-static verification failed")
	}

	log.Infof("Successfully installed %s", packageName)
	return nil
}

func StopGPGComponents(chrootPath string) error {
	log := logger.Logger()

	if !shell.IsBashAvailable(chrootPath) {
		log.Debugf("Bash not available in chroot environment, skipping GPG components stop")
		return nil
	}

	cmdExist, err := shell.IsCommandExist("gpgconf", chrootPath)
	if err != nil {
		return fmt.Errorf("failed to check if gpgconf command exists in chroot environment: %w", err)
	}
	if !cmdExist {
		log.Debugf("gpgconf command not found in chroot environment, skipping GPG components stop")
		return nil
	}
	output, err := shell.ExecCmd("gpgconf --list-components", false, chrootPath, nil)
	if err != nil {
		return fmt.Errorf("failed to list GPG components in chroot environment: %w", err)
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue // Skip empty lines or lines without a colon
		}
		component := strings.TrimSpace(strings.Split(line, ":")[0])
		log.Debugf("Stopping GPG component: %s", component)
		if _, err := shell.ExecCmd("gpgconf --kill "+component, true, chrootPath, nil); err != nil {
			return fmt.Errorf("failed to stop GPG component %s: %w", component, err)
		}
	}

	return nil
}
