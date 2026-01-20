# Edge Developer Kit Image Templates

This directory contains image templates for building Ubuntu 24.04 images pre-configured for Intel Edge Device Qualification, eliminating the need to run the `main_installer.sh` script from [edge-developer-kit-reference-scripts](https://github.com/open-edge-platform/edge-developer-kit-reference-scripts).

## Available Templates

| Template | Platform | Kernel | Description |
|----------|----------|--------|-------------|
| `ubuntu24-x86_64-edge-developer-kit-raw.yml` | Standard Intel (Xeon, Atom, Core) | HWE 6.14 | Base template with GPU drivers |
| `ubuntu24-x86_64-edge-developer-kit-coreultra-raw.yml` | Intel Core Ultra | HWE 6.14 | Includes NPU support |
| `ubuntu24-x86_64-edge-developer-kit-ptl-raw.yml` | Intel Panther Lake (Core Ultra 3xx) | OEM 6.17 | Includes PTL-specific fixes |

## System Requirements

From the [getting-started.md](https://github.com/open-edge-platform/edge-developer-kit-reference-scripts/blob/main/docs/getting-started.md):

- **RAM**: 16GB minimum
- **Disk Space**: 100GB free recommended
- **BIOS Settings**: Enable "Resizable BAR" or "Above 4G Decoding" for Intel Arc GPUs

## What's Included

All templates include:

### Packages
- **Ubuntu Desktop Minimal** - Full desktop environment
- **Build Essentials** - gcc, g++, make, cmake, pkg-config, git, curl, wget
- **Intel GPU Drivers** - From Kobuk PPA
  - Compute: libze-intel-gpu1, libze1, intel-opencl-icd, clinfo
  - Media: intel-media-va-driver-non-free, vainfo, libvpl2, i965-va-driver
- **Docker** - For AI use cases (OpenWebUI/Ollama)
- **OpenVINO Dependencies** - python3-pip, python3-venv, libtbb12

### Configuration
- User `edgeuser` with sudo, video, render, and docker group access
- GPU udev rules for proper device permissions
- Docker service enabled
- Pre-configured for OpenVINO installation

## Platform Selection Guide

### Standard Intel Platforms (Xeon, Atom, Core non-Ultra)
Use: `ubuntu24-x86_64-edge-developer-kit-raw.yml`
- Intel integrated GPU (Gen9+)
- Intel Arc discrete GPU (DG2/BMG)

### Intel Core Ultra (MTL/ARL)
Use: `ubuntu24-x86_64-edge-developer-kit-coreultra-raw.yml`
- Intel Arc GPU (integrated)
- Intel NPU support
- Meteor Lake / Arrow Lake platforms

### Intel Core Ultra 3xx (PTL - Panther Lake)
Use: `ubuntu24-x86_64-edge-developer-kit-ptl-raw.yml`
- Core Ultra 325U, 355H, 385P, etc.
- Requires OEM 6.17 kernel
- Includes Kisak Mesa PPA
- PTL-specific kernel parameters

## Building an Image

```bash
# Standard platform
os-image-composer build -t image-templates/ubuntu24-x86_64-edge-developer-kit-raw.yml

# Core Ultra platform
os-image-composer build -t image-templates/ubuntu24-x86_64-edge-developer-kit-coreultra-raw.yml

# Panther Lake platform
os-image-composer build -t image-templates/ubuntu24-x86_64-edge-developer-kit-ptl-raw.yml
```

## Post-Deployment Steps

### For Core Ultra / PTL Platforms (NPU Installation)
NPU drivers must be installed after first boot:
```bash
sudo /opt/intel/install-npu-drivers.sh
sudo reboot
```

### OpenVINO Installation
OpenVINO Python packages are installed via:
```bash
sudo /opt/intel/install-openvino.sh
source /opt/intel/activate_openvino.sh
```

### Verification
```bash
# Run the installation verification script
/opt/intel/verify-installation.sh

# Check GPU
clinfo -l
vainfo

# Check NPU (Core Ultra only)
ls -la /dev/accel/accel0

# Check Docker
docker info

# Check OpenVINO
source /opt/intel/activate_openvino.sh
python -c "from openvino import Core; print(Core().available_devices)"
```

## Your First AI Application

After deployment, run the ChatGPT-like interface demo (from getting-started.md):

```bash
# Clone the reference scripts (for use cases)
git clone https://github.com/open-edge-platform/edge-developer-kit-reference-scripts.git
cd edge-developer-kit-reference-scripts/usecases/ai/openwebui-ollama

# Start the AI chat interface
docker compose up -d

# Access at http://localhost
```

## Additional Files

The `edge-developer-kit/` subdirectory contains:

| File | Purpose |
|------|---------|
| `99-intel-gpu.rules` | udev rules for GPU render device permissions |
| `10-intel-vpu.rules` | udev rules for NPU accelerator device |
| `99-ptl-grub.cfg` | PTL-specific GRUB kernel parameters |
| `install-npu-drivers.sh` | NPU driver installation script |
| `install-openvino.sh` | OpenVINO installation script |
| `verify-installation.sh` | Installation verification script (like print_summary_table.sh) |

## Customization

### Adding Custom Packages
Edit the `packages` section in the template YAML.

### Changing Default User
Modify the `users` section:
```yaml
users:
  - name: myuser
    password: "mypassword"
    groups:
      - video
      - render
      - sudo
```

### Adding Custom Files
Use the `additionalFiles` section:
```yaml
additionalFiles:
  - local: "path/to/local/file"
    final: "/path/in/image"
```

## Package Repository Configuration

Before building, update the PPA GPG key URLs in the templates:

```yaml
packageRepositories:
  - codename: "kobuk-intel-graphics"
    url: "https://ppa.launchpadcontent.net/kobuk-team/intel-graphics/ubuntu"
    pkey: "<ACTUAL_GPG_KEY_URL>"  # Replace with actual key URL
    component: "main"
```

To get the GPG key URL for a PPA:
```bash
# Example for kobuk-team PPA
apt-key adv --keyserver keyserver.ubuntu.com --recv-keys <KEY_ID>
```

## Comparison with main_installer.sh

| Feature | main_installer.sh | OS Image Composer |
|---------|------------------|-------------------|
| Deployment time | 30-60 min | 5 min (image flash) |
| Network required | Yes | No (offline capable) |
| Consistency | Varies | Identical every time |
| Rollback | Manual | Keep previous images |
| Customization | Edit scripts | Modify YAML |
