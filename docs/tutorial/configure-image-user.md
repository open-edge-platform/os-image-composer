# Secure Boot Configuration Tutorial

This guide walks you through setting up setting login users for targetted OS

## Prerequisites

- Linux environment with OpenSSL installed
- QEMU with OVMF UEFI firmware
- OS Image Composer tool configured

## Step 1: Generate Secure Boot Keys

Create a directory for your keys and generate the required certificates:

```bash
# Create a directory for secure boot keys
mkdir -p /data/secureboot/keys
cd /data/secureboot/keys

# Generate private key and certificate using RSA 3072-bit with SHA-384
openssl req -new -x509 -newkey rsa:3072 -sha384 -keyout DB.key -out DB.crt -days 3650 -nodes -subj "/CN=ICT Secure Boot Key/"

# Convert certificate to DER format (required by UEFI)
openssl x509 -outform DER -in DB.crt -out DB.cer
```

NOTE: The signing keypair strength should align with the crypto implementation
supported by the UEFI Secure boot implementation on a specific system. The
recommendation is to test the support for `RSA3072SHA384` before moving to
`RSA2048SHA256`.

**What you'll have:**

- `DB.key` - Private key (keep secure)
- `DB.crt` - Certificate in PEM format
- `DB.cer` - Certificate in DER format (for UEFI)

## Step 2: Configure Your Template

Edit your OS Image Composer template YAML file to include the Secure Boot
configuration:

```yaml
# Add this section to your template
immutability:
  enabled: true
  secureBootDBKey: "/data/secureboot/keys/DB.key"
  secureBootDBCrt: "/data/secureboot/keys/DB.crt"
  secureBootDBCer: "/data/secureboot/keys/DB.cer"
```

**Important:** Use absolute paths to your key files.

## Step 3: Build Your OS Image

Run ICT to build your image as usual.

## Step 4: Verify Build Output

After a successful build, check the output directory, for example:

```bash
ls ./tmp/os-image-composer/wind-river-elxr-elxr12-x86_64/imagebuild/Default_Raw/ -la
```

**Expected output:**

- `minimal-os-image-elxr.raw` - Your bootable OS image
- `DB.cer` - Secure Boot certificate (copied during build)

## Step 5: Prepare Image for Testing

Copy the certificate to the EFI partition for easier key enrollment:

```bash
# Mount the raw image
sudo losetup -Pf minimal-os-image-elxr.raw

# Find the loop device (usually /dev/loop0)
LOOP_DEVICE=$(losetup -l | grep minimal-os-image-elxr.raw | awk '{print $1}')
echo "Using loop device: $LOOP_DEVICE"

# Check partitions
lsblk $LOOP_DEVICE

# Mount EFI partition (usually partition 1)
sudo mkdir -p /mnt/efi
sudo mount ${LOOP_DEVICE}p1 /mnt/efi

# Create keys directory and copy certificate
sudo mkdir -p /mnt/efi/EFI/keys
sudo cp DB.cer /mnt/efi/EFI/keys/

# Cleanup
sudo umount /mnt/efi
sudo losetup -d $LOOP_DEVICE
```

## Step 6: Boot Image in QEMU

Launch QEMU with UEFI firmware:

```bash
sudo qemu-system-x86_64 \
  -m 2048 \
  -enable-kvm \
  -cpu host \
  -bios /usr/share/OVMF/OVMF_CODE.fd \
  -device virtio-scsi-pci \
  -drive if=none,id=drive0,file=minimal-os-image-elxr.raw,format=raw \
  -device scsi-hd,drive=drive0 \
  -nographic \
  -serial mon:stdio \
  -boot menu=on
```

**Tip:** Press `Esc` repeatedly as soon as QEMU starts to enter UEFI setup.

## Step 7: Enroll Secure Boot Keys

Once you're in the UEFI setup menu, do the following.

**Note:** Menu names vary by firmware. Look for similar options if the exact
names differ.

### Navigate to Secure Boot

1. Use arrow keys to find **"Device Manager"** or
   **"Secure Boot Configuration"**
2. Look for **"Secure Boot"** or **"Security"** menu

### Enable Custom Mode

1. Find **"Secure Boot Mode"**
2. Change from **"Standard"** to **"Custom"**
3. This allows manual key management

### Enroll Your Key

1. Navigate to **"Custom Secure Boot Options"**
2. Select **"DB Options"** (Database Options)
3. Choose **"Enroll Signature"** or **"Enroll DB"**
4. Navigate to: **`fs0:\EFI\keys\DB.cer`**
5. Select the file and confirm enrollment

### Save and Exit

1. Press **F10** to save changes
2. Select **"Reset"** or **"Exit"**
3. System will reboot

## Step 8: Verify Secure Boot

After the system boots completely, verify that Secure Boot is working:

```bash
# Check if Secure Boot is enabled
sudo dmesg | grep -i secure

# Expected output:
# [    0.000000] secureboot: Secure boot enabled
# [    0.716009] integrity: Loaded X.509 cert 'ICT Secure Boot Key: [key-hash]'
```

## Troubleshooting

**Common Issues:**

1. **Can't find keys in UEFI:** Ensure the EFI partition is mounted and files
   are in `/EFI/keys/`.
2. **Secure Boot not enabled:** Verify you're in "Custom" mode, not
   "Standard" mode.
