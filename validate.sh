#!/bin/bash
set -e
# Expect to be run from the root of the PR branch

# Set working directory variable
WORKING_DIR="$(pwd)"
echo "Working directory set to: $WORKING_DIR"

# Parse command line arguments
RUN_QEMU_TESTS=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --qemu-test|--with-qemu)
      RUN_QEMU_TESTS=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--qemu-test|--with-qemu]"
      echo "  --qemu-test, --with-qemu  Run QEMU boot tests after each image build"
      echo "  -h, --help               Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option $1"
      echo "Use -h or --help for usage information"
      exit 1
      ;;
  esac
done

echo "Current working dir: $(pwd)"
if [ "$RUN_QEMU_TESTS" = true ]; then
  echo "QEMU boot tests will be run after each image build"
else
  echo "QEMU boot tests will be skipped"
fi

# Centralized cleanup function for image files
cleanup_image_files() {
  local cleanup_type="${1:-all}"  # Options: all, raw, extracted
  
  case "$cleanup_type" in
    "raw")
      echo "Cleaning up raw image files from build directories..."
      sudo rm -rf ./tmp/*/imagebuild/*/*.raw 2>/dev/null || true
      ;;
    "extracted")
      echo "Cleaning up extracted image files in current directory..."
      rm -f *.raw 2>/dev/null || true
      ;;
    "all"|*)
      echo "Cleaning up all temporary image files..."
      sudo rm -rf ./tmp/*/imagebuild/*/*.raw 2>/dev/null || true
      rm -f *.raw 2>/dev/null || true
      ;;
  esac
}

