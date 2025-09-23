# Image Composer Tool

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Lint Check](https://github.com/open-edge-platform/os-image-composer/actions/workflows/go-lint.yml/badge.svg)](https://github.com/open-edge-platform/os-image-composer/actions/workflows/go-lint.yml)

The Image Composer Tool (ICT) is a toolchain that builds immutable
Linux distributions using a simple toolchain from pre-built packages provided by different Operating System Vendors (OSVs).

The ICT is developed in the Go programming language (or `golang`) and initially builds custom
images for [Edge Microvisor Toolkit](https://github.com/open-edge-platform/edge-microvisor-toolkit), [Linux OS for Azure 1P services and edge appliances](https://github.com/microsoft/azurelinux)
and Wind River eLxr.

## Documentation

- [📖 CLI Specification](./docs/architecture/os-image-composer-cli-specification.md) - Complete command-line reference and usage guide
- [🔧 Build Process](./docs/architecture/os-image-composer-build-process.md) - Details on the five-stage build pipeline
- [⚡ Caching](./docs/architecture/os-image-composer-caching.md) - Explanations on package cache and image cache to improve build performance and reduce resource usage
- [📋 Templates](./docs/architecture/os-image-composer-templates.md) - Explanations on how to create and reuse image templates

## Quick Start

```bash
# Build the tool
go build ./cmd/os-image-composer

# Or run directly
go run ./cmd/os-image-composer --help

# Build an image from template
./os-image-composer build image-templates/azl3-x86_64-edge-raw.yml

# Validate a template
./os-image-composer validate image-templates/azl3-x86_64-edge-raw.yml
```

For complete usage instructions, see the [CLI Specification](./docs/architecture/os-image-composer-cli-specification.md).

## Get Started

### Prerequisites

Image Composer Tool is developed in the Go programming language (or `golang`) and requires golang version 1.22.12 and above. See installation instructions for a specific distribution [here](https://go.dev/doc/manage-install).

> **Note:** Before building, check out [docs/tutorial/Pre-requisite](./docs/tutorial/Pre-requisite.md) for instructions to install required binaries.

### Build

Build the os-image-composer using Go directly:

```bash
go build ./cmd/os-image-composer
```

Or use Earthly framework for a reproducible build:

```bash
# Default build
earthly +build

# Build with specific version
earthly +build --version=1.0.0
```

The Earthly build automatically includes:

- Version number (from the --version parameter)
- Build date (the current UTC date)
- Git commit SHA (current repository commit)

## Configuration

### Global Configuration

Image Composer Tool supports global configuration files for setting tool-level parameters that apply across all image builds. Image-specific parameters are defined in YAML image template files.

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
workers: 12                              # Number of concurrent download workers (1-100, default: 8)
cache_dir: "/var/cache/os-image-composer"   # Package cache directory (default: ./cache)
work_dir: "/tmp/os-image-composer"          # Working directory for builds (default: ./workspace)
temp_dir: ""                             # Temporary directory (empty = system default)

# Logging configuration
logging:
  level: "info"                          # Log level: debug, info, warn, error (default: info)
```

#### Configuration Management Commands

```bash
# Create a new configuration file
./os-image-composer config init

# Create config file at specific location
./os-image-composer config init /path/to/config.yaml

# Show current configuration
./os-image-composer config show

# Use specific configuration file
./os-image-composer --config /path/to/config.yaml build template.yml
```

### Usage

The Image Composer Tool uses a command-line interface with various commands:

```bash
# Show help
./os-image-composer --help

# Build command with template file as positional argument
./os-image-composer build image-templates/azl3-x86_64-edge-raw.yml

# Override config settings with command-line flags
./os-image-composer build --workers 16 --cache-dir /tmp/cache image-templates/azl3-x86_64-edge-raw.yml

# Validate a template file against the schema
./os-image-composer validate image-templates/azl3-x86_64-edge-raw.yml

# Display version information
./os-image-composer version

# Install shell completion for your current shell
./os-image-composer install-completion
```

### Commands

The Image Composer Tool provides the following commands:

#### build

Builds a Linux distribution image based on the specified image template file:

```bash
./os-image-composer build [flags] TEMPLATE_FILE
```

Flags:

- `--workers, -w`: Number of concurrent download workers (overrides the configuration file)
- `--cache-dir, -d`: Package cache directory (overrides the configuration file)
- `--work-dir`: Working directory for builds (overrides the configuration file)
- `--verbose, -v`: Enable verbose output
- `--dotfile, -f`: Generate dependency graph as a dot file
- `--config`: Path to the configuration file
- `--log-level`: Log level (debug, info, warn, and error)

Example:

```bash
./os-image-composer build --workers 12 --cache-dir ./package-cache image-templates/azl3-x86_64-edge-raw.yml
```

#### config

Manages the global configuration:

```bash
# Show current configuration
./os-image-composer config show

# Initialize new configuration file
./os-image-composer config init [config-file]
```

#### validate

Validates a YAML template file against the schema without building an image:

```bash
./os-image-composer validate TEMPLATE_FILE
```

This is useful for verifying template configurations before starting the potentially time-consuming build process.

#### version

Displays the tool’s version and information:

```bash
./os-image-composer version
```

Shows the version number, build date, and Git commit SHA.

#### install-completion

Installs the shell completion feature for your current shell or a specified shell:

```bash
# Auto-detect shell
./os-image-composer install-completion

# Specify shell type
./os-image-composer install-completion --shell zsh

# Force overwrite existing completion
./os-image-composer install-completion --force
```

Reload your shell configuration based on the shell that you are using:
Bash:

```bash
source ~/.bashrc
```

Zsh:

```bash
source ~/.zshrc
```

Fish: (Nothing needed, it should work immediately)

PowerShell:

```powershell
. $PROFILE
```

Test the completion:

```bash
os-image-composer [TAB]
os-image-composer b[TAB]
os-image-composer build --[TAB]
```

See the [Shell Completion](#shell-completion) section for more details.

### Image Template Format

Image templates are written in the YAML format and define the requirements for building a specific OS image. The template structure allows users to define key parameters such as the OS distribution, version, architecture, software packages, output format, and kernel configuration.

```yaml
image:
  name: azl3-x86_64-edge
  version: "1.0.0"

target:
  os: azure-linux    # Target OS name
  dist: azl3          # Target OS distribution
  arch: x86_64        # Target OS architecture
  imageType: raw      # Image type: raw, iso, img, vhd

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

#### Key Components

##### 1. `image`

**Description:** Basic image identification and metadata.
- `name`: Name of the resulting image
- `version`: Version for tracking and naming

##### 2. `target`

**Description:** Defines the target OS and image configuration.
- `os`: Target OS (`azure-linux`, `emt`, and `elxr`)
- `dist`: Distribution identifier (`azl3`, `emt3`, and `elxr12`)
- `arch`: Target architecture (`x86_64`and `aarch64`)
- `imageType`: Output format (`raw`, `iso`, `img`, and `vhd`)

##### 3. `systemConfigs`

**Description:** Array of system configurations that define what goes into the image.
- `name`: Configuration name
- `description`: Human-readable description
- `packages`: List of packages to include in the OS build
- `kernel`: Kernel configuration with version and command-line parameters

#### Supported Distributions

| OS | Distribution | Version | Provider |
|----|-------------|---------|----------|
| azure-linux | azl3 | 3 | AzureLinux3 |
| emt | emt3 | 3.0 | EMT3.0 |
| elxr | elxr12 | 12 | eLxr12 |

#### Package Examples

Common packages that can be included:
- `cloud-init`: For initializing cloud instances
- `python3`: The Python 3 programming language interpreter
- `rsyslog`: A logging system for Linux OS
- `openssh-server`: SSH server for remote access
- `docker-ce`: Docker container runtime

The image template format is validated against a JSON schema to ensure correctness before building.

### Shell Completion Feature

The os-image-composer CLI supports shell auto-completion for Bash, Zsh, Fish, and PowerShell command-line shells. This feature helps users discover and use commands and flags more efficiently.

#### Generate Completion Scripts

```bash
# Bash
./os-image-composer completion bash > os-image-composer_completion.bash

# Zsh
./os-image-composer completion zsh > os-image-composer_completion.zsh

# Fish
./os-image-composer completion fish > os-image-composer_completion.fish

# PowerShell
./os-image-composer completion powershell > os-image-composer_completion.ps1
```

#### Install Completion Scripts

**Bash**:

```bash
# Temporary use
source os-image-composer_completion.bash

# Permanent installation (Linux)
sudo cp os-image-composer_completion.bash /etc/bash_completion.d/
# or add to your ~/.bashrc
echo "source /path/to/os-image-composer_completion.bash" >> ~/.bashrc
```

**Zsh**:

```bash
# Add to your .zshrc
echo "source /path/to/os-image-composer_completion.zsh" >> ~/.zshrc
# Or copy to a directory in your fpath
cp os-image-composer_completion.zsh ~/.zfunc/_os-image-composer
```

**Fish**:

```bash
cp os-image-composer_completion.fish ~/.config/fish/completions/os-image-composer.fish
```

**PowerShell**:

```powershell
# Add to your PowerShell profile
echo ". /path/to/os-image-composer_completion.ps1" >> $PROFILE
```

After installing, you can use tab completion to navigate commands, flags, and arguments when using the ICT.

#### Examples of Completion Script in Action

Once the completion script is installed:

```bash
# Tab-complete commands
./os-image-composer <TAB>
build      completion  config     help       validate    version

# Tab-complete flags
./os-image-composer build --<TAB>
--cache-dir  --config    --help       --log-level  --verbose    --work-dir   --workers

# Tab-complete YAML files for template file argument
./os-image-composer build <TAB>
# Will show YAML files in the current directory
```

The tool is specifically configured to suggest YAML files when completing the template file argument for the build and validate commands.

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

## Get Help

- **Quick Reference**: Run `./os-image-composer --help` to see all available commands and options
- **Complete Guide**: See the [CLI Specification](./docs/architecture/os-image-composer-cli-specification.md) for detailed documentation
- **Examples**: Check the [template examples](#template-examples) section below
- **Troubleshooting**: Refer to the [Build Process documentation](./docs/architecture/os-image-composer-build-process.md#troubleshooting-build-issues)

## Contribute

## License Information
