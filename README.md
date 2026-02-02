# Madani OS Builder

> **Note:** This is a fork of upstream/project. It contains custom changes for Madani OS use and is not intended for upstream contribution.

---

## Overview

Madani OS Builder is a specialized command-line tool designed to build lightweight Linux distributions optimized for low-end machines with pre-built AI stack capabilities. Using a simple toolchain, it creates mutable or immutable Madani OS images from pre-built packages sourced from various OS distribution repositories.

Developed in Go, the tool specializes in building custom Madani OS images optimized for efficient AI workload execution on resource-constrained hardware. The tool's versatile architecture also supports building images for other Linux distributions including Ubuntu, Wind River eLxr, Azure Linux, and Intel EMT.

---

## Table of Contents

- [Overview](#overview)
- [System Requirements](#system-requirements)
  - [Windows WSL Setup (Automated)](#windows-wsl-setup-automated)
- [Quick Start Guide](#quick-start-guide)
- [Testing with the OS Raw Image](#testing-with-the-os-raw-image)
  - [Usage Examples](#usage-examples)
- [Advanced Topics](#advanced-topics)
  - [Build Options](#build-options)
  - [Debian Package Installation](#debian-package-installation)
- [Configuration](#configuration)
- [Operations Requiring Sudo Access](#operations-requiring-sudo-access)
- [Usage](#usage)
- [Image Template Format](#image-template-format)
- [Template Examples](#template-examples)
- [Resources](#resources)
- [Legal](#legal)

---

## System Requirements

**Recommended Operating System:** Ubuntu 24.04

> **Note:** Madani Team has validated and recommends using Ubuntu OS version 24.04. Other Linux distributions have not been validated. Future releases will include a containerized version for enhanced portability.

### Windows WSL Setup (Automated)

**For Windows users who want to run Madani OS Builder in WSL:**

Use the provided automated setup script to configure Ubuntu 24.04 in WSL with all required dependencies:

```cmd
# Download and run the setup script
setup-wsl.bat
```

**What the script does:**
- Installs Ubuntu 24.04 in WSL (if not already installed)
- Sets up Go programming language (v1.25.5)
- Creates `/data` directory for the project
- Clones the repository to `/data/madani-os-builder`
- Configures proper permissions and PATH variables

**After running the script:**
1. The script will automatically launch WSL
2. Navigate to the project directory: `cd /data/madani-os-builder`
3. Continue with [step 3 of the Quick Start Guide](#3-build-the-tool)

> **Note:** If prompted to create a username/password during Ubuntu installation, complete the setup and return to the batch script window.

---

## Quick Start Guide

### 1. Download the Tool

```bash
git clone https://github.com/yockgen/madani-os-builder/
cd madani-os-builder
git checkout $(git describe --tags --abbrev=0)
```

### 2. Install Prerequisites

Install Go programming language version 1.22.12 or later. See the [Go installation instructions](https://go.dev/doc/manage-install) for your Linux distribution.

### 3. Build the Tool

```bash
sudo go build -buildmode=pie -ldflags "-s -w" ./cmd/os-image-composer
```

### 4. Build a Madani OS Image

```bash
sudo -E ./os-image-composer build --cache-dir ./cache/ -v image-templates/madani24-x86_64-minimal-raw.yml 2>&1 | tee yockgen-madani.txt
```

### 5. Locate the Output Image

```bash
ls ./workspace/madani-madani24-x86_64/imagebuild/minimal/minimal-os-image-madani-24.04.raw.gz
```

> **Note:** The `minimal-os-image-madani-24.04.raw.gz` file is a compressed raw disk image containing the complete Madani OS.

### 6. Cleanup (Optional)

> **⚠️ Warning:** Copy the raw image to another location before running this command if you need to preserve it.

```bash
sudo rm -rf ./tmp/ ./build ./cache/ ./workspace/
```

---

## Testing with the OS Raw Image

The raw image file can be deployed on various platforms:

- **Physical machines:** Flash directly to a disk or USB drive for bare-metal installation
- **Virtual Machines:** Import into virtualization platforms (VirtualBox, VMware, etc.)
- **QEMU:** Boot directly using QEMU emulator
- **Cloud instances:** Deploy to cloud providers that support raw disk images

### Usage Examples

#### Extract the Compressed File

```bash
gzip -dc ./workspace/madani-madani24-x86_64/imagebuild/minimal/minimal-os-image-madani-24.04.raw.gz > /data/raw/test-madani-final.raw
```

#### Run with QEMU (No GUI)

```bash
sudo qemu-system-x86_64 \
  -machine q35 \
  -m 2048 \
  -cpu max \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,file=/usr/share/OVMF/OVMF_VARS_4M.fd \
  -device virtio-scsi-pci \
  -drive if=none,id=drive0,file=test-madani-final.raw,format=raw \
  -device scsi-hd,drive=drive0 \
  -netdev user,id=net0,hostfwd=tcp::2223-:22,hostfwd=tcp::8081-:80 \
  -device virtio-net-pci,netdev=net0 \
  -nographic \
  -serial mon:stdio
```

#### Run with QEMU (with GUI)

**For Windows WSL users:**

```bash
# Install QEMU and setup permissions
sudo apt install qemu-system-x86 ovmf
sudo usermod -aG kvm $USER && newgrp kvm
sudo chmod 666 /dev/kvm

# Prepare VM files
cd /data/raw
chmod 777 test-madani-final.raw
cp /usr/share/OVMF/OVMF_VARS_4M.fd ./my_vars.fd
chmod 644 ./my_vars.fd

# Run QEMU with GUI
qemu-system-x86_64 \
  -machine q35 \
  -m 4G \
  -smp 4,cores=4,threads=1 \
  -cpu host,migratable=no,+invtsc \
  -accel kvm \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
  -drive if=pflash,format=raw,file=./my_vars.fd \
  -device virtio-scsi-pci \
  -drive if=none,id=drive0,file=test-madani-final.raw,format=raw,cache=none,aio=native \
  -device scsi-hd,drive=drive0 \
  -netdev user,id=net0,hostfwd=tcp::2223-:22,hostfwd=tcp::8081-:80 \
  -device virtio-net-pci,netdev=net0 \
  -vga virtio \
  -display gtk,gl=on
```

**Troubleshooting:** If no GUI appears, ensure X11 server is running on Windows or use WSLg (Windows 11 22H2+).

> **⚠️ Warning:** Flashing this image will overwrite all existing data on the target device. Ensure you have selected the correct destination and have backed up any important data.

---

## Advanced Topics

### Build Options

#### Development Build (Go Programming Language)

For development and testing purposes:

```bash
# Build the tool
go build -buildmode=pie -ldflags "-s -w" ./cmd/os-image-composer

# Build the live-installer (Required for ISO image)
go build -buildmode=pie -o ./build/live-installer -ldflags "-s -w" ./cmd/live-installer

# Or run it directly
go run ./cmd/os-image-composer --help
```

> **Note:** Development builds using `go build` show default version information (e.g., `Version: 0.1.0`, `Build Date: unknown`). This is expected during development.

#### Development Build with Version Information

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u '+%Y-%m-%d')

go build -buildmode=pie \
  -ldflags "-s -w \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Version=$VERSION' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Toolname=Image-Composer' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Organization=Open Edge Platform' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.BuildDate=$BUILD_DATE' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.CommitSHA=$COMMIT'" \
  ./cmd/os-image-composer

# Required for ISO image
go build -buildmode=pie \
  -o ./build/live-installer \
  -ldflags "-s -w \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Version=$VERSION' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Toolname=Image-Composer' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Organization=Open Edge Platform' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.BuildDate=$BUILD_DATE' \
    -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.CommitSHA=$COMMIT'" \
  ./cmd/live-installer
```

#### Production Build (Earthly Framework)

For production and release builds with reproducible builds:

```bash
# Default build (uses latest Git tag for version)
earthly +build

# Build with custom version metadata
earthly +build --VERSION=1.2.0
```

### Debian Package Installation

#### Build the Debian Package

```bash
# Build with default parameters (latest git tag, amd64)
earthly +deb

# Build with custom version and architecture
earthly +deb --VERSION=1.2.0 --ARCH=amd64

# Build for ARM64
earthly +deb --VERSION=1.0.0 --ARCH=arm64
```

The package is created in the `dist/` directory as `os-image-composer_<VERSION>_<ARCH>.deb`.

#### Install the Package

```bash
# Install using apt (recommended - automatically resolves dependencies)
sudo apt install <PATH TO FILE>/os-image-composer_1.0.0_amd64.deb

# Or using dpkg (requires manual dependency installation)
sudo apt-get update
sudo apt-get install -y bash coreutils unzip dosfstools xorriso grub-common
sudo dpkg -i dist/os-image-composer_1.0.0_amd64.deb
# Optionally install bootstrap tools:
sudo apt-get install -y mmdebstrap || sudo apt-get install -y debootstrap
```

#### Verify Installation

```bash
# Check if package is installed
dpkg -l | grep os-image-composer

# View installed files
dpkg -L os-image-composer

# Verify the binary works
os-image-composer version
```

#### Package Contents

The Debian package installs:

- **Binary:** `/usr/local/bin/os-image-composer` - Main executable file
- **Configuration:** `/etc/os-image-composer/` - Default configuration and OS variant configurations
  - `/etc/os-image-composer/config.yml` - Global configuration with system paths
  - `/etc/os-image-composer/config/` - OS variant configuration files
- **Examples:** `/usr/share/os-image-composer/examples/` - Sample image templates
- **Documentation:** `/usr/share/doc/os-image-composer/` - README, LICENSE, and CLI specification
- **Cache Directory:** `/var/cache/os-image-composer/` - Package cache storage

#### Package Dependencies

**Required Dependencies:**
- `bash` - Shell for script execution
- `coreutils` - Core GNU utilities
- `unzip` - Archive extraction utility
- `dosfstools` - FAT filesystem utilities
- `xorriso` - ISO image creation tool
- `grub-common` - Bootloader utilities

**Recommended Dependencies:**
- `mmdebstrap` - Debian bootstrap tool (preferred, version 1.4.3+ required)
- `debootstrap` - Alternative Debian bootstrap tool

> **Important:** `mmdebstrap` version 0.8.x (included in Ubuntu 22.04) has known issues. For Ubuntu 22.04 users, you must install `mmdebstrap` version 1.4.3+ manually.

#### Uninstall the Package

```bash
# Remove the package but keep configuration files
sudo dpkg -r os-image-composer

# Remove the package and configuration files
sudo dpkg --purge os-image-composer
```

### Prerequisites for Image Composition

Before composing an OS image, install additional prerequisites:

**Required Tools:**

- **`ukify`** - Combines kernel, initrd, and UEFI boot stub to create signed Unified Kernel Images (UKI)
  - **Ubuntu 23.04+**: `sudo apt install systemd-ukify`
  - **Ubuntu 22.04 and earlier**: Must be installed manually from systemd source
  - See [detailed ukify installation instructions](./docs/tutorial/prerequisite.md#ukify)

- **`mmdebstrap`** - Downloads and installs Debian packages to initialize a chroot
  - **Ubuntu 23.04+**: Automatically installed with the Debian package (version 1.4.3+)
  - **Ubuntu 22.04**: The version in repositories (0.8.x) has known bugs
    - **Required:** Manually install version 1.4.3+. See [mmdebstrap installation instructions](./docs/tutorial/prerequisite.md#mmdebstrap)
  - **Alternative**: Can use `debootstrap` for Debian-based images

### Compose or Validate an Image

```bash
# Build an image from template
sudo -E ./os-image-composer build image-templates/azl3-x86_64-edge-raw.yml

# If installed via Debian package, use system paths:
sudo os-image-composer build /usr/share/os-image-composer/examples/azl3-x86_64-edge-raw.yml

# Validate a template:
./os-image-composer validate image-templates/azl3-x86_64-edge-raw.yml
```

After the image is built, check your output directory:

```
/os-image-composer/tmp/os-image-composer/azl3-x86_64-edge-raw/imagebuild/Minimal_Raw
```

---

## Configuration

### Global Configuration

The OS Image Composer tool supports global configuration files for setting tool-level parameters that apply across all image builds.

#### Configuration File Locations

The tool searches for configuration files in the following order:

1. `os-image-composer.yaml` (current directory)
2. `os-image-composer.yml` (current directory)
3. `.os-image-composer.yaml` (hidden file in current directory)
4. `~/.os-image-composer/config.yaml` (user home directory)
5. `~/.config/os-image-composer/config.yaml` (XDG config directory)
6. `/etc/os-image-composer/config.yaml` (system-wide)

#### Configuration Parameters

```yaml
# Core tool settings
workers: 12                                 # Number of concurrent download workers (1-100, default: 8)
cache_dir: "/var/cache/os-image-composer"   # Package cache directory (default: ./cache)
work_dir: "/tmp/os-image-composer"          # Working directory for builds (default: ./workspace)
temp_dir: ""                               # Temporary directory (empty = system default)

# Logging configuration
logging:
  level: "info"                            # Log level: debug, info, warn, error (default: info)
```

#### Configuration Management Commands

```bash
# Create a new configuration file
./os-image-composer config init

# Create configuration file at specific location
./os-image-composer config init /path/to/config.yaml

# Show current configuration
./os-image-composer config show

# Use specific configuration file
./os-image-composer --config /path/to/config.yaml build template.yml
```

---

## Operations Requiring Sudo Access

The OS Image Composer performs several system-level operations that require elevated privileges (sudo access).

### System Directory Access and Modification

The following system directories require root access for OS Image Composer operations:

- **`/etc/` directory operations**: Writing system configuration files, modifying network configurations, updating system settings
- **`/dev/` device access**: Block device operations, loop device management, and hardware access
- **`/sys/` filesystem access**: System parameter modification and kernel interface access
- **`/proc/` filesystem modification**: Process and system state changes
- **`/boot/` directory**: Boot loader and kernel image management
- **`/var/` system directories**: System logs, package databases, and runtime state
- **`/usr/sbin/` and `/sbin/`**: System administrator binaries

### Common Privileged Operations

OS Image Composer typically requires sudo access for:

- **Block device management**: Creating loop devices, partitions, and filesystem
- **Mount/unmount operations**: Mounting filesystems and managing mount points
- **Chroot environment setup**: Creating and managing isolated build environments
- **Package installation**: System-wide package management operations
- **Boot configuration**: Installing bootloaders and managing EFI settings
- **Security operations**: Secure boot signing and cryptographic operations

---

## Usage

### Basic Commands

```bash
# Show help
./os-image-composer --help

# Build command with template file as positional argument
sudo -E ./os-image-composer build image-templates/azl3-x86_64-edge-raw.yml

# Override config settings with command-line flags
sudo -E ./os-image-composer build --workers 16 --cache-dir /tmp/cache image-templates/azl3-x86_64-edge-raw.yml

# Validate a template file against the schema
./os-image-composer validate image-templates/azl3-x86_64-edge-raw.yml

# Display version information
./os-image-composer version

# Install shell completion for your current shell
./os-image-composer completion install
```

### Commands Reference

#### `build`

Builds a Linux distribution image based on the specified image template file:

```bash
sudo -E ./os-image-composer build [flags] TEMPLATE_FILE
```

**Flags:**
- `--workers, -w`: Number of concurrent download workers (overrides the configuration file)
- `--cache-dir, -d`: Package cache directory (overrides the configuration file)
- `--work-dir`: Working directory for builds (overrides the configuration file)
- `--verbose, -v`: Enable verbose output
- `--config`: Path to the configuration file
- `--log-level`: Log level (debug, info, warn, and error)
- `--log-file`: Override the log file path defined in the configuration

**Example:**
```bash
sudo -E ./os-image-composer build --workers 12 --cache-dir ./package-cache image-templates/azl3-x86_64-edge-raw.yml
```

#### `config`

Manages the global configuration:

```bash
# Show current configuration
./os-image-composer config show

# Initialize new configuration file
./os-image-composer config init [config-file]
```

#### `validate`

Validates a YAML template file against the schema without building an image:

```bash
./os-image-composer validate TEMPLATE_FILE
```

#### `version`

Displays the tool's version number, build date, and Git commit SHA:

```bash
./os-image-composer version
```

#### `completion`

Generates and installs shell completion scripts for various shells.

**Prerequisites:** The `os-image-composer` binary must be accessible in your system's `$PATH`.

##### Generate Completion Scripts

```bash
# Generate completion script for bash (output to stdout)
os-image-composer completion bash

# Generate completion script for other shells
os-image-composer completion zsh
os-image-composer completion fish
os-image-composer completion powershell
```

##### Install Completion Automatically

```bash
# Auto-detect shell and install completion file
os-image-composer completion install

# Specify shell type
os-image-composer completion install --shell bash
os-image-composer completion install --shell zsh
os-image-composer completion install --shell fish
os-image-composer completion install --shell powershell

# Force overwrite existing completion files
os-image-composer completion install --force
```

##### Shell-Specific Activation

**Bash:**
```bash
# Add to your ~/.bashrc
echo "source ~/.bash_completion.d/os-image-composer.bash" >> ~/.bashrc
source ~/.bashrc
```

**Zsh:**
```zsh
# Ensure completion directory is in fpath (add to ~/.zshrc if needed)
echo 'fpath=(~/.zsh/completion $fpath)' >> ~/.zshrc
source ~/.zshrc
```

**Fish:** (Works automatically after restart)

**PowerShell:**
```powershell
# May need to allow script execution
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
. $PROFILE
```

**Test the completion:**
```bash
os-image-composer [TAB]
os-image-composer b[TAB]
os-image-composer build --[TAB]
```

---

## Image Template Format

Written in YAML format, templates define the requirements for building an OS image. The template structure enables you to define key parameters, such as the OS distribution, version, architecture, software packages, output format, and kernel configuration.

### Basic Template Structure

```yaml
image:
  name: azl3-x86_64-edge
  version: "1.0.0"

target:
  os: azure-linux    # Target OS name
  dist: azl3          # Target OS distribution
  arch: x86_64        # Target OS architecture
  imageType: raw      # Image type: raw, iso

systemConfigs:
  - name: edge
    description: Default configuration for edge image

    # Package Configuration
    packages:
      # Additional packages beyond the base system
      - openssh-server      # Remote access
      - docker-ce          # Container runtime
      - vim                # Text editor
      - curl               # HTTP client
      - wget               # File downloader

    # Kernel Configuration
    kernel:
      version: "6.12"
      cmdline: "quiet splash"
```

### Key Components

#### 1. `image`
Basic image identification and metadata:
- `name`: Name of the resulting image
- `version`: Version for tracking and naming

#### 2. `target`
Defines the target OS and image configuration:
- `os`: Target OS (`azure-linux`, `emt`, and `elxr`)
- `dist`: Distribution identifier (`azl3`, `emt3`, and `elxr12`)
- `arch`: Target architecture (`x86_64` and `aarch64`)
- `imageType`: Output format (`raw` and `iso`)

#### 3. `systemConfigs`
Array of system configurations that define what goes into the image:
- `name`: Configuration name
- `description`: Human-readable description
- `packages`: List of packages to include in the OS build
- `kernel`: Kernel configuration with version and command-line parameters

### Supported Distributions

| OS | Distribution | Version | Provider |
|----|-------------|---------|----------|
| azure-linux | azl3 | 3 | AzureLinux3 |
| emt | emt3 | 3.0 | EMT3.0 |
| wind-river-elxr | elxr12 | 12 | eLxr12 |
| ubuntu | ubuntu24 | | ubuntu24 |
| madani | madani24 | | madani24 |

### Common Packages

- `cloud-init`: For initializing cloud instances
- `python3`: The Python 3 programming language interpreter
- `rsyslog`: A logging system for Linux OS
- `openssh-server`: SSH server for remote access
- `docker-ce`: Docker container runtime

---

## Template Examples

### Minimal Edge Device

```yaml
image:
  name: minimal-edge
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

systemConfigs:
  - name: minimal
    description: Minimal edge device configuration
    packages:
      - openssh-server
      - ca-certificates
    kernel:
      version: "6.12"
      cmdline: "quiet"
```

### Development Environment

```yaml
image:
  name: dev-environment
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

systemConfigs:
  - name: development
    description: Development environment with tools
    packages:
      - openssh-server
      - git
      - docker-ce
      - vim
      - curl
      - wget
      - python3
    kernel:
      version: "6.12"
      cmdline: "quiet splash"
```

### Edge Microvisor Toolkit

```yaml
image:
  name: emt-edge-device
  version: "1.0.0"

target:
  os: emt
  dist: emt3
  arch: x86_64
  imageType: raw

systemConfigs:
  - name: edge
    description: Edge Microvisor Toolkit configuration
    packages:
      - openssh-server
      - docker-ce
      - edge-runtime
      - telemetry-agent
    kernel:
      version: "6.12"
      cmdline: "quiet splash systemd.unified_cgroup_hierarchy=0"
```

---

## Resources

### Documentation

- Run `./os-image-composer --help` for all commands and options
- [CLI Specification and Reference](./docs/architecture/os-image-composer-cli-specification.md)
- [Complete Documentation](https://github.com/open-edge-platform/os-image-composer/tree/main/docs)
- [Build Process Documentation](./docs/architecture/os-image-composer-build-process.md#troubleshooting-build-issues)
- [Creating and Reusing Image Templates](./docs/architecture/os-image-composer-templates.md)

### Community

- [Participate in Discussions](https://github.com/open-edge-platform/os-image-composer/discussions)
- [Open an Issue](https://github.com/open-edge-platform/os-image-composer/issues)
- [Submit a Pull Request](https://github.com/open-edge-platform/os-image-composer/pulls)

### Security

- [Report a Security Vulnerability](./SECURITY.md)

---

## Legal

### License Information

See [License](./LICENSE).
