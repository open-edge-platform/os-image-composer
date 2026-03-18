# OIC vs mkosi v25: Image Build Benchmark

Side-by-side comparison of **OS Image Composer (OIC)** and **mkosi v25** building
identical Ubuntu 24.04 (noble) x86_64 minimal raw images.

Both tools produce the same 4-partition GPT image with the same 209 packages at
identical versions. The key differentiator is **build time** and **configuration
complexity**.

## Benchmark Results

Tested on Intel NUC (Ubuntu 22.04 host), cold builds (no cache), same network.

### Build Time

| | OIC | mkosi v25 | Delta |
|-|-----|-----------|-------|
| **Cold build** | **4m 12s** | 5m 14s | OIC **20% faster** |

OIC is faster primarily because it uses direct chroot + losetup, whereas mkosi v25
downloads a Debian testing tools tree (for systemd-repart v256) and uses user
namespace isolation, adding overhead.

### Image Equivalence

Both images are functionally identical:

| Metric | OIC | mkosi v25 |
|--------|-----|-----------|
| Total packages | 209 | 213 |
| Common packages | 209 | 209 |
| **Version match** | **100% (209/209)** | **100% (209/209)** |
| mkosi extras | - | 4 (`gawk`, `libmpfr6`, `libsigsegv2`, `systemd-boot`)\* |
| Kernel | `linux-image-generic-hwe-24.04` | `linux-image-generic-hwe-24.04` |
| Bootloader | systemd-boot (UKI via dracut) | systemd-boot (UKI via dracut) |
| Init system | systemd | systemd |

\*`gawk` + deps are explicitly added for dracut UKI generation; `systemd-boot` is
listed explicitly in mkosi config (OIC pulls it as a dependency).

### Image Size

| Metric | OIC | mkosi v25 |
|--------|-----|-----------|
| File size (apparent/sparse) | 6.0 GiB | 5.4 GiB |
| Disk usage (actual) | 1.7 GiB | 1.0 GiB |

Both images are sparse. mkosi's smaller actual size comes from more aggressive
sparse allocation by systemd-repart.

### Partition Layout

| # | OIC | mkosi v25 |
|---|-----|-----------|
| 1 | `boot` (EF00), 512 MiB vfat | `esp` (EF00), 512 MiB vfat |
| 2 | `rootfs` (8304), 3.9 GiB ext4 | `root-x86-64` (8304), 3.9 GiB ext4 |
| 3 | `roothashmap` (8300), 500 MiB ext4 | `roothashmap` (8300), 500 MiB ext4 |
| 4 | `userdata` (8300), 1.1 GiB ext4 | `userdata` (8300), 512 MiB ext4 |

Partition type codes match. Name and size differences are cosmetic (OIC default
userdata is larger).

### Configuration Complexity

| Metric | OIC | mkosi v25 |
|--------|-----|-----------|
| User config files | **1 YAML** | 13 files |
| Lines of config | ~50 | ~120 |
| Partition definition | Declarative in YAML | `mkosi.repart/*.conf` drop-ins |
| Default handling | Auto-merged from `config/osv/` | Manual in mkosi.conf |
| Post-install hooks | Built-in (provider) | `mkosi.postinst.chroot` script |
| Proxy config | Env vars (`-E`) | Skeleton + sandbox apt.conf drop-ins |

### Feature Comparison

| Feature | OIC | mkosi v25 |
|---------|-----|-----------|
| SBOM generation | SPDX JSON (auto) | JSON manifest |
| Image compression | gz (configurable) | xz, zstd, etc. |
| Multi-OS support | 5 distros (azl, elxr, emt, rcd, ubuntu) | Generic (Ubuntu/Debian/Fedora/etc.) |
| Reproducibility | Depends on mirror state | `--seed` + `SOURCE_DATE_EPOCH` |
| Caching | Built-in package cache | `--incremental` / tools tree cache |
| Build isolation | chroot | User namespaces (unprivileged) |

## mkosi v25 Setup

### Prerequisites

- **mkosi v25.3**:
  ```bash
  pip3 install --user git+https://github.com/systemd/mkosi.git@v25.3
  ```
- **Root privileges** for image assembly
- `/dev/full` must exist (create if missing: `sudo mknod /dev/full c 1 7 && sudo chmod 666 /dev/full`)
- Current `debian-archive-keyring` (≥ 2025.1) for the `ToolsTree=default` download

### Proxy Note

The `mkosi.skeleton/`, `mkosi.extra/`, and `mkosi.sandbox/` directories contain
placeholder proxy configuration (`proxy.example.com`). Replace the hostnames and
ports in all `90proxy` files with your actual proxy, or remove them if building on
a direct internet connection.

