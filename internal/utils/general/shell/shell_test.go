package shell

import (
	"strings"
	"testing"
)

func TestGetFullCmdStr(t *testing.T) {
	cmd, err := GetFullCmdStr("echo hello", false, HostPath, nil)
	if err != nil {
		t.Fatalf("GetFullCmdStr failed: %v", err)
	}
	if !strings.Contains(cmd, "/usr/bin/echo hello") {
		t.Errorf("Expected full path for echo, got: %s", cmd)
	}
}

func TestExecCmd(t *testing.T) {
	out, err := ExecCmd("echo test-exec-cmd", false, HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-cmd") {
		t.Errorf("Expected output to contain 'test-exec-cmd', got: %s", out)
	}
}

func TestExecCmdWithStream(t *testing.T) {
	out, err := ExecCmdWithStream("echo test-exec-stream", false, HostPath, nil)
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
