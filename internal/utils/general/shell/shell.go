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

	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
)

var (
	HostPath string = ""
)

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

// getShell returns the preferred shell, falling back to /bin/sh if bash is not available
func getShell() string {
	shells := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"}
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}
	return "/bin/sh" // fallback
}

// IsCommandExist checks if a command exists in the system or in a chroot environment
func IsCommandExist(cmd string, chrootPath string) bool {
	var cmdStr string
	if chrootPath != HostPath {
		cmdStr = "sudo chroot " + chrootPath + " command -v " + cmd
	} else {
		cmdStr = "command -v " + cmd
	}

	shell := getShell()
	output, _ := exec.Command(shell, "-c", cmdStr).Output()
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return false
	} else {
		return true
	}
}

// GetFullCmdStr prepares a command string with necessary prefixes
func GetFullCmdStr(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var fullCmdStr string
	log := logger.Logger()
	envValStr := ""
	for _, env := range envVal {
		envValStr += env + " "
	}

	if chrootPath != HostPath {
		if _, err := os.Stat(chrootPath); os.IsNotExist(err) {
			return cmdStr, fmt.Errorf("chroot path %s does not exist", chrootPath)
		}

		proxyEnv := GetOSProxyEnvirons()

		for key, value := range proxyEnv {
			envValStr += key + "=" + value + " "
		}

		fullCmdStr = "sudo " + envValStr + "chroot " + chrootPath + " " + cmdStr
		chrootDir := filepath.Base(chrootPath)
		log.Debugf("Chroot " + chrootDir + " Exec: [" + cmdStr + "]")

	} else {
		if sudo {
			proxyEnv := GetOSProxyEnvirons()

			for key, value := range proxyEnv {
				envValStr += key + "=" + value + " "
			}

			fullCmdStr = "sudo " + envValStr + cmdStr
			log.Debugf("Exec: [sudo " + cmdStr + "]")
		} else {
			fullCmdStr = cmdStr
			log.Debugf("Exec: [" + cmdStr + "]")
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

	shell := getShell()
	cmd := exec.Command(shell, "-c", fullCmdStr)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if outputStr != "" {
			log.Infof(outputStr)
		}
		return outputStr, fmt.Errorf("failed to exec %s: %w", fullCmdStr, err)
	} else {
		if outputStr != "" {
			log.Debugf(outputStr)
		}
		return outputStr, nil
	}
}

// ExecCmdWithStream executes a command and streams its output
func ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var outputStr string
	log := logger.Logger()

	fullCmdStr, err := GetFullCmdStr(cmdStr, sudo, chrootPath, envVal)
	if err != nil {
		return "", fmt.Errorf("failed to get full command string: %w", err)
	}

	shell := getShell()
	cmd := exec.Command(shell, "-c", fullCmdStr)
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
				log.Infof(str)
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			str := scanner.Text()
			if str != "" {
				log.Infof(str)
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

	shell := getShell()
	cmd := exec.Command(shell, "-c", fullCmdStr)
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
