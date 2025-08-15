package shell

import (
	"fmt"
	"strings"
	"testing"
)

var ExpectedOutput map[string][]interface{} = map[string][]interface{}{
	"echo 'hello'":                  {"hello\n", nil},
	"echo 'test-exec-cmd'":          {"test-exec-cmd\n", nil},
	"echo 'test-exec-cmd-override'": {"override-test\n", nil},
	"echo 'test-exec-stream'":       {"test-exec-stream\n", nil},
}

func ExecCmdOverride(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if output, exists := ExpectedOutput[cmdStr]; exists {
		if output[1] != nil {
			return output[0].(string), output[1].(error)
		} else {
			return output[0].(string), nil
		}
	} else {
		return "", fmt.Errorf("Unexpected command for override: %s", cmdStr)
	}
}

func ExecCmdWithInputOverride(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if output, exists := ExpectedOutput[cmdStr]; exists {
		if output[1] != nil {
			return output[0].(string), output[1].(error)
		} else {
			return output[0].(string), nil
		}
	} else {
		return "", fmt.Errorf("Unexpected command for override: %s", cmdStr)
	}
}

func TestGetFullCmdStr(t *testing.T) {
	cmd, err := GetFullCmdStr("echo 'hello'", false, HostPath, nil)
	if err != nil {
		t.Fatalf("GetFullCmdStr failed: %v", err)
	}
	if !strings.Contains(cmd, "/usr/bin/echo 'hello'") {
		t.Errorf("Expected full path for echo, got: %s", cmd)
	}
}

func TestExecCmd(t *testing.T) {
	out, err := ExecCmd("echo 'test-exec-cmd'", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-cmd") {
		t.Errorf("Expected output to contain 'test-exec-cmd', got: %s", out)
	}
}

func TestExecCmdWithStream(t *testing.T) {
	out, err := ExecCmdWithStream("echo 'test-exec-stream'", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithStream failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-stream") {
		t.Errorf("Expected output to contain 'test-exec-stream', got: %s", out)
	}
}

func TestExecCmdWithInput(t *testing.T) {
	out, err := ExecCmdWithInput("input-line", "cat", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithInput failed: %v", err)
	}
	if !strings.Contains(out, "input-line") {
		t.Errorf("Expected output to contain 'input-line', got: %s", out)
	}
}

func TestExecCmdOverride(t *testing.T) {
	var originalExecCmd = ExecCmd
	defer func() { ExecCmd = originalExecCmd }()
	ExecCmd = ExecCmdOverride
	out, err := ExecCmd("echo 'test-exec-cmd-override'", true, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdSilentOverride(t *testing.T) {
	var originalExecCmd = ExecCmdSilent
	defer func() { ExecCmdSilent = originalExecCmd }()
	ExecCmdSilent = ExecCmdOverride
	out, err := ExecCmdSilent("echo 'test-exec-cmd-override'", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd with silent override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdWithStreamOverride(t *testing.T) {
	var originalExecCmd = ExecCmdWithStream
	defer func() { ExecCmdWithStream = originalExecCmd }()
	ExecCmdWithStream = ExecCmdOverride
	out, err := ExecCmdWithStream("echo 'test-exec-cmd-override'", true, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithStream with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdWithInputOverride(t *testing.T) {
	var originalExecCmd = ExecCmdWithInput
	defer func() { ExecCmdWithInput = originalExecCmd }()
	ExecCmdWithInput = ExecCmdWithInputOverride
	out, err := ExecCmdWithInput("input-line", "echo 'test-exec-cmd-override'", true, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithInput with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}
