# Image Composer Tool

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

The Image Composer Tool (ICT) is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).

The ICT is developed in `golang` and is initially targeting to build custom
images for [EMT](https://github.com/open-edge-platform/edge-microvisor-toolkit)
(Edge Microvisor Toolkit), [Azure Linux](https://github.com/microsoft/azurelinux)
and WindRiver Linux.

## Get Started

### Building

Build the image-composer

```bash
go build ./cmd/image-composer
```

Run `go test` for the validator

```bash
go test ./internal/validate
```

Run the image composer with the AzureLinux provider. The testdata/valid.json file
is composed to use AzureLinux v3. The provider name is created by `name`+`version`,
so the provider name is `azurelinux3`.

```bash
go run ./cmd/image-composer testdata/valid.json
```

### User Input Json

This section provides an explanation of the JSON input used to configure the
build of a Linux-based operating system. The JSON structure allows users to
specify various parameters, including packages, OS type, output format,
immutability, and kernel version. This configuration can be used to build
different types of Linux OS, such as Ubuntu, Wind River, and Edge Microvisor
Toolkits.

```bash
{
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
    "immutability": ["false"],
    "output": ["iso"],
    "OSType" : ["EdgeMicrovisorToolkit"],
    "kernel": ["6.12"]
}
```

#### Key Components

#### 1. Packages

**Description:** A list of software packages to be added in the OS build that user would like to be pre-built.    
**Example:**

- cloud-init: Used for initializing cloud instances.
- python3: The Python 3 programming language interpreter.
- rsyslog: A logging system for Linux.

#### 2. Immutability

**Description:** Specifies whether the OS should be immutable.
**Value**

- "true": The OS is immutable, meaning it cannot be modified after creation.
- "false": The OS can be modified after creation.

#### 3. Output

**Description:** Defines the format of the output build.
**Value**

- "iso": The OS will be built as an ISO file, suitable for installation or booting.
- "raw": The OS will be built as a raw disk image, useful for direct disk writing.
- "vhd": The OS will be built as a VHD (Virtual Hard Disk) file, often used for virtual environments.

#### 4. OSType

**Description:** Specifies the type of operating system to be built.
**Value**

- "EdgeMicrovisorToolkit": Indicates the build is for Edge Microvisor Toolkits.
- Other possible values could include "Ubuntu", "Wind River", etc.

#### 5. Kernel

**Description:** Specifies the kernel version or type to be used in the OS, and
allows one to customize the kernel command line.

Run the sample JSON files against the defined [schema](schema/os-image-composer.schema.json).
There are two sample JSON files, one [valid](/testdata/valid.json) and one with
[invalid](testdata/invalid.json) content.

## Getting Help

## Contributing

## License Information