3. **Boot fails after key enrollment:** Check that your image was built with
   the same keys.

**Recovery:**

- Boot QEMU without Secure Boot: Remove `-bios /usr/share/OVMF/OVMF_CODE.fd`
- Reset UEFI settings: In UEFI setup, look for "Reset to defaults."

## Summary

You've successfully:

- ✅ Generated Secure Boot keys
- ✅ Built an image with Secure Boot enabled
- ✅ Enrolled keys in UEFI firmware
- ✅ Verified Secure Boot functionality

# Image User Configuration Tutorial

This guide walks you through setting up login users for your target OS image using OS Image Composer.

## Prerequisites

- Linux environment
- OS Image Composer tool configured
- Basic understanding of YAML configuration

## Step 1: Understanding User Configuration

OS Image Composer supports two types of user password configuration:

1. **Plaintext passwords** (for development/testing only)
2. **Hashed passwords** (recommended for production)

## Step 2: Generate Password Hashes

For production environments, generate secure password hashes:

```bash
# Generate SHA-512 hash for a password
python3 -c "import crypt; print(crypt.crypt('your_password', crypt.mksalt(crypt.METHOD_SHA512)))"

# Alternative using openssl
openssl passwd -6 your_password

# Interactive password prompt (recommended)
python3 -c "import crypt, getpass; print(crypt.crypt(getpass.getpass(), crypt.mksalt(crypt.METHOD_SHA512)))"
```

**Security Note:** Never commit plaintext passwords to version control.

## Step 3: Configure Users in Your Template

Edit your OS Image Composer template YAML file to include user configurations:

```yaml
# Basic user configuration examples
users:
  # Development user with plaintext password (NOT for production)
  - name: devuser
    password: "devpass123"  # Do not commit real plaintext passwords
    groups: ["wheel", "sudo"]
    
  # Production user with hashed password
  - name: admin
    hash_algo: "sha512"
    password: "$6$qisZydr7DPWjCwDk$uiFDXvewTwAqs4H0gO7lRkmc5j2IUiuxSA8Yi.kjN9aLu4w3vysV80mD6C/0DvaBPLYCWU2fJwatYxVASJVL20"
    groups: ["wheel", "sudo"]
    
  # Service account user
  - name: serviceuser
    hash_algo: "sha512"
    password: "$6$rounds=656000$saltstring$hash_value_here"
    groups: ["docker", "systemd-journal"]
    
  # Regular user without admin privileges
  - name: operator
    hash_algo: "sha512"
    password: "$6$aB3$example.hash.value.here"
    groups: ["users"]
```

## Step 4: Example User Configurations

Here are some example user configurations for different scenarios:

```yaml
users:
  # System administrator
  - name: sysadmin
    hash_algo: "sha512"
    password: "$6$kL9mN2oP$8xY4vW6qR5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2h"
    groups: ["wheel", "sudo", "adm"]
    
  # Application developer
  - name: appdev
    hash_algo: "sha512"  
    password: "$6$xY4vW6qR$5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2hJ8kL4mN6"
    groups: ["docker", "users", "systemd-journal"]
    
  # Guest user (limited access)
  - name: guest
    hash_algo: "sha512"
    password: "$6$fG2hJ8kL$4mN6oP8qR5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2hJ8kL4mN6oP8qR5tU3iO7pA1sD9fG2h"
    groups: ["users"]
  
```

## Step 5: Common User Groups

Understanding common Linux groups for user assignment:

- **`wheel`** - Administrative group (sudo access on some systems)
- **`sudo`** - Sudo access group
- **`adm`** - System monitoring and log access
- **`users`** - Standard user group
- **`docker`** - Docker daemon access
- **`systemd-journal`** - System journal access
- **`audio`** - Audio device access
- **`video`** - Video device access
- **`dialout`** - Serial port access
- **`plugdev`** - Pluggable device access

## Step 6: Build Your OS Image

Run OS Image Composer to build your image with the configured users.

## Step 7: Verify User Configuration

After building and booting your image, verify the users were created correctly:

```bash
# List all users
cat /etc/passwd | grep -E "(sysadmin|appdev|dbadmin|monitor|guest|maintenance)"

# Check user groups
groups sysadmin
groups appdev

# Verify sudo access (for users in wheel/sudo groups)
sudo -l -U sysadmin
```

## Step 8: Test User Login

Test logging in with your configured users:

```bash
# Switch to a configured user
su - sysadmin

# Test sudo access
sudo whoami

# Check user's groups
id
```

## Security Best Practices

1. **Never use plaintext passwords in production**
2. **Use strong, unique passwords for each user**
3. **Regularly rotate passwords**
4. **Assign minimal required group permissions**
5. **Remove or disable unused accounts**
6. **Consider using SSH keys instead of passwords**

## Troubleshooting

**Common Issues:**

1. **User cannot login:** Check password hash generation and syntax
2. **No sudo access:** Verify user is in `wheel` or `sudo` group
3. **Permission denied:** Check group assignments for required resources

**Debugging:**

```bash
# Check if user exists
id username

# Verify password hash
sudo cat /etc/shadow | grep username

# Check group membership
groups username
```

## Summary

You've successfully learned how to:

- ✅ Configure users in OS Image Composer templates
- ✅ Generate secure password hashes
- ✅ Assign appropriate user groups
- ✅ Build images with pre-configured users
- ✅ Verify user configuration after deployment
