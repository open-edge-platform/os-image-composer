#!/bin/bash
# OpenVINO Installation Script for Edge Developer Kit Images
# Installs OpenVINO toolkit with Python virtual environment
#
# Based on: openvino_installer.sh from edge-developer-kit-reference-scripts
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
#
# Usage: sudo /opt/intel/install-openvino.sh [--version VERSION]

set -euo pipefail

# Configuration
OPENVINO_VERSION="${OPENVINO_VERSION:-2025.0}"
OPENVINO_ENV_PATH="/opt/intel/openvino_env"

# Status indicators
S_ERROR="[ERROR]"
S_VALID="[✓]"
S_WARNING="[!]"
S_INFO="[INFO]"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_error() { echo -e "${RED}${S_ERROR} $1${NC}"; }
print_success() { echo -e "${GREEN}${S_VALID} $1${NC}"; }
print_warning() { echo -e "${YELLOW}${S_WARNING} $1${NC}"; }
print_info() { echo -e "${S_INFO} $1"; }

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                OPENVINO_VERSION="$2"
                shift 2
                ;;
            --help|-h)
                echo "Usage: $0 [--version VERSION]"
                echo "  --version VERSION  OpenVINO version to install (default: $OPENVINO_VERSION)"
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
}

# Check if running as root
check_privileges() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Install system dependencies
install_dependencies() {
    print_info "Installing system dependencies..."
    
    apt-get update
    apt-get install -y python3-pip python3-venv python3-dev
    
    print_success "System dependencies installed"
}

# Create Python virtual environment
create_venv() {
    print_info "Creating OpenVINO Python virtual environment at $OPENVINO_ENV_PATH..."
    
    if [ -d "$OPENVINO_ENV_PATH" ]; then
        print_warning "Virtual environment already exists, recreating..."
        rm -rf "$OPENVINO_ENV_PATH"
    fi
    
    python3 -m venv "$OPENVINO_ENV_PATH"
    print_success "Virtual environment created"
}

# Install OpenVINO
install_openvino() {
    print_info "Installing OpenVINO $OPENVINO_VERSION..."
    
    # Activate virtual environment
    source "$OPENVINO_ENV_PATH/bin/activate"
    
    # Upgrade pip
    pip install --upgrade pip setuptools wheel
    
    # Install OpenVINO
    if pip install "openvino==$OPENVINO_VERSION"; then
        print_success "OpenVINO $OPENVINO_VERSION installed"
    else
        print_warning "Exact version not found, installing latest OpenVINO..."
        pip install openvino
        print_success "OpenVINO installed (latest version)"
    fi
    
    # Install additional OpenVINO packages
    pip install openvino-dev
    print_success "OpenVINO development tools installed"
    
    deactivate
}

# Verify installation
verify_installation() {
    print_info "Verifying OpenVINO installation..."
    
    source "$OPENVINO_ENV_PATH/bin/activate"
    
    if python -c "import openvino; print(f'OpenVINO version: {openvino.__version__}')" 2>/dev/null; then
        print_success "OpenVINO Python package verified"
        
        # Check available devices
        print_info "Checking available OpenVINO devices..."
        python -c "
from openvino import Core
core = Core()
devices = core.available_devices
print(f'Available devices: {devices}')
for device in devices:
    try:
        full_name = core.get_property(device, 'FULL_DEVICE_NAME')
        print(f'  {device}: {full_name}')
    except:
        print(f'  {device}')
" 2>/dev/null || print_warning "Could not enumerate devices (may require GPU/NPU drivers)"
        
    else
        print_error "OpenVINO verification failed"
        deactivate
        return 1
    fi
    
    deactivate
    return 0
}

# Create activation script for users
create_activation_script() {
    print_info "Creating user activation script..."
    
    cat > /opt/intel/activate_openvino.sh << 'EOF'
#!/bin/bash
# Activate OpenVINO environment
# Usage: source /opt/intel/activate_openvino.sh

if [ -f "/opt/intel/openvino_env/bin/activate" ]; then
    source /opt/intel/openvino_env/bin/activate
    echo "OpenVINO environment activated"
    python -c "import openvino; print(f'OpenVINO version: {openvino.__version__}')" 2>/dev/null || true
else
    echo "OpenVINO environment not found. Run: sudo /opt/intel/install-openvino.sh"
fi
EOF
    
    chmod +x /opt/intel/activate_openvino.sh
    print_success "Activation script created: /opt/intel/activate_openvino.sh"
}

# Main
main() {
    print_info "OpenVINO Installation Script"
    print_info "============================"
    
    parse_args "$@"
    check_privileges
    
    print_info "Installing OpenVINO version: $OPENVINO_VERSION"
    print_info "Installation path: $OPENVINO_ENV_PATH"
    echo ""
    
    install_dependencies
    create_venv
    install_openvino
    create_activation_script
    verify_installation
    
    echo ""
    print_success "OpenVINO installation completed!"
    echo ""
    print_info "To use OpenVINO, run: source /opt/intel/activate_openvino.sh"
    echo ""
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
