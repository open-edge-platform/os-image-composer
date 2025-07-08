package shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

var (
	HostPath string = ""
)

var commandMap = map[string]string{
	"apt":                "/usr/bin/apt",
	"apt-cache":          "/usr/bin/apt-cache",
	"apt-get":            "/usr/bin/apt-get",
	"basename":           "/usr/bin/basename",
	"bash":               "/usr/bin/bash",
	"blkid":              "/usr/sbin/blkid",
	"bootctl":            "/usr/bin/bootctl",
	"bunzip2":            "/usr/bin/bunzip2",
	"cat":                "/usr/bin/cat",
	"cd":                 "cd", // 'cd' is a shell builtin, not a standalone command
	"chroot":             "/usr/sbin/chroot",
	"chmod":              "/usr/bin/chmod",
	"cp":                 "/usr/bin/cp",
	"createrepo_c":       "/usr/bin/createrepo_c",
	"cryptsetup":         "/usr/sbin/cryptsetup",
	"dd":                 "/usr/bin/dd",
	"df":                 "/usr/bin/df",
	"dirname":            "/usr/bin/dirname",
	"dnf":                "/usr/bin/dnf",
	"dpkg":               "/usr/bin/dpkg",
	"dpkg-scanpackages":  "/usr/bin/dpkg-scanpackages",
	"echo":               "/usr/bin/echo",
	"e2fsck":             "/usr/sbin/e2fsck",
	"fallocate":          "/usr/bin/fallocate",
	"fdisk":              "/usr/sbin/fdisk",
	"find":               "/usr/bin/find",
	"findmnt":            "/usr/bin/findmnt",
	"flock":              "/usr/bin/flock",
	"fuser":              "/usr/bin/fuser",
	"gpgconf":            "/usr/bin/gpgconf",
	"gunzip":             "/usr/bin/gunzip",
	"grep":               "/usr/bin/grep",
	"grub2-mkconfig":     "/usr/sbin/grub2-mkconfig",
	"gzip":               "/usr/bin/gzip",
	"head":               "/usr/bin/head",
	"ls":                 "/usr/bin/ls",
	"lsof":               "/usr/bin/lsof",
	"lsb_release":        "/usr/bin/lsb_release",
	"lsblk":              "/usr/bin/lsblk",
	"losetup":            "/usr/sbin/losetup",
	"lvcreate":           "/usr/sbin/lvcreate",
	"mkdir":              "/usr/bin/mkdir",
	"mkfs":               "/usr/sbin/mkfs",
	"mkswap":             "/usr/sbin/mkswap",
	"mktemp":             "/usr/bin/mktemp",
	"mount":              "/usr/bin/mount",
	"opkg":               "/usr/bin/opkg",
	"parted":             "/usr/sbin/parted",
	"partx":              "/usr/bin/partx",
	"pvcreate":           "/usr/sbin/pvcreate",
	"qemu-img":           "/usr/bin/qemu-img",
	"qemu-system-x86_64": "/usr/bin/qemu-system-x86_64",
	"rm":                 "/usr/bin/rm",
	"rpm":                "/usr/bin/rpm",
	"run":                "/usr/bin/run",
	"sed":                "/usr/bin/sed",
	"sfdisk":             "/usr/sbin/sfdisk",
	"sgdisk":             "/usr/bin/sgdisk",
	"sha256sum":          "/usr/bin/sha256sum",
	"sh":                 "/bin/sh",
	"sleep":              "/usr/bin/sleep",
	"sudo":               "/usr/bin/sudo",
	"swapon":             "/usr/sbin/swapon",
	"swapoff":            "/usr/sbin/swapoff",
	"sync":               "/usr/bin/sync",
	"tail":               "/usr/bin/tail",
	"tar":                "/usr/bin/tar",
	"tdnf":               "/usr/bin/tdnf",
	"touch":              "/usr/bin/touch",
	"truncate":           "/usr/bin/truncate",
	"tune2fs":            "/usr/sbin/tune2fs",
	"ukify":              "/usr/bin/ukify",
	"umount":             "/usr/bin/umount",
	"uname":              "/usr/bin/uname",
	"uniq":               "/usr/bin/uniq",
	"veritysetup":        "/usr/sbin/veritysetup",
	"vgcreate":           "/usr/sbin/vgcreate",
	"wipefs":             "/usr/sbin/wipefs",
	"xz":                 "/usr/bin/xz",
	"yum":                "/usr/bin/yum",
	"zstd":               "/usr/bin/zstd",
	"dracut":             "/usr/bin/dracut",
	"useradd":            "/usr/sbin/useradd",
	"usermod":            "/usr/sbin/usermod",
	"groups":             "/usr/bin/groups",
	"passwd":             "/usr/bin/passwd",
	"mv":                 "/usr/bin/mv",
	// Add more mappings as needed
}

// GetOSEnvirons returns the system environment variables
func GetOSEnvirons() map[string]string {
	// Convert os.Environ() to a map
	environ := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			environ[parts[0]] = parts[1]
		}
	}
	return environ
}

// GetOSProxyEnvirons retrieves HTTP and HTTPS proxy environment variables
func GetOSProxyEnvirons() map[string]string {
	osEnv := GetOSEnvirons()
	proxyEnv := make(map[string]string)

	// Extract http_proxy and https_proxy variables
	for key, value := range osEnv {
		if strings.Contains(strings.ToLower(key), "http_proxy") ||
			strings.Contains(strings.ToLower(key), "https_proxy") {
			proxyEnv[key] = value
		}
	}

	return proxyEnv
}

