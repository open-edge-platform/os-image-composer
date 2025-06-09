package shell

import (
	"os/exec"
	"strings"
	"testing"
)

// checkShellAvailable checks if a shell is available for testing
func checkShellAvailable(t *testing.T) {
	t.Helper()
	shells := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"}
	for _, shell := range shells {
		if _, err := exec.LookPath(shell); err == nil {
			return // Found a shell
		}
	}
	t.Skip("No shell (bash or sh) available in test environment")
}

func TestGetFullCmdStr(t *testing.T) {
	cmd, err := GetFullCmdStr("echo hello", false, HostPath, nil)
	if err != nil {
		t.Fatalf("GetFullCmdStr failed: %v", err)
	}
	if !strings.Contains(cmd, "echo hello") {
		t.Errorf("Expected command 'echo hello', got: %s", cmd)
	}
}

func TestExecCmd(t *testing.T) {
	checkShellAvailable(t)

	out, err := ExecCmd("echo test-exec-cmd", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-cmd") {
		t.Errorf("Expected output to contain 'test-exec-cmd', got: %s", out)
	}
}

func TestExecCmdWithStream(t *testing.T) {
	checkShellAvailable(t)

	out, err := ExecCmdWithStream("echo test-exec-stream", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithStream failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-stream") {
		t.Errorf("Expected output to contain 'test-exec-stream', got: %s", out)
	}
}

func TestExecCmdWithInput(t *testing.T) {
	checkShellAvailable(t)

	out, err := ExecCmdWithInput("input-line", "cat", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithInput failed: %v", err)
	}
	if !strings.Contains(out, "input-line") {
		t.Errorf("Expected output to contain 'input-line', got: %s", out)
	}
}
