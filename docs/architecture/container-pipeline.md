# Multi-Stage and Multi-Container OS Image Composer

## Introduction: Why Multi-Stage and Multi-Container OS Builds

Building a modern Linux operating system distribution is no longer a simple linear process. It requires dependency isolation, reproducibility, toolchain flexibility, and support for secure artifacts like signed RPMs, Secure Boot Unified Kernel Images (UKIs), and verifiable system images. A multi-stage, multi-container build pipeline is the industry-proven approach used by modern Linux projects such as Fedora CoreOS, Flatcar, CBL-Mariner (Azure Linux), and Talos Linux.

In this approach, the OS is not built in a single environment. Instead, each stage of the build process — such as toolchain setup, root filesystem creation, package management, image assembly, and secure artifact signing — is executed inside separate, purpose-built containers. Each container has only the tools required for its stage. 

For example, just concept wide:

```
                   +-------------------------------------------+
                   |          Build Host (Any Linux)           |
                   |   (Ubuntu, RHEL, Debian – supports CI)    |
                   +-----------------------+-------------------+
                                           |
   -----------------------------------------------------------------------------------------------------------------------------------
   |                         |                           |                           |                                      |
+--------------------------------+ +--------------------------------+ +--------------------------------+ +--------------------------------+
| Container #1                  | | Container #2                   | | Container #3                   | | Container #4                   |
| Package Fetch Stage           | | RootFS Build Stage             | | System Config Stage            | | UKI Build & Secure Boot Stage |
| (Download Debian/RPM          | | (debootstrap or rpm + chroot)  | | (Immutable + overlay setup)    | | (ukify + sign)                |
|  packages to local cache)     | |                                | | (/etc, /var, /home persistence)| | (EFI+initrd+kernel packaged)  |
+--------------------------------+ +--------------------------------+ +--------------------------------+ +--------------------------------+
                                           |                           |                           |
                                           ------------------------------------------------------------------
                                                                 |
                                                     +-----------------------------+
                                                     |     Container #5            |
                                                     |     Image Assembly Stage    |
                                                     | (Partitions + systemd-boot  |
                                                     |    + RAW image output)      |
                                                     +-----------------------------+
                                                                 |
                          ----------------------------------------------------------------------------------
                          Output Artifacts:
                          - Local package cache (.rpm/.deb + dependencies)
                          - Root filesystem directory
                          - Immutable OS root with writable overlays
                          - Signed Unified Kernel Image (UKI)
                          - Bootable RAW disk image

         

```

---

### Problems with Existing Chroot-Based Build System

Our current OS build tooling uses a monolithic workflow where a single chroot workspace is created and all tooling (dnf/yum, mkfs, rpm, UKI tools, signing tools, etc.) is installed inside this same environment. While this works for simple builds, it introduces serious scalability, maintenance, and reliability problems.

#### Key Limitations of Chroot Approach

- **Poor Tool Isolation**
  - All build dependencies must coexist inside the same chroot.
  - Conflicting package requirements (RPM + DEB tools cannot co-exist cleanly).
  - Example: `ukify` cannot be installed in a RHEL-based chroot.

- **Environment Contamination**
  - Installing tools inside the chroot modifies it continuously.
  - Hard to guarantee reproducible builds.
  - Chroot often becomes a "snowball mess" over time.

- **Host Dependency Problems**
  - Chroot still depends heavily on host kernel + userspace compatibility.
  - Differences in host OS versions break chroot behavior.
  - Example: `systemd-nspawn`, loop devices, `mkfs` behavior vary by host.

- **Security Risks**
  - Chroot is not a security boundary.
  - Tools installed inside chroot may accidentally modify host filesystems.
  - No isolation from build host; unsafe for CI.

- **Difficult to Maintain Toolchains**
  - Updating a tool version requires rebuilding and validating the entire chroot.
  - No easy package version pinning inside chroot.
  - Hard to maintain multiple toolchains for different OS targets.

- **Not Scalable for Complex Pipelines**
  - Cannot mix Debian tools in RHEL chroot or vice versa.
  - No modularity—every new build capability adds complexity inside the same environment.
  - No reuse of tooling per stage (rootfs vs. signing vs. UKI vs. ISO build).

---

### Why Containers Solve These Problems

| Problem in Chroot Model | Solved by Multi-Stage Containers |
|------------------------|----------------------------------|
| Single environment polluted by tools | Each stage uses a clean container |
| Hard to cross RPM/DEB tool usage | Use different base images per stage |
| Reproducibility issues | Tool versions locked by container images |
| Unsafe for CI | Containers provide isolation |
| Hard to debug and extend | Modular stages, easier pipeline |
| Host dependency issues | Host only needs Docker/Podman |

---

### Conclusion

> Chroot-based builds are fragile, hard to maintain, and unsuitable for secure, reproducible OS build pipelines. Containers provide isolation, cross-platform tool support, reproducibility, security, and clean pipeline modularity. This makes them the modern industry standard for OS image composition (used by Fedora CoreOS, Flatcar, Talos Linux, CBL-Mariner, and Ubuntu Core pipelines).


### ✅ Key Advantages of Multi-Stage Containers

- **No Host Dependency Issues**
  - Each build stage runs inside its own container with the correct toolchain.
  - Example: Rootfs uses RHEL/Fedora-based container with `dnf`; UKI uses Debian container with `ukify`.

- **Guaranteed Tool Availability**
  - All required build tools exist inside their container stage.
  - Host never needs `dnf`, `debootstrap`, `dracut`, `ukify`, or `sbsign` installed.

- **Supports Cross-Distro Pipelines**
  - Can mix **RPM-based target OS** with **Debian-based UKI tooling** in different build stages.
  - No longer blocked by missing cross-distro packages.

- **Reproducible and Deterministic**
  - Container versions freeze build dependencies.
  - Ensures consistent builds across developers and CI pipelines.

- **Clean Separation of Responsibilities**
  - Each build step has a dedicated container:
    - Package download/cache
    - Rootfs assembly
    - OS configuration (immutable setup, overlays, fstab, users)
    - Kernel + UKI build & signing
    - Raw disk image creation

- **Security and Compliance Friendly**
  - Reduced attack surface compared to installing tools on host.
  - Easier SBOM tracking and supply chain auditing.

- **CI/CD Friendly and Scalable**
  - Works with GitLab CI, GitHub Actions, Jenkins, etc.
  - Only requirement: Docker or Podman available on build runner.

---

### ✅ Example Problem Solved: `ukify` Missing on RPM Hosts

| Single Host Build (Current) | Multi-Stage Container Build (Proposed) |
|-----------------------------|----------------------------------------|
| UKI build fails because `ukify` not available on RHEL host | UKI stage uses Debian container with `ukify` installed |
| Requires mixing DEB and RPM tools on host | No cross-tool contamination; clean separation |
| Host dependency hell | Host only runs containers |
| Inconsistent developer environments | Identical builds across all machines |

---

### ✅ Summary

> The multi-stage container approach guarantees a portable, reproducible, host-independent OS build pipeline and eliminates tool conflict issues permanently.




