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
package_update: true
runcmd:  
  - [ chmod, "+x", "/opt/software/setup-wait-for-network.sh" ]
  - [ chmod, "+x", "/opt/software/setup-ollama.sh" ]  
  - [ /bin/bash, "/opt/software/setup-wait-for-network.sh" ]    
  - [ /bin/bash, "/opt/software/setup-ollama.sh" ]
EOF

echo "Cloud-init seed configuration complete."