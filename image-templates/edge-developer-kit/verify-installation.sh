#!/bin/bash
# Installation Verification Script for Edge Developer Kit Images
# Mimics the output from print_summary_table.sh
#
# Based on: print_summary_table.sh from edge-developer-kit-reference-scripts
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
#
# Usage: /opt/intel/verify-installation.sh

set -euo pipefail

# Status indicators
S_ERROR="[ERROR]"
S_VALID="[✓]"
S_WARNING="[!]"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo ""
    echo "========================================================================"
    echo "Running Installation Summary"
    echo ""
    echo "==================== System Installation Summary ===================="
    printf "%-25s | %-45s\n" "Item" "Value"
    echo "------------------------- -+----------------------------------------------"
}

print_row() {
    local item="$1"
    local value="$2"
    printf "%-25s | %-45s\n" "$item" "$value"
}

print_section_header() {
    echo "------------------------- -+----------------------------------------------"
    printf "%-25s |\n" "$1"
    echo "------------------------- -+----------------------------------------------"
}

# Get system info
get_kernel_version() {
    uname -r
}

get_ubuntu_version() {
    if [ -f /etc/os-release ]; then
        source /etc/os-release
        echo "$PRETTY_NAME"
    else
        echo "Unknown"
    fi
}

check_hwe_stack() {
    if dpkg -l | grep -q "linux-generic-hwe-24.04"; then
        echo "Installed"
    else
        echo "Not Installed"
    fi
}

# Check NPU status
check_npu_status() {
    if [ -e /dev/accel/accel0 ]; then
        echo "Detected"
    else
        echo "Not Detected"
    fi
}

get_npu_packages() {
    local packages=("intel-driver-compiler-npu" "intel-fw-npu" "intel-level-zero-npu" "level-zero")
    for pkg in "${packages[@]}"; do
        local version
        version=$(dpkg-query -W -f='${Version}' "$pkg" 2>/dev/null || echo "Not Installed")
        print_row "$pkg" "$version"
    done
}

# Check GPU info
check_gpu_driver() {
    if lsmod | grep -q "i915"; then
        echo "i915 (loaded)"
    elif lsmod | grep -q "xe"; then
        echo "xe (loaded)"
    else
        echo "Not loaded"
    fi
}

get_gpu_type() {
    if lspci -nn 2>/dev/null | grep -qi "VGA.*Intel"; then
        echo "Intel"
    else
        echo "Unknown"
    fi
}

count_gpu_devices() {
    local count
    count=$(lspci -nn 2>/dev/null | grep -Ei 'VGA|DISPLAY' | grep -i intel | wc -l)
    echo "$count Intel graphics device(s) detected"
}

list_gpu_devices() {
    local device_num=1
    while IFS= read -r line; do
        print_row "GPU Device $device_num" "$line"
        ((device_num++))
    done < <(lspci 2>/dev/null | grep -Ei 'VGA|DISPLAY' | grep -i Intel)
}

# Check Intel graphics packages
get_intel_packages() {
    local packages=(
        "i965-va-driver:amd64"
        "intel-gsc"
        "intel-media-va-driver-non-free:amd64"
        "intel-opencl-icd"
        "libegl-mesa0:amd64"
        "libze-intel-gpu1"
        "libze1"
        "clinfo"
        "vainfo"
    )
    
    for pkg in "${packages[@]}"; do
        local version
        # Strip :amd64 for dpkg query
        local pkg_name="${pkg%:amd64}"
        version=$(dpkg-query -W -f='${Version}' "$pkg_name" 2>/dev/null || echo "Not Installed")
        print_row "$pkg" "$version"
    done
}

# Check OpenCL status
check_opencl() {
    if command -v clinfo >/dev/null 2>&1; then
        local device_count
        device_count=$(clinfo -l 2>/dev/null | grep -c "Device" || echo "0")
        echo "$device_count device(s)"
    else
        echo "clinfo not installed"
    fi
}

# Check VA-API status
check_vaapi() {
    if command -v vainfo >/dev/null 2>&1; then
        if vainfo 2>/dev/null | grep -q "vainfo:"; then
            echo "Available"
        else
            echo "Not working"
        fi
    else
        echo "vainfo not installed"
    fi
}

# Check Docker status
check_docker() {
    if command -v docker >/dev/null 2>&1; then
        if docker info >/dev/null 2>&1; then
            echo "Running"
        else
            echo "Installed (not running)"
        fi
    else
        echo "Not Installed"
    fi
}

# Check OpenVINO status
check_openvino() {
    if [ -d "/opt/intel/openvino_env" ]; then
        if /opt/intel/openvino_env/bin/python -c "import openvino" 2>/dev/null; then
            local version
            version=$(/opt/intel/openvino_env/bin/python -c "import openvino; print(openvino.__version__)" 2>/dev/null || echo "Unknown")
            echo "Installed ($version)"
        else
            echo "Environment exists (not configured)"
        fi
    else
        echo "Not Installed (run /opt/intel/install-openvino.sh)"
    fi
}

# Check platform status
check_platform_status() {
    local status="configured"
    local issues=0
    
    # Check GPU driver
    if ! lsmod | grep -qE "i915|xe"; then
        ((issues++))
    fi
    
    # Check render device
    if ! ls /dev/dri/renderD* >/dev/null 2>&1; then
        ((issues++))
    fi
    
    # Check user groups
    if ! groups 2>/dev/null | grep -qE "video|render"; then
        ((issues++))
    fi
    
    if [ $issues -eq 0 ]; then
        echo -e "${GREEN}${S_VALID} Platform is configured${NC}"
    else
        echo -e "${YELLOW}${S_WARNING} Platform has $issues issue(s)${NC}"
    fi
}

# Main
main() {
    print_header
    
    # System Info
    print_row "Kernel Version" "$(get_kernel_version)"
    print_row "HWE Stack" "$(check_hwe_stack)"
    print_row "Ubuntu Version" "$(get_ubuntu_version)"
    
    # NPU Info
    print_row "NPU Status" "$(check_npu_status)"
    
    # NPU Packages (if any installed)
    if dpkg -l | grep -qE "intel.*npu|level-zero"; then
        get_npu_packages
    fi
    
    # GPU Info
    print_row "GPU Type" "$(get_gpu_type)"
    print_row "GPU Count" "$(count_gpu_devices)"
    print_row "GPU Driver" "$(check_gpu_driver)"
    list_gpu_devices
    
    # Intel Graphics Packages
    print_section_header "Intel Graphics Packages"
    get_intel_packages
    
    # Additional Status
    print_section_header "Additional Components"
    print_row "OpenCL Devices" "$(check_opencl)"
    print_row "VA-API Status" "$(check_vaapi)"
    print_row "Docker Status" "$(check_docker)"
    print_row "OpenVINO Status" "$(check_openvino)"
    
    # Platform Status
    print_section_header "Platform Status"
    check_platform_status
    
    echo "========================================================================"
    echo ""
    echo "Verification completed: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "========================================================================"
}

main "$@"
