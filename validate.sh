#!/bin/bash
set -e
# Expect to be run from the root of the PR branch
echo "Current working dir: $(pwd)"


run_qemu_boot_test() {
  cd /home/user/image-composer/tmp/image-composer/azure-linux-azl3-x86_64/imagebuild/Default
  IMAGE="azl3-default-x86_64.raw"  # image file
  BIOS="/usr/share/OVMF/OVMF_CODE.fd"
  TIMEOUT=30
  SUCCESS_STRING="login:"
  LOGFILE="qemu_serial.log"
  ORIGINAL_IMAGE="azl3-default-x86_64.raw"
  COPY_IMAGE="/tmp/azl3-test.raw"


  echo "Booting image: $IMAGE "
  echo "Current working dir: $(pwd)"
  
  cp "$ORIGINAL_IMAGE" "$COPY_IMAGE" 
  sudo bash -c "touch '$LOGFILE' && chmod 666 '$LOGFILE'"

  # echo "$sudo_pwd" | sudo qemu-system-x86_64 -m 2048 -enable-kvm -cpu host   -drive if=none,file=$IMAGE,format=raw,id=nvme0   -device nvme,drive=nvme0,serial=deadbeef   -bios /usr/share/OVMF/OVMF_CODE.fd   -nographic -serial mon:stdio > "$LOGFILE" >2

sudo bash -c '
  nohup qemu-system-x86_64 \
    -m 2048 \
    -enable-kvm \
    -cpu host \
    -drive if=none,file="'$COPY_IMAGE'",format=raw,id=nvme0 \
    -device nvme,drive=nvme0,serial=deadbeef \
    -bios "'$BIOS'" \
    -nographic \
    -serial mon:stdio \
    > "'$LOGFILE'" 2>&1 &

  qemu_pid=$!
  echo "QEMU launched as root with PID $qemu_pid"

  # Wait for SUCCESS_STRING or timeout
    timeout=60
    elapsed=0
    while ! grep -q "'$SUCCESS_STRING'" "'$LOGFILE'" && [ $elapsed -lt $timeout ]; do
      sleep 1
      elapsed=$((elapsed + 1))
    done

    if grep -q "'$SUCCESS_STRING'" "'$LOGFILE'"; then
      echo "Boot success!"
      result=0
    else
      echo "Boot failed or timed out"
      result=1
    fi

    kill $qemu_pid
    exit $result
'
}

git branch
#Build the ICT
echo "Building the Image Composer Tool..."
go build ./cmd/image-composer

# Run tests

echo "Building the Linux image..."


#output=$(echo $sudo_pwd | sudo -S ./image-composer build image-templates/azl3-x86_64-edge-raw.yml)
sudo_pwd=user1234
output=$(echo $sudo_pwd | sudo -S ./image-composer build config/osv/azure-linux/azl3/imageconfigs/defaultconfigs/default-raw-x86_64.yml 2>&1)

# Check for the success message in the output

if echo "$output" | grep -q "image build completed successfully"; then
  echo "Image build passed. Proceeding to QEMU boot test..."
  
  if create_vm_aszl3; then
    echo "QEMU boot test PASSED"
  else
    echo "QEMU boot test FAILED"
  fi

else
  echo "Build did not complete successfully. Skipping QEMU test."
fi