// IsCommandExist checks if a command exists in the system or in a chroot environment
func IsCommandExist(cmd string, chrootPath string) (bool, error) {
	var cmdStr string
	log := logger.Logger()
	if chrootPath != HostPath {
		cmdStr = "sudo chroot " + chrootPath + " /bin/sh -c 'command -v " + cmd + "'"
	} else {
		cmdStr = "command -v " + cmd
	}

	output, err := exec.Command("bash", "-c", cmdStr).Output()
	if err != nil {
		log.Errorf("failed to execute command %s: output %s, err %v", cmdStr, output, err)
		return false, fmt.Errorf("failed to execute command %s: %w", cmdStr, err)
	}
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

func verifyCmdWithFullPath(cmd string) (string, error) {
	separators := []string{"&&", ";"}

	sepIdx := -1
	sep := ""
	for _, s := range separators {
		if idx := strings.Index(cmd, s); idx != -1 && (sepIdx == -1 || idx < sepIdx) {
			sepIdx = idx
			sep = s
		}
	}
	if sepIdx != -1 {
		left := strings.TrimSpace(cmd[:sepIdx])
		right := strings.TrimSpace(cmd[sepIdx+len(sep):])
		leftCmdStr, err := verifyCmdWithFullPath(left)
		if err != nil {
			return "", fmt.Errorf("failed to verify command: %w", err)
		}
		rightCmdStr, err := verifyCmdWithFullPath(right)
		if err != nil {
			return "", fmt.Errorf("failed to verify command: %w", err)
		}
		return leftCmdStr + " " + sep + " " + rightCmdStr, nil
	}

	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return cmd, nil
	}
	bin := fields[0]
	fullPath, ok := commandMap[bin]
	if ok {
		fields[0] = fullPath
	} else {
		return "", fmt.Errorf("command %s not found in commandMap", bin)
	}
	return strings.Join(fields, " "), nil
}

// GetFullCmdStr prepares a command string with necessary prefixes
func GetFullCmdStr(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var fullCmdStr string
	log := logger.Logger()
	envValStr := ""
	for _, env := range envVal {
		envValStr += env + " "
	}

	fullPathCmdStr, err := verifyCmdWithFullPath(cmdStr)
	if err != nil {
		return fullPathCmdStr, fmt.Errorf("failed to verify command with full path: %w", err)
	}

	if chrootPath != HostPath {
		if _, err := os.Stat(chrootPath); os.IsNotExist(err) {
			return fullPathCmdStr, fmt.Errorf("chroot path %s does not exist", chrootPath)
		}

		proxyEnv := GetOSProxyEnvirons()

		for key, value := range proxyEnv {
			envValStr += key + "=" + value + " "
		}

		fullCmdStr = "sudo " + envValStr + "chroot " + chrootPath + " " + fullPathCmdStr
		chrootDir := filepath.Base(chrootPath)
		log.Debugf("Chroot " + chrootDir + " Exec: [" + fullPathCmdStr + "]")

	} else {
		if sudo {
			proxyEnv := GetOSProxyEnvirons()

			for key, value := range proxyEnv {
				envValStr += key + "=" + value + " "
			}

			fullCmdStr = "sudo " + envValStr + fullPathCmdStr
			log.Debugf("Exec: [sudo " + fullPathCmdStr + "]")
		} else {
			fullCmdStr = fullPathCmdStr
			log.Debugf("Exec: [" + fullPathCmdStr + "]")
		}
	}

	return fullCmdStr, nil
}

// ExecCmd executes a command and returns its output
func ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	log := logger.Logger()
	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	cmd := exec.Command("bash", "-c", fullCmdStr)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if outputStr != "" {
			log.Errorf("failed to exec %s: output %s, err %v", fullCmdStr, outputStr, err)
		} else {
			log.Errorf("failed to exec %s: %v", fullCmdStr, err)
		}
		return outputStr, fmt.Errorf("failed to exec %s: %w", fullCmdStr, err)
	} else {
		if outputStr != "" {
			log.Debugf(outputStr)
		}
		return outputStr, nil
	}
}

// ExecCmdSilent executes a command without logging its output
func ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	cmd := exec.Command("bash", "-c", fullCmdStr)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ExecCmdWithStream executes a command and streams its output
func ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var outputStr string
	log := logger.Logger()

	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}
	cmd := exec.Command("bash", "-c", fullCmdStr)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe for command %s: %w", fullCmdStr, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe for command %s: %w", fullCmdStr, err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command %s: %w", fullCmdStr, err)
	}

	// Stream output in goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			str := scanner.Text()
			if str != "" {
				outputStr += str
				log.Debugf(str)
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			str := scanner.Text()
			if str != "" {
				log.Debugf(str)
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return outputStr, fmt.Errorf("failed to wait for command %s: %w", fullCmdStr, err)
	}

	return outputStr, nil
}

// ExecCmdWithInput executes a command with input string
func ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	log := logger.Logger()
	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	cmd := exec.Command("bash", "-c", fullCmdStr)
	cmd.Stdin = strings.NewReader(inputStr)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if outputStr != "" {
			log.Infof(outputStr)
		}
		return outputStr, fmt.Errorf("failed to exec %s with input %s: %w", fullCmdStr, inputStr, err)
	} else {
		if outputStr != "" {
			log.Debugf(outputStr)
		}
		return outputStr, nil
	}
}
