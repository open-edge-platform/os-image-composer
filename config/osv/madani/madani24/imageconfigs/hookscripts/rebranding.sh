#!/bin/sh

# 1. Safety Check: Ensure TARGET_ROOTFS is defined
if [ -z "$TARGET_ROOTFS" ]; then
    echo "Error: TARGET_ROOTFS environment variable is not set."
    echo "Aborting to prevent overwriting host system files."
    exit 1
fi

# Ensure essential directories exist
mkdir -p "$TARGET_ROOTFS/etc/update-motd.d"

echo "Step 1: Setting up permanent Package Diversions..."
# These commands run via chroot to update the internal dpkg database.
# This ensures Ubuntu updates never overwrite your custom brand files.
chroot "$TARGET_ROOTFS" dpkg-divert --add --rename --divert /etc/os-release.ubuntu /etc/os-release
chroot "$TARGET_ROOTFS" dpkg-divert --add --rename --divert /etc/lsb-release.ubuntu /etc/lsb-release
chroot "$TARGET_ROOTFS" dpkg-divert --add --rename --divert /etc/legal.ubuntu /etc/legal

echo "Step 2: Applying Madani OS Identity files..."

# Overwrite os-release
cat <<EOF > "$TARGET_ROOTFS/etc/os-release"
NAME="Madani OS"
VERSION="0.1 (Alpha)"
ID=madanios
ID_LIKE="ubuntu debian"
PRETTY_NAME="Madani OS 0.1"
VERSION_ID="0.1"
HOME_URL="https://madanios.com"
BUG_REPORT_URL="https://madanios.com/support"
EOF

# Overwrite lsb-release
cat <<EOF > "$TARGET_ROOTFS/etc/lsb-release"
DISTRIB_ID=madanios
DISTRIB_RELEASE=0.1
DISTRIB_CODENAME=noble
DISTRIB_DESCRIPTION="Madani OS 0.1 experimental"
EOF

# Overwrite legal (Removes the Ubuntu-specific warranty text)
cat <<EOF > "$TARGET_ROOTFS/etc/legal"
The programs included with the Madani OS system are free software;
the exact distribution terms for each program are described in the
individual files in /usr/share/doc/*/copyright.

Madani OS comes with ABSOLUTELY NO WARRANTY, to the extent permitted by
applicable law.
EOF

# Pre-login text (The banner shown at the TTY login)
echo "Welcome to Madani OS 0.1 experimental" > "$TARGET_ROOTFS/etc/issue"
echo "Welcome to Madani OS 0.1 experimental" > "$TARGET_ROOTFS/etc/issue.net"

echo "Step 3: Cleaning up Ubuntu MOTD and Sudo hints..."

# Remove dynamic Ubuntu links, news, and upgrade notifications
rm -f "$TARGET_ROOTFS/etc/update-motd.d/10-help-text"
rm -f "$TARGET_ROOTFS/etc/update-motd.d/50-motd-news"
rm -f "$TARGET_ROOTFS/etc/update-motd.d/80-livepatch"
rm -f "$TARGET_ROOTFS/etc/update-motd.d/91-release-upgrade"

# Create a clean Madani banner that appears after login
cat <<EOF > "$TARGET_ROOTFS/etc/update-motd.d/00-madani-banner"
#!/bin/sh
echo "Welcome to Madani OS 0.1 experimental (\$(uname -rsv))"
EOF
chmod +x "$TARGET_ROOTFS/etc/update-motd.d/00-madani-banner"

# Remove the specific Ubuntu sudo helper hint from the system bash config
if [ -f "$TARGET_ROOTFS/etc/bash.bashrc" ]; then
    sed -i '/To run a command as administrator/d' "$TARGET_ROOTFS/etc/bash.bashrc"
    sed -i '/See "man sudo_root"/d' "$TARGET_ROOTFS/etc/bash.bashrc"
fi

echo "Branding complete. Madani OS identity is now locked."