### Build

```bash
cd docs/benchmarks/mkosi-v25

# Build the image (all 4 partitions created natively by systemd-repart)
sudo -E mkosi -f build

# Preview configuration without building
mkosi summary
```

Output: `minimal-os-image-ubuntu_24.04.raw` (~5.4 GiB sparse, ~1 GiB actual).

### Write to Disk / Boot

```bash
# Write to a USB/NVMe device
sudo dd if=minimal-os-image-ubuntu_24.04.raw of=/dev/sdX bs=4M status=progress

# Boot in QEMU
qemu-system-x86_64 -m 2G -bios /usr/share/ovmf/OVMF.fd \
    -drive file=minimal-os-image-ubuntu_24.04.raw,format=raw \
    -nographic
```

## OIC Setup

The OIC template is modified to disable gz compression (default), so both tools
output uncompressed raw images for a fair timing comparison. The full `disk` section
is included (since it fully replaces defaults during merge) with `compression` omitted.

```bash
# From the repo root
sudo -E ./build/os-image-composer build image-templates/ubuntu24-x86_64-minimal-raw.yml
```

Output: `workspace/ubuntu-ubuntu24-x86_64/imagebuild/minimal/minimal-os-image-ubuntu-24.04.raw`

## Mapping: OIC Template → mkosi v25

| OIC template field | mkosi v25 equivalent |
|---|---|
| `target.os: ubuntu` | `Distribution=ubuntu` |
| `target.dist: ubuntu24` (noble) | `Release=noble` |
| `target.arch: x86_64` | `Architecture=x86-64` |
| `target.imageType: raw` | `Format=disk` |
| `disk.partitions` (4 partitions) | `mkosi.repart/*.conf` |
| `systemConfig.bootloader: systemd-boot` | `Bootable=yes`, `Bootloader=systemd-boot` |
| `systemConfig.packages` | `Packages=...` in `mkosi.conf` |
| `kernel.packages` | `linux-image-generic-hwe-24.04` in `Packages=` |
| `kernel.cmdline` | `KernelCommandLine=...` |
| Package repos (noble + updates + security) | `mkosi.skeleton/` sources.list + mkosi native repos |
| Network config (DHCP) | `mkosi.extra/etc/systemd/network/dhcp.network` |
| Auto-login | `Autologin=yes` + getty drop-ins in `mkosi.extra/` |
| SBOM / manifest | `ManifestFormat=json` → `*.manifest` |

## Directory Structure

```
docs/benchmarks/mkosi-v25/
├── mkosi.conf                    # Main config (distro, packages, boot, output)
├── mkosi.postinst.chroot         # Post-install (enables services, passwd, fstab)
├── mkosi.repart/                 # systemd-repart partition definitions
│   ├── 00-esp.conf               #   EFI System Partition (512M, vfat)
│   ├── 10-root.conf              #   Root filesystem (4044M, ext4)
│   ├── 20-roothashmap.conf       #   Roothashmap (500M, ext4)
│   └── 30-userdata.conf          #   Userdata → /opt (512M+, ext4)
├── mkosi.skeleton/               # Files injected BEFORE package installation
│   └── etc/apt/
│       ├── apt.conf.d/90proxy
│       └── sources.list.d/ubuntu-noble.list
├── mkosi.extra/                  # Files overlaid into the final image
│   └── etc/
│       ├── apt/
│       │   ├── apt.conf.d/90proxy
│       │   └── sources.list.d/ubuntu-noble.list
│       └── systemd/
│           ├── network/dhcp.network
│           └── system/
│               ├── getty@.service.d/autologin.conf
│               └── serial-getty@.service.d/autologin.conf
├── mkosi.sandbox/                # Host-side sandbox proxy
│   └── etc/apt/apt.conf.d/90proxy
└── README.md
```

## Troubleshooting

| Issue | Fix |
|-------|-----|
| `/dev/full` missing | `sudo mknod /dev/full c 1 7 && sudo chmod 666 /dev/full` |
| GPG errors fetching tools tree | Update `debian-archive-keyring` to ≥ 2025.1 |
| `systemctl` fails in postinst | Ensure file is named `mkosi.postinst.chroot` (`.chroot` suffix required) |
| `systemd-repart` too old (< 254) | Keep `ToolsTree=default`; it downloads repart v256 from Debian testing |
| Proxy issues | Verify `90proxy` exists in `mkosi.skeleton/`, `mkosi.extra/`, and `mkosi.sandbox/` |
