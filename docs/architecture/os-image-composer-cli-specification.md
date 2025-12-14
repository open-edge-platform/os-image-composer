# OS Image Composer CLI Specification

## Table of Contents

- [OS Image Composer CLI Specification](#os-image-composer-cli-specification)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [CLI Flow](#cli-flow)
  - [Usage](#usage)
  - [Global Options](#global-options)
  - [Commands](#commands)
    - [Build Command](#build-command)
    - [Validate Command](#validate-command)
    - [Cache Command](#cache-command)
      - [cache clean](#cache-clean)
    - [Config Command](#config-command)
      - [config init](#config-init)
      - [config show](#config-show)
    - [Version Command](#version-command)
    - [Completion Command](#completion-command)
      - [Generate Completion Scripts](#generate-completion-scripts)
      - [Install Completion Automatically](#install-completion-automatically)
  - [Examples](#examples)
    - [Building an Image](#building-an-image)
    - [Managing Configuration](#managing-configuration)
    - [Managing Cache](#managing-cache)
    - [Validating Templates](#validating-templates)
  - [Configuration Files](#configuration-files)
    - [Global Configuration File](#global-configuration-file)
    - [Image Template File](#image-template-file)
  - [Exit Codes](#exit-codes)
  - [Troubleshooting](#troubleshooting)
    - [Common Issues](#common-issues)
    - [Logging](#logging)
  - [Related Documentation](#related-documentation)

## Overview

`os-image-composer` is a command-line tool for generating custom images for
different operating systems, including
[Azure Linux](https://github.com/microsoft/azurelinux),
[Wind River eLxr](https://www.windriver.com/blog/Introducing-eLxr), and
[Edge Microvisor Toolkit](https://github.com/open-edge-platform/edge-microvisor-toolkit).
The tool provides a flexible approach to creating and configuring
production-ready OS images with precise customization.

OS Image Composer uses a single CLI with subcommands to deliver a consistent
user experience while maintaining flexibility. The tool's architecture is built
around the following files:

1. A global configuration file that defines system-wide settings like cache
   locations and provider configurations
2. Image template files in YAML format that define per-image build requirements

The tool follows a staged build process to support package caching, image
caching, and various customization options that speed up development cycles and
ensure reproducible builds.

## CLI Flow

The following diagram illustrates the high-level flow of the OS Image Composer
CLI, the commands of which begin with `os-image-composer`:

```mermaid
flowchart TD

    Start([os-image-composer]) --> Config[Load Configuration]
    Config --> Commands{Commands}

    Commands -->|build| Build[Build OS Image]
    Build --> ReadTemplate[Read YAML Template]
    ReadTemplate --> BuildProcess[Run Build Pipeline]
    BuildProcess --> SaveImage[Save Output Image]

    Commands -->|validate| Validate[Validate Template File]

    Commands -->|config| ConfigCmd[Manage Configuration]
    ConfigCmd --> ConfigOps[init/show]

    Commands -->|cache| Cache[Manage Cache]
    Cache --> CacheOps[Clean Cache]

    Commands -->|version| Version[Show Version Info]
    
    Commands -->|completion| Completion[Generate/Install Shell Completion]

    %% Styling
    classDef command fill:#b5e2fa,stroke:#0077b6,stroke-width:2px;
    classDef process fill:#f8edeb,stroke:#333,stroke-width:1px;

    class Start command;
    class Build,Validate,ConfigCmd,Cache,Version,Completion command;
    class ReadTemplate,BuildProcess,SaveImage,ConfigOps,CacheOps process;
```

The primary workflow is through the `build` command, which reads an image
template file and runs the build pipeline to create a new image.

See also:

- [Build Stages](./os-image-composer-build-process.md#build-stages-in-detail)
  for the stages of the build pipeline

## Usage

```bash
os-image-composer [global options] command [command options] [arguments...]
```

## Global Options

The OS Image Composer command-line utility uses a layered configuration approach,
with command-line options taking priority over the configuration file settings:

| Option | Description |
|--------|-------------|
| `--config FILE` | Global configuration file. This file contains system-wide settings that apply to all image builds. If not specified, the tool searches for configuration files in standard locations. |
| `--log-level LEVEL` | Log level: debug, info, warn, error (overrides config). Use debug for troubleshooting build issues. |
| `--log-file PATH` | Tee logs to a specific file path (overrides `logging.file` in the configuration). |
| `--help, -h` | Show help for any command or subcommand. |
| `--version` | Show `os-image-composer` version information. |

## Commands

### Build Command

Build an OS image from an image template file. This is the primary command for
creating custom OS images according to your requirements.

```bash
os-image-composer build [flags] TEMPLATE_FILE
```

**Arguments:**

- `TEMPLATE_FILE` - Path to the YAML image template file (required)

**Flags:**

| Flag | Description |
|------|-------------|
| `--workers, -w INT` | Number of concurrent download workers (overrides config). |
| `--cache-dir, -d DIR` | Package cache directory (overrides config). Proper caching significantly improves build times. |
| `--work-dir DIR` | Working directory for builds (overrides config). This directory is where images are constructed before being finalized. |
| `--verbose, -v` | Enable verbose output (equivalent to --log-level debug). Displays detailed information about each step of the build process. |

**Example:**

```bash
# Build an image with default settings
sudo -E os-image-composer build my-image-template.yml

# Build with custom workers and cache directory
sudo -E os-image-composer build --workers 16 --cache-dir /tmp/cache my-image-template.yml

# Build with verbose output
sudo -E os-image-composer build --verbose my-image-template.yml
```

**Note:** The build command typically requires sudo privileges for operations like creating loopback devices and mounting filesystems.

See also:

- [Build Stages in Detail](./os-image-composer-build-process.md#build-stages-in-detail) for information about each build stage
- [Build Performance Optimization](./os-image-composer-build-process.md#build-performance-optimization) for tips to improve build speed

### Validate Command

Validate an image template file without building it. This allows checking for
errors in your template before committing to a full build process.

```bash
os-image-composer validate TEMPLATE_FILE
```

**Arguments:**

- `TEMPLATE_FILE` - Path to the YAML image template file to validate (required)

**Description:**

The validate command performs the following checks:

- YAML syntax validation
- Schema validation against the image template JSON schema
- Required fields verification
- Type checking for all fields

**Example:**

```bash
# Validate a template file
os-image-composer validate my-image-template.yml

# Validate with verbose output
os-image-composer --log-level debug validate my-image-template.yml
```

See also:

- [Validate Stage](./os-image-composer-build-process.md#1-validate-stage)
  for details on the validation process

### Cache Command

Manage cached artifacts created during the build process.

```bash
os-image-composer cache SUBCOMMAND
```

#### cache clean

Remove cached packages or workspace chroot data.

```bash
os-image-composer cache clean [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--packages` | Remove cached packages (default when no scope flags are provided). |
| `--workspace` | Remove cached chroot environments and chroot tarballs under the workspace directory. |
| `--all` | Enable both package and workspace cleanup in a single invocation. |
| `--provider-id STRING` | Restrict cleanup to a specific provider (format: `os-dist-arch`). |
| `--dry-run` | Show what would be removed without deleting anything. |

**Examples:**

```bash
# Remove all cached packages
os-image-composer cache clean

# Remove chroot caches for a single provider
os-image-composer cache clean --workspace --provider-id azure-linux-azl3-x86_64

# Preview everything that would be deleted
os-image-composer cache clean --all --dry-run
```

When no scope flag is supplied, the command defaults to `--packages`.

### Config Command

Manage the global configuration file. The config command provides subcommands
for initializing and viewing configuration.

```bash
os-image-composer config SUBCOMMAND
```

**Subcommands:**

#### config init

Initialize a new configuration file with default values.

```bash
os-image-composer config init [CONFIG_FILE]
```

**Arguments:**

- `CONFIG_FILE` - Path where the configuration file should be created (optional). If not specified, creates the configuration in a standard location.

**Example:**

```bash
# Initialize configuration in current directory
os-image-composer config init os-image-composer.yml

# Initialize in default location
os-image-composer config init
```

#### config show

Show the current configuration settings.

```bash
os-image-composer config show
```

**Example:**

```bash
# Show current configuration
os-image-composer config show

# Show configuration from specific file
os-image-composer --config /path/to/config.yml config show
```

### Version Command

Display the tool's version information, including build date, Git commit SHA, and organization.

```bash
os-image-composer version
```

**Example:**

```bash
os-image-composer version
```

**Output includes:**

- Version number
- Build date
- Git commit SHA
- Organization

### Completion Command

Generate or install shell completion scripts for os-image-composer. Supports bash, zsh, fish, and PowerShell.

**Prerequisites:** The `os-image-composer` binary must be in your system's `$PATH` for completion to function properly. The completion script is registered for the command name `os-image-composer`, not for relative or absolute paths.

#### Generate Completion Scripts

Generate completion scripts to stdout for manual installation:

```bash
os-image-composer completion [bash|zsh|fish|powershell]
```

**Example:**

```bash
# Generate bash completion script
os-image-composer completion bash > /etc/bash_completion.d/os-image-composer

# Generate zsh completion script
os-image-composer completion zsh > ~/.zsh/completion/_os-image-composer
```

#### Install Completion Automatically

Automatically detect shell and install completion scripts:

```bash
os-image-composer completion install [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--shell STRING` | Shell type (bash, zsh, fish, powershell). If not specified, auto-detects current shell. |
| `--force` | Force overwrite existing completion files. |

**Example:**

```bash
# Auto-detect shell and install completion
os-image-composer completion install

# Install completion for specific shell
os-image-composer completion install --shell bash

# Force reinstall
os-image-composer completion install --force
```

**Post-Installation Steps:**

After installing completion, ensure `os-image-composer` is in your PATH, then reload your shell configuration:

**Bash:**

```bash
echo "source ~/.bash_completion.d/os-image-composer.bash" >> ~/.bashrc
source ~/.bashrc
```

**Zsh:**

```zsh
echo 'fpath=(~/.zsh/completion $fpath)' >> ~/.zshrc
source ~/.zshrc
```

**Fish:**
Fish automatically loads completions from the standard location. Just restart your terminal.

**PowerShell:**

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
. $PROFILE
```

## Examples

### Building an Image

```bash
# Build an image with default settings
sudo -E os-image-composer build image-templates/azl3-x86_64-edge-raw.yml

# Build with custom configuration
sudo -E os-image-composer --config=/path/to/config.yaml build image-templates/azl3-x86_64-edge-raw.yml

# Build with custom workers and cache
sudo -E os-image-composer build --workers 12 --cache-dir ./package-cache image-templates/azl3-x86_64-edge-raw.yml

# Build with debug logging
sudo -E os-image-composer --log-level debug build image-templates/azl3-x86_64-edge-raw.yml
```

### Managing Configuration

```bash
# Initialize a new configuration file
os-image-composer config init my-config.yml

# Show current configuration
os-image-composer config show

# Use a specific configuration file
os-image-composer --config /etc/os-image-composer/config.yml build template.yml
```

### Managing Cache

```bash
# Remove all cached packages
os-image-composer cache clean

# Remove workspace chroot caches for a specific provider
os-image-composer cache clean --workspace --provider-id azure-linux-azl3-x86_64

# Preview both package and workspace cleanup without deleting files
os-image-composer cache clean --all --dry-run
```

### Validating Templates

```bash
# Validate a template
os-image-composer validate image-templates/azl3-x86_64-edge-raw.yml

# Validate with debug output
os-image-composer --log-level debug validate image-templates/azl3-x86_64-edge-raw.yml
```

## Configuration Files

### Global Configuration File

The global configuration file (YAML format) defines system-wide settings that
apply to all image builds. The tool searches for configuration files in the following locations (in order):

1. Path specified with `--config` flag
2. `os-image-composer.yml` in current directory
3. `.os-image-composer.yml` in current directory
4. `os-image-composer.yaml` in current directory
5. `.os-image-composer.yaml` in current directory
6. `~/.os-image-composer/config.yml`
7. `~/.os-image-composer/config.yaml`
8. `~/.config/os-image-composer/config.yml`
9. `~/.config/os-image-composer/config.yaml`
10. `/etc/os-image-composer/config.yml`
11. `/etc/os-image-composer/config.yaml`

**Example Configuration:**

```yaml
# Number of concurrent workers for package downloads
workers: 8

# Directory for caching downloaded packages
cache_dir: "./cache"

# Working directory for build process
work_dir: "./workspace"

# Configuration files directory
config_dir: "./config"

# Temporary directory
temp_dir: "/tmp"

# Logging configuration
logging:
  level: "info"  # debug, info, warn, error
```

**Configuration Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `workers` | integer | Number of concurrent download workers (1-100). Default: 8 |
| `cache_dir` | string | Directory for package cache. Default: "./cache" |
| `work_dir` | string | Working directory for builds. Default: "./workspace" |
| `config_dir` | string | Directory for configuration files. Default: "./config" |
| `temp_dir` | string | Temporary directory. Default: system temp directory |
| `logging.level` | string | Log level (debug/info/warn/error). Default: "info" |

### Image Template File

The image template file (YAML format) defines the specifications for a single image build.
With this file, you can define exactly what goes into your custom OS image,
including packages, configurations, and customizations.

**Example Template:**

```yaml
image:
  # Basic image identification
  name: edge-device-image                    # Name of the resulting image
  version: "1.2.0"                           # Version for tracking and naming

target:
  # Target OS and image configuration
  os: azure-linux                            # Base operating system
  dist: azl3                                 # Distribution identifier
  arch: x86_64                               # Target architecture
  imageType: raw                             # Output format (supported: raw, iso only)

systemConfig:
  # System configuration
  name: edge                                 # Configuration name
  description: Edge device image with Microvisor support

  # Package configuration
  packages:                                  # Packages to install
    - openssh-server
    - docker-ce
    - vim
    - curl
    - wget

  # Kernel configuration
  kernel:
    version: "6.12"                          # Kernel version to include
    cmdline: "quiet splash"                  # Additional kernel command-line parameters
```

See also:

- [Common Build Patterns](./os-image-composer-build-process.md#common-build-patterns)
  for example image templates
- [Template Structure](./os-image-composer-templates.md#template-structure)
  for detailed template documentation

## Exit Codes

The tool provides consistent exit codes that can be used in scripting and
automation:

| Code | Description |
|------|-------------|
| 0 | Success: The command completed successfully. |
| 1 | General error: An unspecified error occurred during execution. |

## Troubleshooting

### Common Issues

1. **Disk Space**: Building images requires significant temporary disk space.

   ```bash
   # Check free space in workspace directory
   df -h ./workspace
   
   # Check free space in cache directory
   df -h ./cache
   ```

2. **Permissions**: The build command requires sudo privileges.

   ```bash
   # Run with sudo and preserve environment
   sudo -E os-image-composer build template.yml
   ```

3. **Configuration Issues**: Verify configuration is valid.

   ```bash
   # Show current configuration
   os-image-composer config show
   
   # Initialize with defaults
   os-image-composer config init
   ```

4. **Template Validation Errors**: Validate templates before building.

   ```bash
   # Validate template
   os-image-composer validate template.yml
   ```

### Logging

Use the `--log-level` flag or `--verbose` flag to get more detailed output:

```bash
# Debug logging
os-image-composer --log-level debug build template.yml

# Verbose output (same as debug)
os-image-composer build --verbose template.yml

# Error logging only
os-image-composer --log-level error build template.yml
```

## Related Documentation

- [Build Process](./os-image-composer-build-process.md) - Detailed information about the build stages
- [Templates](./os-image-composer-templates.md) - Template structure and usage
- [Caching](./os-image-composer-caching.md) - How caching works
- [Coding Style](./os-image-composer-coding-style.md) - Development guidelines
