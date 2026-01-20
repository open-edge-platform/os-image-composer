#!/bin/bash
# Intel NPU Driver Installation Script for Core Ultra Platforms
# This script downloads and installs the latest NPU drivers from GitHub
# Run as root: sudo /opt/intel/install-npu-drivers.sh
#
# Based on: npu_installer.sh from edge-developer-kit-reference-scripts
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Status indicators
S_ERROR="[ERROR]"
S_VALID="[✓]"
S_WARNING="[!]"
S_INFO="[INFO]"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_error() { echo -e "${RED}${S_ERROR} $1${NC}"; }
print_success() { echo -e "${GREEN}${S_VALID} $1${NC}"; }
print_warning() { echo -e "${YELLOW}${S_WARNING} $1${NC}"; }
print_info() { echo -e "${S_INFO} $1"; }

# Check if running as root
check_privileges() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Verify this is a Core Ultra platform
verify_platform() {
    local cpu_model
    cpu_model=$(grep -m1 "model name" /proc/cpuinfo | cut -d: -f2 | sed 's/^[ \t]*//' || echo "unknown")
    
    if echo "$cpu_model" | grep -qi "Ultra"; then
        print_success "Core Ultra platform detected: $cpu_model"
        return 0
    else
        print_error "This script is only for Intel Core Ultra platforms"
        print_info "Current CPU: $cpu_model"
        exit 1
    fi
}

# Resolve latest NPU driver release from GitHub
resolve_latest_release() {
    print_info "Resolving latest NPU driver release from GitHub..."
    
    local json url tag
    json=$(curl -fsSL https://api.github.com/repos/intel/linux-npu-driver/releases/latest 2>/dev/null || true)
    
    if [ -n "$json" ]; then
        tag=$(echo "$json" | grep -m1 '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
        url=$(echo "$json" | grep '"browser_download_url"' | grep -E 'tar\.gz' | grep -E 'ubuntu2404|ubuntu24\.04|ubuntu24' | head -1 | sed -E 's/.*"(https:[^"]+)".*/\1/')
        
        if [ -z "$url" ]; then
            # Fallback to any tar.gz if ubuntu-specific not found
            url=$(echo "$json" | grep '"browser_download_url"' | grep -E '\.tar\.gz"' | head -1 | sed -E 's/.*"(https:[^"]+)".*/\1/')
        fi
        
        if [ -n "$url" ]; then
            print_success "Found NPU driver release: $tag"
            NPU_ASSET_URL="$url"
            return 0
        fi
    fi
    
    print_error "Failed to resolve NPU driver release from GitHub"
    return 1
}

# Download NPU driver packages
download_npu_packages() {
    print_info "Downloading NPU driver packages..."
    
    local archive_name
    archive_name=$(basename "$NPU_ASSET_URL")
    
    if wget -q --timeout=30 "${NPU_ASSET_URL}"; then
        print_success "Downloaded ${archive_name}"
        
        print_info "Extracting ${archive_name}..."
        if tar -xzf "${archive_name}"; then
            print_success "Extracted NPU packages"
            rm -f "${archive_name}"
        else
            print_error "Failed to extract ${archive_name}"
            return 1
        fi
    else
        print_error "Failed to download NPU driver from ${NPU_ASSET_URL}"
        return 1
    fi
    
    return 0
}

# Install NPU packages
install_npu_packages() {
    print_info "Installing NPU driver packages..."
    
    if dpkg -i ./*.deb; then
        print_success "NPU packages installed successfully"
        return 0
    else
        print_warning "Package installation had issues, attempting to fix dependencies..."
        if apt-get install --fix-broken -y && dpkg -i ./*.deb; then
            print_success "NPU packages installed after dependency fix"
            return 0
        else
            print_error "Failed to install NPU packages"
            return 1
        fi
    fi
}

# Setup device permissions
setup_device_permissions() {
    print_info "Configuring NPU device permissions..."
    
    # Create udev rules (should already exist from image build)
    if [ ! -f /etc/udev/rules.d/10-intel-vpu.rules ]; then
        echo 'SUBSYSTEM=="accel", KERNEL=="accel*", GROUP="render", MODE="0660"' > /etc/udev/rules.d/10-intel-vpu.rules
        print_success "Created udev rules"
    else
        print_info "udev rules already exist"
    fi
    
    # Reload udev rules
    udevadm control --reload-rules
    udevadm trigger
    print_success "udev rules reloaded"
}

# Verify installation
verify_installation() {
    print_info "Verifying NPU installation..."
    
    local all_installed=true
    local npu_packages=("intel-driver-compiler-npu" "intel-fw-npu" "intel-level-zero-npu")
    
    for pkg in "${npu_packages[@]}"; do
        if dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q "install ok installed"; then
            print_success "Package $pkg installed"
        else
            print_error "Package $pkg NOT installed"
            all_installed=false
        fi
    done
    
    # Check NPU device
    if [ -e /dev/accel/accel0 ]; then
        print_success "NPU device found: /dev/accel/accel0"
        ls -la /dev/accel/accel0
    else
        print_warning "NPU device not found (may require reboot)"
    fi
    
    # Check dmesg for intel_vpu
    if dmesg 2>/dev/null | grep -qi "intel_vpu"; then
        print_success "intel_vpu driver messages found in dmesg"
    else
        print_warning "intel_vpu driver messages not found (may require reboot)"
    fi
    
    if [ "$all_installed" = true ]; then
        print_success "NPU installation verified successfully"
    else
        print_warning "Some NPU components may not be installed correctly"
    fi
}

# Main installation function
main() {
    print_info "Intel NPU Driver Installation Script"
    print_info "====================================="
    
    check_privileges
    verify_platform
    
    # Create temporary directory
    local temp_dir="/tmp/npu_installer_$$"
    mkdir -p "$temp_dir"
    cd "$temp_dir" || exit 1
    
    # Installation steps
    print_info "Step 1: Resolving latest NPU driver release..."
    resolve_latest_release || exit 1
    
    print_info "Step 2: Downloading NPU packages..."
    download_npu_packages || exit 1
    
    print_info "Step 3: Installing NPU packages..."
    install_npu_packages || exit 1
    
    print_info "Step 4: Setting up device permissions..."
    setup_device_permissions || exit 1
    
    # Cleanup
    print_info "Step 5: Cleaning up temporary files..."
    cd / || exit 1
    rm -rf "$temp_dir"
    print_success "Cleanup completed"
    
    # Verify
    print_info "Step 6: Verifying installation..."
    verify_installation
    
    echo ""
    print_success "NPU installation completed!"
    echo ""
    print_warning "System reboot is recommended for NPU to be fully functional"
    print_info "After reboot, verify with: ls -la /dev/accel/accel0"
    print_info "Also check dmesg for intel_vpu messages: dmesg | grep intel_vpu"
    echo ""
}

# Run main if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
