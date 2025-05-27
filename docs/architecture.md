# Image Composer Architecture

## Introduction

The image composer is intended to be a generic toolkit to build operating system
images from pre-built artifacts such as `rpm`, `deb` supporting a range of
common OS for the distributed edge. You can customize the content of the OS to
suit your specific needs based on the type of application and workloads. The
image composer will first be targeting to support:

1. Edge Microvisor Toolkit (EMT) 3.0
1. AzureLinux 3.0
1. WindRiver eLxr

The image composer generates the desired image based on a JSON input which
defines what the target distribution, version, packages and kernel to include
and can generate both immutable and mutable images supporting bare metal and
VM deployments with ISO installers, raw image files and VHD (Virtual Hard
Drive).
## Image Composer Tool System Network Context

Image Composer Tool network context is shown in the following diagram.
![Image Composer Network Diagram](assets/image-composer-network-diagram.drawio.svg) illustrates how different components of the product's system architecture communicate with each other. This type of diagram is useful for technical documentation, infrastructure planning, security review and troubleshooting.

### Network Security Considerations
Image Composer Tool downloads required packages using HTTP requests to the distribution specific package repos over TLS 1.2+ connections. Each of the package repos does server side validation on the package download requests. So it is expected that the system running the image composer tool is provisioned with a CA root chain.
 
 ### Package Sign Verification
 All the downloaded packages are integrity verified during download using GPG Public Keys and SHA256/MD5 checksum published at the package repos.

## Architecture Overview

The following diagram outlines the high level architectural breakdown of the
image composer ![Architecture Overview](assets/architecture.drawio.svg)

The host system should be running a validated and supported Linux distribution
such as Ubuntu 24.04. The image-composer implements all of the common business
logic which remains the same regardless of which distribution you are building
an image for. Providers exists for the supported Operating Systems with a
interface definition that each provider needs to implement to decouple
distribution specific functionality from the core and common business logic.

The image composer generates a `chroot` environment which is used for
the image composition and creation process which is isolated from the host
operating file system. Packages are fetched, with help from a provider, and
cached locally before the image is being created.

The image composer and its providers use several common `modules` across such
as logging, image creation tools for the supported output formats. Configuration
specific information for the supported distributions are held in a configuration
manifest that holds distribution specific information required to build an image.

## Operational Flow

The following flow illustrates the overall image composition flow.

```mermaid
flowchart TD

    A[User JSON Input Configuration] --> B[Input Validation & JSON Schema]
    B --> C[Package Fetching Module]
    C --> D[Dependency Resolution Module]
    D --> E[Image Assembly Module]
    E --> F[Post-Processing Module]
    F --> G[Final Immutable Image]

    %% Distribution Specific Modules
    C -- "Fetch packages from" --> H[OS-Specific Package Sources]
    D -- "Resolve dependencies for" --> I[OS-Specific Dependency Logic]
    E -- "Kernel & Bootloader Integration" --> J[OS-Specific Assembly]
    F -- "Apply custom settings" --> K[OS-Specific Configurations]

    %% Cross-Cutting Concerns
    G --> L[Validation & Testing]
    L --> M[Automated Unit/Integration/System Tests]

    %% Metadata & Security
    E -.-> N[Versioning & SBOM Generation]
    N -.-> G

    %% Logging & Error Handling (applies across modules)

    O[Logging & Error Handling]
    O --- C
    O --- D
    O --- E
    O --- F
```
## Detailed Design 

The following diagram depicts the list of SW entities and their relationships in composing user specified OS image.

```mermaid

classDiagram
    note "SW Entities and their relationships"
    CLI -- OS Image Config File Parser
    CLI -- Config Store 
    CLI -- Image Store
    CLI -- OS Image Provider If
    OS Image Provider If -- Debian Package Handler
    OS Image Provider If -- RPM Package Handler
    OS Image Provider If -- Boot Image Composer
    OS Image Provider If --|> Azure Linux Provider
    OS Image Provider If --|> Wind River Elxr Provider
    OS Image Provider If --|> EMT Provider
    Debian Package Handler -- Debian Package Cache
    RPM Package Handler -- RPM Package Cache
    Boot Image Composer --|> Systemd Boot Composer
    OS Image Provider If -- Image Formatter
    Image Formatter --|> ISO Image Generator
    Image Formatter --|> VHD Image Generator
    Image Formatter --|> Raw Image Generator
    Azure Linux Provider o-- Chroot Environment
    Wind River Elxr Provider o-- Chroot Environment
    EMT Provider o-- Chroot Environment
```

The following table captures the SW entities and description of their functions.

| SW Component | Description |
| -------- | ------- |
| CLI   | A commandline interface module that provides an interface enabling users to create desired OS images |
| OS Image Provider Interface | As described above |
| Debian Package Handler |  for downloading, caching and serving the packages as needed to the OS Image providers  |
| Debian Package Cache |  for storing and retrieving the downloaded debian packages in an efficient way  |
| RPM Package Handler |  for downloading, caching and serving the packages as needed to the OS Image providers  |
| RPM Package Cache |  for storing and retrieving the downloaded rpm packages in an efficient way  |
| Azure Linux Provider | for supporting creation of Azure Linux OS Images  |
| EMT Provider | for supporting creation of EMT OS Images  |
| Windriver LxR Provider |  for supporting creation of WindRiver LxR Images  |
| Chroot Environment |  for creation of chroot environment upon request, which would eventually be used to build the OS image|
| Boot Image Composer |  for creation of Boot Image upon request by the OS provider to bundle boot images with kernel images|
| Systemd-Boot Composer | for creation of Systemd-Boot Image upon request by the Boot Image Composer|
| ISO Maker | Converts an image in chroot environment into an ISO Image|
| Raw Maker | Converts an image in chroot environment into raw format|
| VHD Maker | Converts an image in chroot environment into VHD format|
| Image Store | Stores a limited set of images using LRU principle|
| Config Store | Stores Image Configuration Files for runtime reference and later use|

## Architecture Opens
|S.No | Architecture Open | Status | Comments|
| ----|------------------ | ------- |--------|
| 1.| Development Ownership of each component  | Open |  |
| 2.| JSON vs YML file for capturing the OS Image Configuration  | Open |  |
| 3.| User Facing CLI Commands and user interface  | Open |  |
