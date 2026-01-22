package shell_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

// TestYAMLConfigurationCommands tests all configuration commands from ubuntu24-x86_64-minimal-ptl.yml
// to ensure they can be properly parsed and verified by the shell command verification system.
// This test is particularly important for commands with nested quotes.
func TestYAMLConfigurationCommands(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{
			name:    "simple touch command",
			cmd:     "touch /etc/dummytest.txt",
			wantErr: false,
		},
		{
			name:    "echo with simple single quotes",
			cmd:     "echo 'yockgn01 dlstreamer x86_64 ubuntu24 image' > /etc/yockgn01.txt",
			wantErr: false,
		},
		{
			name:    "echo with double quotes inside single quotes - ftp proxy",
			cmd:     `echo 'Acquire::ftp::Proxy "http://proxy-dmz.intel.com:911";' > /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr: false,
		},
		{
			name:    "echo with double quotes inside single quotes - http proxy",
			cmd:     `echo 'Acquire::http::Proxy "http://proxy-dmz.intel.com:911";' >> /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr: false,
		},
		{
			name:    "echo with double quotes inside single quotes - https proxy",
			cmd:     `echo 'Acquire::https::Proxy "http://proxy-dmz.intel.com:911";' >> /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr: false,
		},
		{
			name:    "echo with double quotes inside single quotes - direct proxy domain",
			cmd:     `echo 'Acquire::https::proxy::af01p-png.devtools.intel.com "DIRECT";' >> /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr: false,
		},
		{
			name:    "echo with double quotes inside single quotes - wildcard domain",
			cmd:     `echo 'Acquire::https::proxy::*.intel.com "DIRECT";' >> /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr: false,
		},
		{
			name:    "echo environment variable",
			cmd:     "echo 'http_proxy=http://proxy-dmz.intel.com:911' >> /etc/environment",
			wantErr: false,
		},
		{
			name:    "echo with complex no_proxy list",
			cmd:     "echo 'no_proxy=localhost,127.0.0.1,127.0.1.1,127.0.0.0/8,172.16.0.0/20,192.168.0.0/16,10.0.0.0/8,10.1.0.0/16,10.152.183.0/24,devtools.intel.com,jf.intel.com,teamcity-or.intel.com,caas.intel.com,inn.intel.com,isscorp.intel.com,gfx-assets.fm.intel.com' >> /etc/environment",
			wantErr: false,
		},
		{
			name:    "sed with special characters",
			cmd:     "sed -e 's@archive.ubuntu.com@mirrors.gbnetwork.com@g' -i /etc/apt/sources.list.d/ubuntu.sources",
			wantErr: false,
		},
		{
			name:    "mkdir with mode and path expansion",
			cmd:     "mkdir -m 700 -p ~sys_olvtelemetry/.ssh",
			wantErr: false,
		},
		{
			name:    "echo ssh key to file",
			cmd:     "echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOPEVYF28+I92b3HFHOSlPQXt3kHXQ9IqtxFE4/0YkK5 swsbalabuser@BA02RNL99999' > ~sys_olvtelemetry/.ssh/authorized_keys",
			wantErr: false,
		},
		{
			name:    "chmod command",
			cmd:     "chmod 600 ~sys_olvtelemetry/.ssh/authorized_keys",
			wantErr: false,
		},
		{
			name:    "chown with recursive flag",
			cmd:     "chown sys_olvtelemetry:sys_olvtelemetry -R ~sys_olvtelemetry/.ssh",
			wantErr: false,
		},
		{
			name:    "sed with double quotes and special chars in replacement",
			cmd:     `sed -i 's/GRUB_CMDLINE_LINUX=.*/GRUB_CMDLINE_LINUX="xe.max_vfs=7 xe.force_probe=* modprobe.blacklist=i915 udmabuf.list_limit=8192 console=tty0 console=ttyS0,115200n8"/' /etc/default/grub`,
			wantErr: false,
		},
		{
			name:    "sed with path containing angle brackets",
			cmd:     `sed -i 's/GRUB_DEFAULT=.*/GRUB_DEFAULT="Advanced options for Ubuntu>Ubuntu, with Linux 6.17-intel"/' /etc/default/grub`,
			wantErr: false,
		},
		{
			name:    "mkdir with verbose flag",
			cmd:     "mkdir -m 755 -pv /opt/vpu",
			wantErr: false,
		},
		{
			name:    "curl with tar pipeline",
			cmd:     "curl -s https://af01p-ir.devtools.intel.com/artifactory/drivers_vpu_linux_client-ir-local/engineering-drops/driver/main/release/25ww49.1.1/npu-linux-driver-ci-1.30.0.20251128-19767695845-ubuntu2404-release.tar.gz | tar -zxv --strip-components=1 -C /opt/vpu -f -",
			wantErr: false,
		},
		{
			name:    "wget with output flag",
			cmd:     "wget https://af01p-png.devtools.intel.com/artifactory/hspe-edge-png-local/ubuntu-mtl-audio-tplg-6/c0/intel/sof-ipc4/mtl/sof-mtl.ldc -O /lib/firmware/intel/sof-ipc4/mtl/sof-mtl.ldc",
			wantErr: false,
		},
		{
			name:    "sed with comment substitution",
			cmd:     "sed -e 's@^GRUB_TIMEOUT_STYLE=hidden@# GRUB_TIMEOUT_STYLE=hidden@' -e 's@^GRUB_TIMEOUT=0@GRUB_TIMEOUT=5@g' -i /etc/default/grub",
			wantErr: false,
		},
		{
			name:    "sed with uncomment pattern",
			cmd:     "sed -i 's/#WaylandEnable=/WaylandEnable=/g' /etc/gdm3/custom.conf",
			wantErr: false,
		},
		{
			name:    "sed with double quote substitution",
			cmd:     `sed -i 's/"1"/"0"/g' /etc/apt/apt.conf.d/20auto-upgrades`,
			wantErr: false,
		},
		{
			name:    "echo with pipe to tee",
			cmd:     "echo 'source /etc/profile.d/mesa_driver.sh' | tee -a /etc/bash.bashrc",
			wantErr: false,
		},
		{
			name:    "echo with append to file",
			cmd:     "echo 'set enable-bracketed-paste off' >> /etc/inputrc",
			wantErr: false,
		},
		{
			name:    "sed with boolean value substitution",
			cmd:     "sed -i 's/.*AutomaticLoginEnable =.*/AutomaticLoginEnable = true/g' /etc/gdm3/custom.conf",
			wantErr: false,
		},
		{
			name:    "echo with sudo permissions",
			cmd:     "echo 'sys_olvtelemetry ALL=(ALL) NOPASSWD: /usr/sbin/biosdecode, /usr/sbin/dmidecode, /usr/sbin/ownership, /usr/sbin/vpddecode' > /etc/sudoers.d/user-sudo",
			wantErr: false,
		},
		{
			name:    "chmod with octal mode",
			cmd:     "chmod 440 /etc/sudoers.d/user-sudo",
			wantErr: false,
		},
		{
			name:    "echo with kernel parameter",
			cmd:     "echo 'kernel.printk = 7 4 1 7' > /etc/sysctl.d/99-kernel-printk.conf",
			wantErr: false,
		},
		{
			name:    "echo with shebang",
			cmd:     "echo '#!/bin/bash' > /opt/snapd_refresh.sh",
			wantErr: false,
		},
		{
			name:    "chmod with execute permission",
			cmd:     "chmod +x /opt/snapd_refresh.sh",
			wantErr: false,
		},
		{
			name:    "echo with command substitution",
			cmd:     `echo "BUILD_TIME=$(date +%Y%m%d-%H%M)" > /opt/jenkins-build-timestamp`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the command can be properly verified and processed
			fullCmd, err := shell.GetFullCmdStr(tt.cmd, false, shell.HostPath, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetFullCmdStr() expected error but got none for cmd: %s", tt.cmd)
				}
				return
			}

			if err != nil {
				t.Errorf("GetFullCmdStr() error = %v, cmd = %s", err, tt.cmd)
				return
			}

			// Verify that the command was processed (should contain a full path)
			if !strings.Contains(fullCmd, "/") {
				t.Errorf("GetFullCmdStr() returned command without full path: %s", fullCmd)
			}
		})
	}
}

// TestEchoWithNestedQuotes specifically tests the fix for echo commands with nested quotes
func TestEchoWithNestedQuotes(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		wantErr     bool
		description string
	}{
		{
			name:        "single quotes with double quotes inside",
			cmd:         `echo 'Acquire::ftp::Proxy "http://proxy-dmz.intel.com:911";' > /etc/apt/apt.conf.d/99proxy.conf`,
			wantErr:     false,
			description: "This was the failing case - single quotes containing double quotes",
		},
		{
			name:        "single quotes with multiple double quotes",
			cmd:         `echo 'key1="value1" key2="value2"' > /etc/config`,
			wantErr:     false,
			description: "Multiple key-value pairs with double quotes inside single quotes",
		},
		{
			name:        "double quotes with escaped double quotes inside",
			cmd:         `echo "She said \"hello\" to me" > /etc/greeting`,
			wantErr:     false,
			description: "Double quotes with escaped double quotes inside",
		},
		{
			name:        "single quotes with special characters and double quotes",
			cmd:         `echo 'Pattern: "*.txt" Count: 42' > /etc/pattern.conf`,
			wantErr:     false,
			description: "Mix of special characters and double quotes in single quotes",
		},
		{
			name:        "empty string in single quotes",
			cmd:         `echo '' > /etc/empty`,
			wantErr:     false,
			description: "Empty string should work",
		},
		{
			name:        "empty string in double quotes",
			cmd:         `echo "" > /etc/empty`,
			wantErr:     false,
			description: "Empty double quoted string should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullCmd, err := shell.GetFullCmdStr(tt.cmd, false, shell.HostPath, nil)

			if tt.wantErr && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("%s: unexpected error = %v\nCommand: %s", tt.description, err, tt.cmd)
			}

			if err == nil && !strings.Contains(fullCmd, "echo") {
				t.Errorf("%s: processed command doesn't contain 'echo': %s", tt.description, fullCmd)
			}
		})
	}
}

// TestComplexPipelineCommands tests commands with multiple stages and pipelines
func TestComplexPipelineCommands(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{
			name:    "pipeline with curl and tar",
			cmd:     "curl -s https://example.com/file.tar.gz | tar -zxv --strip-components=1 -C /opt/dir -f -",
			wantErr: false,
		},
		{
			name:    "pipeline with tee",
			cmd:     "echo 'source /etc/profile.d/script.sh' | tee -a /etc/bashrc",
			wantErr: false,
		},
		{
			name:    "multiple commands with semicolon",
			cmd:     "mkdir -p /opt/dir; chmod 755 /opt/dir",
			wantErr: false,
		},
		{
			name:    "multiple commands with AND operator",
			cmd:     "mkdir -p /opt/dir && cd /opt/dir",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.GetFullCmdStr(tt.cmd, false, shell.HostPath, nil)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none for cmd: %s", tt.cmd)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error = %v for cmd: %s", err, tt.cmd)
			}
		})
	}
}

// TestChrootCommandVerification simulates the EXACT flow used in imageos.go
// where commands are wrapped with chroot and quoted with strconv.Quote
func TestChrootCommandVerification(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		installRoot string
		wantErr     bool
	}{
		{
			name:        "simple echo in chroot",
			cmd:         `echo 'yockgn01 dlstreamer x86_64 ubuntu24 image' > /etc/yockgn01.txt`,
			installRoot: "/data/os-image-composer/workspace/ubuntu-ubuntu24-x86_64/chrootenv/workspace/imagebuild/minimal",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the EXACT process from imageos.go:823
			chrootCmd := fmt.Sprintf("chroot %s /bin/bash -c %s", tt.installRoot, strconv.Quote(tt.cmd))

			// This is what gets called in shell.ExecCmd -> GetFullCmdStr
			_, err := shell.GetFullCmdStr(chrootCmd, true, shell.HostPath, nil)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
				t.Logf("Command: %s", tt.cmd)
				t.Logf("ChrootCmd: %s", chrootCmd)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error = %v", err)
				t.Logf("Command: %s", tt.cmd)
				t.Logf("ChrootCmd: %s", chrootCmd)
			}
		})
	}
}
