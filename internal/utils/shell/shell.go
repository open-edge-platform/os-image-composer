package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
)

const HostPath string = "/"

var log = logger.Logger()

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

// IsBashAvailable checks if bash is available in the given chroot environment
func IsBashAvailable(chrootPath string) bool {
	bashPath := "/usr/bin/bash"
	if _, err := os.Stat(filepath.Join(chrootPath, bashPath)); err == nil {
		return true
	}
	log.Debugf("bash not found in chroot path %s", chrootPath)
	return false
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

// GetFullCmdStr prepares a command string with necessary prefixes
func GetFullCmdStr(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	var fullCmdStr string
	envValStr := ""
	for _, env := range envVal {
		envValStr += env + " "
	}

	if chrootPath != HostPath {
		if _, err := os.Stat(chrootPath); os.IsNotExist(err) {
			return "", fmt.Errorf("chroot path %s does not exist", chrootPath)
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
