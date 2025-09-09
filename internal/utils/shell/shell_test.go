package shell_test

import (
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func TestGetFullCmdStr(t *testing.T) {
	cmd, err := shell.GetFullCmdStr("echo 'hello'", false, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("GetFullCmdStr failed: %v", err)
	}
	if !(strings.Contains(cmd, "/usr/bin/echo 'hello'") || strings.Contains(cmd, "/bin/echo 'hello'")) {
		t.Errorf("Expected full path for echo, got: %s", cmd)
	}
}

func TestExecCmd(t *testing.T) {
	out, err := shell.ExecCmd("echo 'test-exec-cmd'", false, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-cmd") {
		t.Errorf("Expected output to contain 'test-exec-cmd', got: %s", out)
	}
}

func TestExecCmdWithStream(t *testing.T) {
	out, err := shell.ExecCmdWithStream("echo 'test-exec-stream'", false, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithStream failed: %v", err)
	}
	if !strings.Contains(out, "test-exec-stream") {
		t.Errorf("Expected output to contain 'test-exec-stream', got: %s", out)
	}
}

func TestExecCmdWithInput(t *testing.T) {
	out, err := shell.ExecCmdWithInput("input-line", "cat", false, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithInput failed: %v", err)
	}
	if !strings.Contains(out, "input-line") {
		t.Errorf("Expected output to contain 'input-line', got: %s", out)
	}
}

func TestExecCmdOverride(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "echo 'test-exec-cmd-override'", Output: "override-test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)
	out, err := shell.ExecCmd("echo 'test-exec-cmd-override'", true, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdSilentOverride(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "echo 'test-exec-cmd-override'", Output: "override-test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)
	out, err := shell.ExecCmdSilent("echo 'test-exec-cmd-override'", false, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmd with silent override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdWithStreamOverride(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "echo 'test-exec-cmd-override'", Output: "override-test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)
	out, err := shell.ExecCmdWithStream("echo 'test-exec-cmd-override'", true, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithStream with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}

func TestExecCmdWithInputOverride(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "echo 'test-exec-cmd-override'", Output: "override-test\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)
	out, err := shell.ExecCmdWithInput("input-line", "echo 'test-exec-cmd-override'", true, shell.HostPath, nil)
	if err != nil {
		t.Fatalf("ExecCmdWithInput with override failed: %v", err)
	}
	if !strings.Contains(out, "override-test") {
		t.Errorf("Expected output to contain 'override-test', got: %s", out)
	}
}
