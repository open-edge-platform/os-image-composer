#!/bin/bash
set -e

# Redirect all output to a custom log as well so you don't just rely on cloud-init logs
exec > >(tee -a /var/log/ollama-setup-trace.log) 2>&1

echo "--- Setup started at $(date) ---"

# 0. Setup Environment Variables
# These prevent the "panic: $HOME is not defined" error
export HOME=/root
export USER=root
export XDG_CONFIG_HOME=/root/.config
# Ensure the path includes standard binary locations
export PATH=$PATH:/usr/local/bin:/usr/bin:/bin

# 1. Wait for Apt Lock (Crucial for cloud-init)
echo "Waiting for apt locks to release..."
while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
    sleep 5
done

# 2. Ensure Curl is installed
if ! command -v curl >/dev/null 2>&1; then
    apt-get install -y curl
fi

# 3. Use the Official Install script but ensure it's non-interactive
echo "Downloading and running Ollama install script..."
curl -fsSL https://ollama.com/install.sh | sh

# 4. Final check
if command -v ollama >/dev/null 2>&1; then
    echo "✓ Ollama installed successfully"
    ollama --version
else
    echo "✗ Ollama install failed to register in PATH"
    exit 1
fi

# 5. Start Ollama service in the background
echo "Starting Ollama service..."
nohup ollama serve > /var/log/ollama-service.log 2>&1 &
echo "✓ Ollama service started in background"

# 6. Pull tinyllama model
echo "Waiting 20 seconds before starting Ollama service..."
sleep 20
echo "Pulling tinyllama model..."
ollama pull tinyllama
echo "✓ tinyllama model pulled successfully"