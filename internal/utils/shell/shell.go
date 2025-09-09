package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

const HostPath string = ""

var log = logger.Logger()

var commandMap = map[string]string{
	"apt":                "/usr/bin/apt",
	"apt-cache":          "/usr/bin/apt-cache",
	"apt-get":            "/usr/bin/apt-get",
	"basename":           "/usr/bin/basename",
	"bash":               "/usr/bin/bash",
	"blkid":              "/usr/sbin/blkid",
	"bootctl":            "/usr/bin/bootctl",
	"bunzip2":            "/usr/bin/bunzip2",
	"cat":                "/bin/cat",
	"cd":                 "cd", // 'cd' is a shell builtin, not a standalone command
	"chroot":             "/usr/sbin/chroot",
	"chmod":              "/usr/bin/chmod",
	"command":            "command", // 'command' is a shell builtin
	"cp":                 "/bin/cp",
	"createrepo_c":       "/usr/bin/createrepo_c",
	"cryptsetup":         "/usr/sbin/cryptsetup",
	"dd":                 "/usr/bin/dd",
	"df":                 "/usr/bin/df",
	"dirname":            "/usr/bin/dirname",
	"dnf":                "/usr/bin/dnf",
	"dpkg":               "/usr/bin/dpkg",
	"dpkg-scanpackages":  "/usr/bin/dpkg-scanpackages",
	"echo":               "/bin/echo",
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
	"ls":                 "/bin/ls",
	"lsof":               "/usr/bin/lsof",
	"lsb_release":        "/usr/bin/lsb_release",
	"lsblk":              "/usr/bin/lsblk",
	"losetup":            "/usr/sbin/losetup",
	"lvcreate":           "/usr/sbin/lvcreate",
	"mmdebstrap":         "/usr/bin/mmdebstrap",
	"mkdir":              "/bin/mkdir",
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
	"rm":                 "/bin/rm",
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
	"xorriso":            "/usr/bin/xorriso",
	"xz":                 "/usr/bin/xz",
	"yum":                "/usr/bin/yum",
	"zstd":               "/usr/bin/zstd",
	"dracut":             "/usr/bin/dracut",
	"useradd":            "/usr/sbin/useradd",
	"usermod":            "/usr/sbin/usermod",
	"groups":             "/usr/bin/groups",
	"passwd":             "/usr/bin/passwd",
	"mv":                 "/bin/mv",
	"grub-mkstandalone":  "/usr/bin/grub-mkstandalone",
	"sbsign":             "/usr/bin/sbsign",
	"test":               "/bin/test",
	// Add more mappings as needed
}

type Executor interface {
	ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
}

type DefaultExecutor struct{}

var Default Executor = &DefaultExecutor{}

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
	if chrootPath == HostPath {
		cmdStr = "command -v " + cmd
	} else {
		cmdStr = "bash -c 'command -v " + cmd + "'"
	}
	output, err := ExecCmd(cmdStr, false, chrootPath, nil)
	if err != nil {
		output = strings.TrimSpace(output)
		if len(output) == 0 {
			return false, nil
		}
		return false, fmt.Errorf("failed to execute command %s: output %s, err %w", cmdStr, output, err)
	}
	return true, nil
}

func extractSedPattern(command string) (string, error) {
	// This regex handles common sed patterns:
	// - sed -i 's/pattern/replacement/'
	// - sed -i 's/pattern/replacement/g'
	// - sed -i '/pattern/d'
	// - sed -i '/pattern/c\replacement'
	// - sed -i '1,10 d'
	// - sed -i '10 i\text to insert'

	// First try single quotes
	singleRe := regexp.MustCompile(`(?s)sed\s+(?:-[^\s'"]*)?\s+'(.*?)'`)
	matches := singleRe.FindStringSubmatch(command)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Then try double quotes
	doubleRe := regexp.MustCompile(`(?s)sed\s+(?:-[^\s'"]*)?\s+"(.*?)"`)
	matches = doubleRe.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("no quoted string found in sed command")
}

