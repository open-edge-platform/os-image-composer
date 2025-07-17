#!/bin/bash
set -e
# Expect to be run from the root of the PR branch
echo "Current working dir: $(pwd)"

run_qemu_boot_test() {
  IMAGE="azl3-default-x86_64.raw"  # image file
  BIOS="/usr/share/OVMF/OVMF_CODE_4M.fd"
  TIMEOUT=30
  SUCCESS_STRING="login:"
  LOGFILE="qemu_serial.log"


  ORIGINAL_DIR=$(pwd)
  # Find image path
  FOUND_PATH=$(find . -type f -name "$IMAGE" | head -n 1)
  if [ -n "$FOUND_PATH" ]; then
    echo "Found image at: $FOUND_PATH"   
    IMAGE_DIR=$(dirname "$FOUND_PATH")  # Extract directory path where image resides   
    cd "$IMAGE_DIR"  # Change to that directory
  else
    echo "Image file not found!"
    exit 1
  fi

  
  echo "Booting image: $IMAGE "
  #create log file ,boot image into qemu , return the pass or fail after boot sucess
  sudo bash -c 'touch "'$LOGFILE'" && chmod 666 "'$LOGFILE'"    
  nohup qemu-system-x86_64 \
      -m 2048 \
      -enable-kvm \
      -cpu host \
      -drive if=none,file="'$IMAGE'",format=raw,id=nvme0 \
      -device nvme,drive=nvme0,serial=deadbeef \
      -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd \
      -drive if=pflash,format=raw,file=/usr/share/OVMF/OVMF_VARS_4M.fd \
      -nographic \
      -serial mon:stdio \
      > "'$LOGFILE'" 2>&1 &

    qemu_pid=$!
    echo "QEMU launched as root with PID $qemu_pid"
    echo "Current working dir: $(pwd)"

    # Wait for SUCCESS_STRING or timeout
      timeout=30
      elapsed=0
      while ! grep -q "'$SUCCESS_STRING'" "'$LOGFILE'" && [ $elapsed -lt $timeout ]; do
        sleep 1
        elapsed=$((elapsed + 1))
      done
      echo "$elapsed"
      kill $qemu_pid
      cat "'$LOGFILE'"

      if grep -q "'$SUCCESS_STRING'" "'$LOGFILE'"; then
        echo "Boot success!"
        result=0
      else
        echo "Boot failed or timed out"
        result=1
      fi    
      exit $result
  '     
}

git branch
#Build the ICT
echo "Building the Image Composer Tool..."
go build ./cmd/image-composer

# Run tests
echo "Building the Linux image..."
output=$( sudo -S ./image-composer build config/osv/azure-linux/azl3/imageconfigs/defaultconfigs/default-raw-x86_64.yml 2>&1)
# Check for the success message in the output
if echo "$output" | grep -q "image build completed successfully"; then
  echo "Image build passed. Proceeding to QEMU boot test..."
  
  if run_qemu_boot_test; then # call qemu boot function
    echo "QEMU boot test PASSED"
    exit 0
  else
    echo "QEMU boot test FAILED"
    exit 1
  fi

else
  echo "Build did not complete successfully. Skipping QEMU test."
  exit 1
fi
