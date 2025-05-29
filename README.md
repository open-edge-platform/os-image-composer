# Image Composer Tool

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Lint Check](https://github.com/open-edge-platform/image-composer/actions/workflows/go-lint.yml/badge.svg)](https://github.com/open-edge-platform/image-composer/actions/workflows/go-lint.yml)

The Image Composer Tool (ICT) is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).

The ICT is developed in `golang` and is initially targeting to build custom
images for [EMT](https://github.com/open-edge-platform/edge-microvisor-toolkit)
(Edge Microvisor Toolkit), [Azure Linux](https://github.com/microsoft/azurelinux)
and WindRiver Linux.

## Get Started

### Prerequisites

Image Composer Tool is developed in golang and requires installation of go version 1.22.12+. Instructions to install for a specific distribution can be found [here](https://go.dev/doc/manage-install).

### Building

Build the image-composer using Go directly:

```bash
go build ./cmd/image-composer
```

Or use Earthly for a reproducible build:

```bash
# Default build
earthly +build

# Build with specific version
earthly +build --version=1.0.0
```

The Earthly build automatically includes:

- Version number (from --version parameter)
- Build date (current UTC date)
- Git commit SHA (current repository commit)

## Configuration

### Global Configuration

Image Composer Tool supports global configuration files to set tool-level parameters that apply across all image builds. Image-specific parameters should still be defined in the JSON specification files.

#### Configuration File Locations

The tool searches for configuration files in the following order:

1. `image-composer.yaml` (current directory)
2. `image-composer.yml` (current directory)
3. `.image-composer.yaml` (hidden file in current directory)
4. `~/.image-composer/config.yaml` (user home directory)
5. `~/.config/image-composer/config.yaml` (XDG config directory)
6. `/etc/image-composer/config.yaml` (system-wide)

#### Configuration Parameters

```yaml
# Core tool settings
workers: 12                              # Number of concurrent download workers (1-100, default: 8)
cache_dir: "/var/cache/image-composer"   # Package cache directory (default: ./cache)
work_dir: "/tmp/image-composer"          # Working directory for builds (default: ./workspace)
temp_dir: ""                             # Temporary directory (empty = system default)

# Logging configuration
logging:
  level: "info"                          # Log level: debug, info, warn, error (default: info)
```

#### Configuration Management Commands

```bash
# Create a new configuration file
./image-composer config init

# Create config file at specific location
./image-composer config init /path/to/config.yaml

# Show current configuration
./image-composer config show

# Use specific configuration file
./image-composer --config /path/to/config.yaml build spec.json
```

### Usage

The Image Composer Tool uses a command-line interface with various commands:

```bash
# Show help
./image-composer --help

# Build command with spec file as positional argument
./image-composer build testdata/valid.json

# Override config settings with command-line flags
./image-composer build --workers 16 --cache-dir /tmp/cache testdata/valid.json

# Validate a spec file against the schema
./image-composer validate testdata/valid.json

# Display version information
./image-composer version

# Install shell completion for your current shell
./image-composer install-completion
```

### Commands

The Image Composer Tool provides the following commands:

#### build

Builds a Linux distribution image based on the specified spec file:

```bash
./image-composer build [flags] SPEC_FILE
```

Flags:

- `--workers, -w`: Number of concurrent download workers (overrides config file)
- `--cache-dir, -d`: Package cache directory (overrides config file)
- `--work-dir`: Working directory for builds (overrides config file)
- `--verbose, -v`: Enable verbose output
- `--dotfile, -f`: Generate dependency graph as a dot file
- `--config`: Path to configuration file
- `--log-level`: Log level (debug, info, warn, error)

Example:

```bash
./image-composer build --workers 12 --cache-dir ./package-cache testdata/valid.json
```

#### config

Manages global configuration:

```bash
# Show current configuration
./image-composer config show

# Initialize new configuration file
./image-composer config init [config-file]
```

#### validate

Validates a JSON spec file against the schema without building an image:

```bash
./image-composer validate SPEC_FILE
```

This is useful for verifying configurations before starting the potentially time-consuming build process.

#### version

Displays version information about the tool:

```bash
./image-composer version
```

This shows the version number, build date, and Git commit SHA.

#### install-completion

Installs shell completion for your current shell or a specified shell:

```bash
# Auto-detect shell
./image-composer install-completion

# Specify shell type
./image-composer install-completion --shell zsh

# Force overwrite existing completion
./image-composer install-completion --force
```

Reload your shell configuration:
Depending on which shell you're using:

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
image-composer [TAB]
image-composer b[TAB]
image-composer build --[TAB]
```

See the [Shell Completion](#shell-completion) section for more details.

### User Input JSON

This section provides a detailed explanation of the JSON input structure used to configure and build a Linux-based operating system. The JSON format allows users to define key parameters such as the operating system distribution, version, architecture, software packages, output format, immutability, and kernel configuration. By customizing these parameters, users can create tailored Linux OS builds, including distributions like Ubuntu, Wind River, and Edge Microvisor Toolkits.

```json
{
    "distro": "eLxr",
    "version": "12",
    "arch": "x86_64",
    "packages": [
        "cloud-init",        
        "cloud-utils-growpart",
        "dhcpcd",        
        "grubby",
        "hyperv-daemons",
        "netplan",        
        "python3",
        "rsyslog",
        "sgx-backwards-compatibility",
        "WALinuxAgent",        
        "wireless-regdb"        
    ],
    "immutable": true,
    "output": "iso",
    "kernel": {
      "version": "5.10.0",
      "cmdline": "quiet splash"
    }
}
```

#### Key Components

##### 1. `distro`

**Description:** Specifies the target Linux distribution to be built.  
**Examples:**

- `AzureLinux`
- `eLxr`

##### 2. `version`

**Description:** Defines the version of the target operating system.  
**Example:** `"12"`

##### 3. `arch`

**Description:** Specifies the architecture of the target operating system.  
**Examples:**

- `x86_64`
- `arm64`

##### 4. `packages`

**Description:** A list of software packages to be included in the OS build. These packages will be pre-installed in the resulting image.  
**Examples:**

- `cloud-init`: Used for initializing cloud instances.
- `python3`: The Python 3 programming language interpreter.
- `rsyslog`: A logging system for Linux.

##### 5. `immutable`

**Description:** Indicates whether the operating system should be immutable.  
**Values:**

- `true`: The OS is immutable, meaning it cannot be modified after creation.
- `false`: The OS is mutable, allowing modifications after creation.

##### 6. `output`

**Description:** Specifies the format of the output build.  
**Values:**

- `iso`: The OS will be built as an ISO file, suitable for installation or booting.
- `raw`: The OS will be built as a raw disk image, useful for direct disk writing.
- `vhd`: The OS will be built as a VHD (Virtual Hard Disk) file, commonly used in virtual environments.

##### 7. `kernel`

**Description:** Defines the kernel version and allows customization of the kernel command line.  
**Attributes:**

- `version`: Specifies the kernel version to be used.  
  **Example:** `"5.10.0"`
- `cmdline`: Provides additional kernel command-line parameters.  
  **Example:** `"quiet splash"`

Run the sample JSON files against the defined [schema](schema/os-image-composer.schema.json).
There are two sample JSON files, one [valid](/testdata/valid.json) and one with
[invalid](testdata/invalid.json) content.

### Shell Completion

The image-composer CLI supports shell auto-completion for Bash, Zsh, Fish, and PowerShell. This feature helps users discover and use commands and flags more efficiently.

#### Generating Completion Scripts

```bash
# Bash
./image-composer completion bash > image-composer_completion.bash

# Zsh
./image-composer completion zsh > image-composer_completion.zsh

# Fish
./image-composer completion fish > image-composer_completion.fish

# PowerShell
./image-composer completion powershell > image-composer_completion.ps1
```

#### Installing Completion Scripts

**Bash**:

```bash
# Temporary use
source image-composer_completion.bash

# Permanent installation (Linux)
sudo cp image-composer_completion.bash /etc/bash_completion.d/
# or add to your ~/.bashrc
echo "source /path/to/image-composer_completion.bash" >> ~/.bashrc
```

**Zsh**:

```bash
# Add to your .zshrc
echo "source /path/to/image-composer_completion.zsh" >> ~/.zshrc
# Or copy to a directory in your fpath
cp image-composer_completion.zsh ~/.zfunc/_image-composer
```

**Fish**:

```bash
cp image-composer_completion.fish ~/.config/fish/completions/image-composer.fish
```

**PowerShell**:

```powershell
# Add to your PowerShell profile
echo ". /path/to/image-composer_completion.ps1" >> $PROFILE
```

After installing, you can use tab completion to navigate commands, flags, and arguments when using the image-composer tool.

#### Examples of Completion in Action

Once completion is installed:

```bash
# Tab-complete commands
./image-composer <TAB>
build      completion  config     help       validate    version

# Tab-complete flags
./image-composer build --<TAB>
--cache-dir  --config    --help       --log-level  --verbose    --work-dir   --workers

# Tab-complete JSON files for spec file argument
./image-composer build <TAB>
# Will show JSON files in the current directory
```

The tool is specifically configured to suggest JSON files when completing the spec file argument for the build and validate commands.

## Getting Help

Run `./image-composer --help` to see all available commands and options.

## Contributing

## License Information