func extractEchoString(command string) (string, error) {
	// Match strings inside echo with single or double quotes
	// Note: Ideally, the pattern should be `(?s)echo\s+(?:-e\s+)?(['"])(.*?)\1'`
	// But the go built-in lib regexp doesn't support this backreferences.

	// First try single quotes
	singleRe := regexp.MustCompile(`(?s)echo\s+(?:-e\s+)?'(.*?)'`)
	matches := singleRe.FindStringSubmatch(command)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Then try double quotes
	doubleRe := regexp.MustCompile(`echo\s+(?:-e\s+)?"(.*?)"`)
	matches = doubleRe.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no quoted string found in echo command")
}

func verifyCmdWithFullPath(cmd string) (string, error) {
	var ignoreStr string
	var err error
	separators := []string{"&&", ";", "|", "||"}

	// If the command is 'sed' or 'echo', we need to ignore the string content
	if strings.HasPrefix(cmd, "sed ") {
		ignoreStr, err = extractSedPattern(cmd)
		if err != nil {
			return "", fmt.Errorf("failed to extract sed pattern: %w", err)
		}
	} else if strings.HasPrefix(cmd, "echo ") {
		ignoreStr, err = extractEchoString(cmd)
		if err != nil {
			return "", fmt.Errorf("failed to extract echo string: %w", err)
		}
	}

	if ignoreStr != "" {
		// Remove the ignore string from the command
		cmd = strings.ReplaceAll(cmd, ignoreStr, "<ignored>")
	}

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
		updatedCmdStr := leftCmdStr + " " + sep + " " + rightCmdStr
		if ignoreStr != "" && ignoreStr != "<ignored>" {
			updatedCmdStr = strings.ReplaceAll(updatedCmdStr, "<ignored>", ignoreStr)
		}
		return updatedCmdStr, nil
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

	updatedCmdStr := strings.Join(fields, " ")
	if ignoreStr != "" && ignoreStr != "<ignored>" {
		updatedCmdStr = strings.ReplaceAll(updatedCmdStr, "<ignored>", ignoreStr)
	}
	return updatedCmdStr, nil
}

// GetFullCmdStr prepares a command string with necessary prefixes
func GetFullCmdStr(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var fullCmdStr string
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
func (d *DefaultExecutor) ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	cmd := exec.Command("bash", "-c", fullCmdStr)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if outputStr != "" {
			return outputStr, fmt.Errorf("failed to exec %s: output %s, err %w", fullCmdStr, outputStr, err)
		} else {
			return outputStr, fmt.Errorf("failed to exec %s: %w", fullCmdStr, err)
		}
	} else {
		if outputStr != "" {
			log.Debugf(outputStr)
		}
		return outputStr, nil
	}
}

// ExecCmdSilent executes a command without logging its output
func (d *DefaultExecutor) ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	cmd := exec.Command("bash", "-c", fullCmdStr)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ExecCmdWithStream executes a command and streams its output
func (d *DefaultExecutor) ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
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

	// Use channels to collect output safely
	outputChan := make(chan string) // Unbuffered channel
	var wg sync.WaitGroup
	wg.Add(3)

	// Collect output immediately in a dedicated goroutine
	var outputStr strings.Builder
	go func() {
		defer wg.Done()
		for output := range outputChan {
			outputStr.WriteString(output)
			outputStr.WriteString("\n") // Add newlines between lines
		}
	}()

	go func() {
		defer wg.Done()
		defer close(outputChan)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			str := scanner.Text()
			if str != "" {
				outputChan <- str
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
		return outputStr.String(), fmt.Errorf("failed to wait for command %s: %w", fullCmdStr, err)
	}

	return outputStr.String(), nil
}

// ExecCmdWithInput executes a command with input string
func (d *DefaultExecutor) ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
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

// Convenience functions for backward compatibility
func ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return Default.ExecCmd(cmdStr, sudo, chrootPath, envVal)
}

func ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return Default.ExecCmdSilent(cmdStr, sudo, chrootPath, envVal)
}

func ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return Default.ExecCmdWithStream(cmdStr, sudo, chrootPath, envVal)
}

func ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	return Default.ExecCmdWithInput(inputStr, cmdStr, sudo, chrootPath, envVal)
}
