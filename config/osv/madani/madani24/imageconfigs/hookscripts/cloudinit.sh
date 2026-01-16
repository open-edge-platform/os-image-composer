#!/bin/sh

# 1. Safety Check: Ensure TARGET_ROOTFS is defined
if [ -z "$TARGET_ROOTFS" ]; then
    echo "Error: TARGET_ROOTFS environment variable is not set."
    echo "Aborting to prevent overwriting host system files."
    exit 1
fi
echo "Setting up cloud-init seed configuration..."

# Create cloud-init seed directory
mkdir -p "$TARGET_ROOTFS/var/lib/cloud/seed/nocloud"

# Create meta-data file
cat <<EOF > "$TARGET_ROOTFS/var/lib/cloud/seed/nocloud/meta-data"
instance-id: uki-001
local-hostname: uki-node
EOF

# Create user-data file
cat <<EOF > "$TARGET_ROOTFS/var/lib/cloud/seed/nocloud/user-data"
#cloud-config
growpart:
  mode: auto
  devices: ['/']
  ignore_growroot_disabled: false

resize_rootfs: true

package_update: false
package_upgrade: false

runcmd:
  - |
    echo "Starting sequential setup..."
    chmod +x /opt/software/setup-wait-for-network.sh
    chmod +x /opt/software/setup-ollama.sh
    
    # This is the secret: one line, one sequence.
    /opt/software/setup-wait-for-network.sh && /opt/software/setup-ollama.sh
    
    echo "Setup sequence complete."
EOF

echo "Cloud-init seed configuration complete."