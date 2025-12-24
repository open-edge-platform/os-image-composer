#!/bin/sh

# 1. Safety Check: Ensure TARGET_ROOTFS is defined
if [ -z "$TARGET_ROOTFS" ]; then
    echo "Error: TARGET_ROOTFS environment variable is not set."
    echo "Aborting to prevent overwriting host system files."
    exit 1
fi

# Ensure the /etc directory exists in the target
mkdir -p "$TARGET_ROOTFS/etc"

echo "Applying Madani OS branding to $TARGET_ROOTFS..."

# 1. Overwrite os-release with commercial branding
cat <<EOF > "$TARGET_ROOTFS/etc/os-release"
NAME="Madani OS"
VERSION="0.1 (Alpha)"
ID=madanios
ID_LIKE=debian
PRETTY_NAME="Madani OS 0.1"
VERSION_ID="0.1"
HOME_URL="https://madanios.com"
BUG_REPORT_URL="https://madanios.com/support"
EOF

# 2. Overwrite lsb-release
cat <<EOF > "$TARGET_ROOTFS/etc/lsb-release"
DISTRIB_ID=madanios
DISTRIB_RELEASE=1.0
DISTRIB_CODENAME=noble
DISTRIB_DESCRIPTION="Madani OS 0.1 experimental"
EOF

# 3. Handle the 'issue' files (the text shown at login)
echo "Welcome to Madani OS 0.1 experimental" > "$TARGET_ROOTFS/etc/issue"
echo "Welcome to Madani OS 0.1 experimental" > "$TARGET_ROOTFS/etc/issue.net"

# 4. Remove standard MOTD dynamic scripts
# These usually provide links to help and news which you likely want to hide
rm -f "$TARGET_ROOTFS/etc/update-motd.d/10-help-text"
rm -f "$TARGET_ROOTFS/etc/update-motd.d/50-motd-news"
rm -f "$TARGET_ROOTFS/etc/update-motd.d/80-livepatch"

echo "Branding complete."