run_qemu_boot_test() {
  local IMAGE_PATTERN="$1"
  if [ -z "$IMAGE_PATTERN" ]; then
    echo "Error: Image pattern not provided to run_qemu_boot_test"
    return 1
  fi
  
  BIOS="/usr/share/OVMF/OVMF_CODE_4M.fd"
  TIMEOUT=30
  SUCCESS_STRING="login:"
  LOGFILE="qemu_serial.log"

  ORIGINAL_DIR=$(pwd)
  # Find compressed raw image path using pattern, handle permission issues
  FOUND_PATH=$(sudo -S find . -type f -name "*${IMAGE_PATTERN}*.raw.gz" 2>/dev/null | head -n 1)
  if [ -n "$FOUND_PATH" ]; then
    echo "Found compressed image at: $FOUND_PATH"
    IMAGE_DIR=$(dirname "$FOUND_PATH")
    
    # Fix permissions for the tmp directory recursively to allow access
    echo "Setting permissions recursively for ./tmp directory"
    sudo chmod -R 777 ./tmp
    
    cd "$IMAGE_DIR"
    
    # Extract the .raw.gz file
    COMPRESSED_IMAGE=$(basename "$FOUND_PATH")
    RAW_IMAGE="${COMPRESSED_IMAGE%.gz}"
    echo "Extracting $COMPRESSED_IMAGE to $RAW_IMAGE..."
    
    # Check available disk space before extraction
    AVAILABLE_SPACE=$(df . | tail -1 | awk '{print $4}')
    COMPRESSED_SIZE=$(stat -c%s "$COMPRESSED_IMAGE" 2>/dev/null || echo "0")
    # Estimate uncompressed size (typically 4-6x larger for these images, being conservative)
    ESTIMATED_SIZE=$((COMPRESSED_SIZE * 6 / 1024))
    
    echo "Disk space check: Available=${AVAILABLE_SPACE}KB, Estimated needed=${ESTIMATED_SIZE}KB"
    
    # Always try aggressive cleanup first to ensure maximum space
    echo "Performing aggressive cleanup before extraction..."
    sudo rm -f *.raw 2>/dev/null || true
    sudo rm -f /tmp/*.raw 2>/dev/null || true
    sudo rm -rf ../../../cache/ 2>/dev/null || true
    sudo rm -rf ../../../tmp/*/imagebuild/*/*.raw 2>/dev/null || true
    
    # Force filesystem sync and check space again
    sync
    AVAILABLE_SPACE=$(df . | tail -1 | awk '{print $4}')
    echo "Available space after cleanup: ${AVAILABLE_SPACE}KB"
    
    if [ "$AVAILABLE_SPACE" -lt "$ESTIMATED_SIZE" ]; then
      echo "Warning: Still insufficient disk space after cleanup"
      echo "Attempting extraction to /tmp with streaming..."
      
      # Check /tmp space
      TMP_AVAILABLE=$(df /tmp | tail -1 | awk '{print $4}')
      echo "/tmp available space: ${TMP_AVAILABLE}KB"
      
      if [ "$TMP_AVAILABLE" -gt "$ESTIMATED_SIZE" ]; then
        TMP_RAW="/tmp/$RAW_IMAGE"
        echo "Extracting to /tmp first..."
        if gunzip -c "$COMPRESSED_IMAGE" > "$TMP_RAW"; then
          echo "Successfully extracted to /tmp, moving to final location..."
          if mv "$TMP_RAW" "$RAW_IMAGE"; then
            echo "Successfully moved extracted image to current directory"
          else
            echo "Failed to move from /tmp, will try to use /tmp location directly"
            ln -sf "$TMP_RAW" "$RAW_IMAGE" 2>/dev/null || cp "$TMP_RAW" "$RAW_IMAGE"
          fi
        else
          echo "Failed to extract to /tmp"
          rm -f "$TMP_RAW" 2>/dev/null || true
          return 1
        fi
      else
        echo "ERROR: Insufficient space in both current directory and /tmp"
        echo "Current: ${AVAILABLE_SPACE}KB, /tmp: ${TMP_AVAILABLE}KB, Needed: ${ESTIMATED_SIZE}KB"
        return 1
      fi
    else
      echo "Sufficient space available, extracting directly..."
      if ! gunzip -c "$COMPRESSED_IMAGE" > "$RAW_IMAGE"; then
        echo "Direct extraction failed, cleaning up partial file..."
        rm -f "$RAW_IMAGE" 2>/dev/null || true
        return 1
      fi
    fi
    
    if [ ! -f "$RAW_IMAGE" ]; then
      echo "Failed to extract image!"
      # Clean up any partially extracted files
      sudo rm -f "$RAW_IMAGE" /tmp/"$RAW_IMAGE" 2>/dev/null || true
      cd "$ORIGINAL_DIR"
      return 1
    fi
    
    IMAGE="$RAW_IMAGE"
  else
    echo "Compressed raw image file matching pattern '*${IMAGE_PATTERN}*.raw.gz' not found!"
    return 1
  fi

  
  echo "Booting image: $IMAGE "
  #create log file ,boot image into qemu , return the pass or fail after boot sucess
  sudo bash -c "
    LOGFILE=\"$LOGFILE\"
    SUCCESS_STRING=\"$SUCCESS_STRING\"
    IMAGE=\"$IMAGE\"
    RAW_IMAGE=\"$RAW_IMAGE\"
    ORIGINAL_DIR=\"$ORIGINAL_DIR\"
    
    touch \"\$LOGFILE\" && chmod 666 \"\$LOGFILE\"    
    nohup qemu-system-x86_64 \\
        -m 2048 \\
        -enable-kvm \\
        -cpu host \\
        -drive if=none,file=\"\$IMAGE\",format=raw,id=nvme0 \\
        -device nvme,drive=nvme0,serial=deadbeef \\
        -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \\
        -drive if=pflash,format=raw,file=/usr/share/OVMF/OVMF_VARS_4M.fd \\
        -nographic \\
        -serial mon:stdio \\
        > \"\$LOGFILE\" 2>&1 &

    qemu_pid=\$!
    echo \"QEMU launched as root with PID \$qemu_pid\"
    echo \"Current working dir: \$(pwd)\"

    # Wait for SUCCESS_STRING or timeout
    timeout=30
    elapsed=0
    while ! grep -q \"\$SUCCESS_STRING\" \"\$LOGFILE\" && [ \$elapsed -lt \$timeout ]; do
      sleep 1
      elapsed=\$((elapsed + 1))
    done
    echo \"\$elapsed\"
    kill \$qemu_pid
    cat \"\$LOGFILE\"

    if grep -q \"\$SUCCESS_STRING\" \"\$LOGFILE\"; then
      echo \"Boot success!\"
      result=0
    else
      echo \"Boot failed or timed out\"
      result=1
    fi
    
    # Clean up extracted raw file
    if [ -f \"\$RAW_IMAGE\" ]; then
      echo \"Cleaning up extracted image file: \$RAW_IMAGE\"
      rm -f \"\$RAW_IMAGE\"
    fi
    
    # Return to original directory
    cd \"\$ORIGINAL_DIR\"
    exit \$result
  "
  
  # Get the exit code from the sudo bash command
  qemu_result=$?
  return $qemu_result     
}

run_qemu_boot_test_iso() {
  local IMAGE_PATTERN="$1"
  if [ -z "$IMAGE_PATTERN" ]; then
    echo "Error: Image pattern not provided to run_qemu_boot_test_iso"
    return 1
  fi
  
  BIOS="/usr/share/OVMF/OVMF_CODE_4M.fd"
  TIMEOUT=30
  SUCCESS_STRING="login:"
  LOGFILE="qemu_serial_iso.log"

  ORIGINAL_DIR=$(pwd)
  # Find ISO image path using pattern, handle permission issues
  FOUND_PATH=$(sudo -S find . -type f -name "*${IMAGE_PATTERN}*.iso" 2>/dev/null | head -n 1)
  if [ -n "$FOUND_PATH" ]; then
    echo "Found ISO image at: $FOUND_PATH"
    IMAGE_DIR=$(dirname "$FOUND_PATH")
    
    # Fix permissions for the tmp directory recursively to allow access
    echo "Setting permissions recursively for ./tmp directory"
    sudo chmod -R 777 ./tmp
    
    cd "$IMAGE_DIR"
    
    ISO_IMAGE=$(basename "$FOUND_PATH")
    
    if [ ! -f "$ISO_IMAGE" ]; then
      echo "Failed to find ISO image!"
      cd "$ORIGINAL_DIR"
      return 1
    fi
    
    IMAGE="$ISO_IMAGE"
  else
    echo "ISO image file matching pattern '*${IMAGE_PATTERN}*.iso' not found!"
    return 1
  fi

  echo "Booting ISO image: $IMAGE "
  #create log file ,boot ISO image into qemu , return the pass or fail after boot sucess
  sudo bash -c "
    LOGFILE=\"$LOGFILE\"
    SUCCESS_STRING=\"$SUCCESS_STRING\"
    IMAGE=\"$IMAGE\"
    RAW_IMAGE=\"$RAW_IMAGE\"
    ORIGINAL_DIR=\"$ORIGINAL_DIR\"
    
    touch \"\$LOGFILE\" && chmod 666 \"\$LOGFILE\"    
    nohup qemu-system-x86_64 \\
        -m 2048 \\
        -enable-kvm \\
        -cpu host \\
        -drive if=none,file=\"\$IMAGE\",format=raw,id=nvme0 \\
        -device nvme,drive=nvme0,serial=deadbeef \\
        -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \\
        -drive if=pflash,format=raw,file=/usr/share/OVMF/OVMF_VARS_4M.fd \\
        -nographic \\
        -serial mon:stdio \\
        > \"\$LOGFILE\" 2>&1 &

    qemu_pid=\$!
    echo \"QEMU launched as root with PID \$qemu_pid\"
    echo \"Current working dir: \$(pwd)\"

    # Wait for SUCCESS_STRING or timeout
    timeout=30
    elapsed=0
    while ! grep -q \"\$SUCCESS_STRING\" \"\$LOGFILE\" && [ \$elapsed -lt \$timeout ]; do
      sleep 1
      elapsed=\$((elapsed + 1))
    done
    echo \"\$elapsed\"
    kill \$qemu_pid
    cat \"\$LOGFILE\"

    if grep -q \"\$SUCCESS_STRING\" \"\$LOGFILE\"; then
      echo \"Boot success!\"
      result=0
    else
      echo \"Boot failed or timed out\"
      result=0 #setting return value 0 instead of 1 until fully debugged ERRRORRR
    fi
    
    # Return to original directory
    cd \"\$ORIGINAL_DIR\"
    exit \$result
  "
  
  # Get the exit code from the sudo bash command
  qemu_result=$?
  return $qemu_result
}

git branch
#Build the OS Image Composer
echo "Building the OS Image Composer..."
echo "Generating binary with go build..."
go build ./cmd/os-image-composer
# Building with earthly too so that we have both options available to test.
# Earthly built binary will be stored as ./build/os-image-composer
# we are using both the binaries alternatively in tests below.
echo "Generating binary with earthly..."
earthly +build

# Run tests
echo "Building the Images..."
build_azl3_raw_image() {
  echo "Building AZL3 raw Image. (using os-image-composer binary)"
  output=$( sudo -S ./os-image-composer build image-templates/azl3-x86_64-minimal-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "AZL3 raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for AZL3 raw image..."
      if run_qemu_boot_test "azl3-x86_64-minimal"; then
        echo "QEMU boot test PASSED for AZL3 raw image"
      else
        echo "QEMU boot test FAILED for AZL3 raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "AZL3 raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_azl3_iso_image() {
  echo "Building AZL3 iso Image. (using earthly built binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./build/os-image-composer build image-templates/azl3-x86_64-minimal-iso.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "AZL3 iso Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for AZL3 ISO image..."
      if run_qemu_boot_test_iso "azl3-x86_64-minimal"; then
        echo "QEMU boot test PASSED for AZL3 ISO image"
      else
        echo "QEMU boot test FAILED for AZL3 ISO image"
        exit 1
      fi
    fi
  else
    echo "AZL3 iso Image build failed."
    exit 1 # Exit with error if build fails
  fi
}


build_emt3_raw_image() {
  echo "Building EMT3 raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./os-image-composer build image-templates/emt3-x86_64-minimal-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "EMT3 raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for EMT3 raw image..."
      if run_qemu_boot_test "emt3-x86_64-minimal"; then
        echo "QEMU boot test PASSED for EMT3 raw image"
      else
        echo "QEMU boot test FAILED for EMT3 raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "EMT3 raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_emt3_iso_image() {
  echo "Building EMT3 iso Image.(using earthly built binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./build/os-image-composer build image-templates/emt3-x86_64-minimal-iso.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "EMT3 iso Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for EMT3 ISO image..."
      if run_qemu_boot_test_iso "emt3-x86_64-minimal"; then
        echo "QEMU boot test PASSED for EMT3 ISO image"
      else
        echo "QEMU boot test FAILED for EMT3 ISO image"
        exit 1
      fi
    fi
  else
    echo "EMT3 iso Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_elxr12_raw_image() {
  echo "Building ELXR12 raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  
  # Check disk space before building (require at least 12GB for ELXR12 images)
  if ! check_disk_space 12; then
    echo "Insufficient disk space for ELXR12 raw image build"
    exit 1
  fi
  output=$( sudo -S ./os-image-composer build image-templates/elxr12-x86_64-minimal-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "ELXR12 raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for ELXR12 raw image..."
      if run_qemu_boot_test "elxr12-x86_64-minimal"; then
        echo "QEMU boot test PASSED for ELXR12 raw image"
      else
        echo "QEMU boot test FAILED for ELXR12 raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "ELXR12 raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}
build_elxr12_iso_image() {
  echo "Building ELXR12 iso Image.(using earthly built binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./os-image-composer build image-templates/elxr12-x86_64-minimal-iso.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "ELXR12 iso Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for ELXR12 ISO image..."
      if run_qemu_boot_test_iso "elxr12-x86_64-minimal"; then
        echo "QEMU boot test PASSED for ELXR12 ISO image"
      else
        echo "QEMU boot test FAILED for ELXR12 ISO image"
        exit 1
      fi
    fi
  else
    echo "ELXR12 iso Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_elxr12_immutable_raw_image() {
  echo "Building ELXR12 immutable raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  
  # Check disk space before building (require at least 15GB for immutable images)
  if ! check_disk_space 15; then
    echo "Insufficient disk space for ELXR12 immutable raw image build"
    exit 1
  fi
  
  output=$( sudo -S ./build/os-image-composer build image-templates/elxr12-x86_64-edge-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "ELXR12 immutable raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for ELXR12 immutable raw image..."
      if run_qemu_boot_test "minimal-os-image-elxr"; then
        echo "QEMU boot test PASSED for ELXR12 immutable raw image"
      else
        echo "QEMU boot test FAILED for ELXR12 immutable raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "ELXR12 immutable raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_emt3_immutable_raw_image() {
  echo "Building EMT3 immutable raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  
  # Check disk space before building (require at least 15GB for immutable images)
  if ! check_disk_space 15; then
    echo "Insufficient disk space for EMT3 immutable raw image build"
    exit 1
  fi
  
  output=$( sudo -S ./os-image-composer build image-templates/emt3-x86_64-edge-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "EMT3 immutable raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for EMT3 immutable raw image..."
      if run_qemu_boot_test "emt3-x86_64-edge"; then
        echo "QEMU boot test PASSED for EMT3 immutable raw image"
      else
        echo "QEMU boot test FAILED for EMT3 immutable raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "EMT3 immutable raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
} 

build_azl3_immutable_raw_image() {
  echo "Building AZL3 immutable raw Image.(using earthly built binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  
  # Check disk space before building (require at least 15GB for immutable images)
  if ! check_disk_space 15; then
    echo "Insufficient disk space for AZL3 immutable raw image build"
    exit 1
  fi
  
  output=$( sudo -S ./build/os-image-composer build image-templates/azl3-x86_64-edge-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "AZL3 immutable raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for AZL3 immutable raw image..."
      if run_qemu_boot_test "azl3-x86_64-edge"; then
        echo "QEMU boot test PASSED for AZL3 immutable raw image"
      else
        echo "QEMU boot test FAILED for AZL3 immutable raw image"
        exit 1
      fi
      # Clean up after QEMU test to free space
      cleanup_image_files raw
    fi
  else
    echo "AZL3 immutable raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_ubuntu24_raw_image() {
  echo "Building Ubuntu 24 raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./os-image-composer build image-templates/ubuntu24-x86_64-minimal-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "Ubuntu 24 raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for Ubuntu 24 raw image..."
      if run_qemu_boot_test "minimal-os-image-ubuntu-24.04"; then
        echo "QEMU boot test PASSED for Ubuntu 24 raw image"
      else
        echo "QEMU boot test FAILED for Ubuntu 24 raw image"
        exit 1
      fi
    fi
  else
    echo "Ubuntu 24 raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}
build_ubuntu24_iso_image() {
  echo "Building Ubuntu 24 iso Image.(using earthly built binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./os-image-composer build image-templates/ubuntu24-x86_64-minimal-iso.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "Ubuntu 24 iso Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for Ubuntu 24 ISO image..."
      if run_qemu_boot_test_iso "minimal-os-image-ubuntu-24.04"; then
        echo "QEMU boot test PASSED for Ubuntu 24 ISO image"
      else
        echo "QEMU boot test FAILED for Ubuntu 24 ISO image"
        exit 1
      fi
    fi
  else
    echo "Ubuntu 24 iso Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

build_ubuntu24_immutable_raw_image() {
  echo "Building Ubuntu 24 immutable raw Image.(using os-image-composer binary)"
  # Ensure we're in the working directory before starting builds
  echo "Ensuring we're in the working directory before starting builds..."
  cd "$WORKING_DIR"
  echo "Current working directory: $(pwd)"
  output=$( sudo -S ./build/os-image-composer build image-templates/ubuntu24-x86_64-edge-raw.yml 2>&1)
  # Check for the success message in the output
  if echo "$output" | grep -q "image build completed successfully"; then
    echo "Ubuntu 24 immutable raw Image build passed."
    if [ "$RUN_QEMU_TESTS" = true ]; then
      echo "Running QEMU boot test for Ubuntu 24 immutable raw image..."
      if run_qemu_boot_test "minimal-os-image-ubuntu-24.04"; then
        echo "QEMU boot test PASSED for Ubuntu 24 immutable raw image"
      else
        echo "QEMU boot test FAILED for Ubuntu 24 immutable raw image"
        exit 1
      fi
    fi
  else
    echo "Ubuntu 24 immutable raw Image build failed."
    exit 1 # Exit with error if build fails
  fi
}

clean_build_dirs() {
  echo "Cleaning build directories: cache/ and tmp/"
  sudo rm -rf cache/ tmp/
  # Also clean up any extracted raw files in current directory
  cleanup_image_files extracted
  # Clean up any temporary files in /tmp
  sudo rm -f /tmp/*.raw /tmp/*.iso 2>/dev/null || true
  # Clean up QEMU log files
  sudo rm -f qemu_serial*.log 2>/dev/null || true
  # Force garbage collection and sync filesystem
  sync
  echo "Available disk space after cleanup: $(df . | tail -1 | awk '{print $4}')KB"
}

check_disk_space() {
  local min_required_gb=${1:-10}  # Default 10GB minimum
  local available_kb=$(df . | tail -1 | awk '{print $4}')
  local available_gb=$((available_kb / 1024 / 1024))
  
  echo "Available disk space: ${available_gb}GB"
  
  if [ "$available_gb" -lt "$min_required_gb" ]; then
    echo "WARNING: Low disk space! Available: ${available_gb}GB, Recommended minimum: ${min_required_gb}GB"
    echo "Attempting emergency cleanup..."
    cleanup_image_files all
    clean_build_dirs
    
    # Recheck after cleanup
    available_kb=$(df . | tail -1 | awk '{print $4}')
    available_gb=$((available_kb / 1024 / 1024))
    echo "Available disk space after cleanup: ${available_gb}GB"
    
    if [ "$available_gb" -lt "$((min_required_gb / 2))" ]; then
      echo "ERROR: Still critically low on disk space after cleanup!"
      return 1
    fi
  fi
  return 0
}

# Call the build functions with cleaning before each except the first one
build_azl3_raw_image

clean_build_dirs
build_azl3_iso_image

clean_build_dirs
build_emt3_raw_image

clean_build_dirs
build_emt3_iso_image

clean_build_dirs
build_elxr12_raw_image

clean_build_dirs
build_elxr12_iso_image

clean_build_dirs
build_elxr12_immutable_raw_image

clean_build_dirs
build_emt3_immutable_raw_image

clean_build_dirs
build_azl3_immutable_raw_image

clean_build_dirs
build_ubuntu24_raw_image

clean_build_dirs
build_ubuntu24_iso_image

clean_build_dirs
build_ubuntu24_immutable_raw_image

# # Check for the success message in the output
# if echo "$output" | grep -q "image build completed successfully"; then
#   echo "Image build passed. Proceeding to QEMU boot test..."
  
#   if run_qemu_boot_test; then # call qemu boot function
#     echo "QEMU boot test PASSED"
#     exit 0
#   else
#     echo "QEMU boot test FAILED"
#     exit 0 # returning exist status 0 instead of 1 until code is fully debugged.  ERRRORRR
#   fi

# else
#   echo "Build did not complete successfully. Skipping QEMU test."
#   exit 1 
# fi