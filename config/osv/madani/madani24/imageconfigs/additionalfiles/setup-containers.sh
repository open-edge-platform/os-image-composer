#!/bin/bash

# Redirect all output to a custom log as well so you don't just rely on cloud-init logs
exec > >(tee -a /var/log/podman-jupyter-setup-trace.log) 2>&1

echo "--- Podman JupyterLab Setup started at $(date) ---"

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

# 2. Update package lists
echo "Updating package lists..."
apt update

# 3. Ensure Curl and necessary packages are installed
if ! command -v curl >/dev/null 2>&1; then
    apt-get install -y curl
fi

if ! command -v wget >/dev/null 2>&1; then
    apt-get install -y wget
fi

if ! command -v chromium-browser >/dev/null 2>&1; then
    echo "Installing Chromium browser..."
    apt-get install -y chromium-browser
fi

# 4. Install Podman if not found
if ! command -v podman >/dev/null 2>&1; then
    echo "Installing Podman..."
    apt-get install -y podman
    
    # Configure Podman for rootless operation
    echo "Configuring Podman..."
    mkdir -p /etc/containers
    echo 'unqualified-search-registries = ["docker.io"]' > /etc/containers/registries.conf
    
    # Enable lingering for root user to allow containers to run after logout
    loginctl enable-linger root
else
    echo "✓ Podman already installed"
fi

# 5. Final check for Podman installation
if command -v podman >/dev/null 2>&1; then
    echo "✓ Podman installed successfully"
    podman --version
else
    echo "✗ Podman install failed"
    exit 1
fi

# 6. Create JupyterLab workspace directory
echo "Creating JupyterLab workspace directory..."
mkdir -p /opt/jupyter/workspace
chmod 755 /opt/jupyter/workspace

# 7. Pull JupyterLab image
echo "Pulling JupyterLab Docker image..."
podman pull docker.io/jupyter/scipy-notebook:latest

# 8. Create systemd service for JupyterLab container
echo "Creating systemd service for JupyterLab..."
cat > /etc/systemd/system/jupyterlab.service << EOF
[Unit]
Description=JupyterLab Container
After=network.target
Wants=network.target

[Service]
Type=exec
ExecStartPre=-/usr/bin/podman stop jupyterlab
ExecStartPre=-/usr/bin/podman rm jupyterlab
ExecStart=/usr/bin/podman run --name jupyterlab \\
    -p 8888:8888 \\
    -v /opt/jupyter/workspace:/home/jovyan/work:Z \\
    -e JUPYTER_ENABLE_LAB=yes \\
    -e JUPYTER_TOKEN='' \\
    --restart=unless-stopped \\
    docker.io/jupyter/scipy-notebook:latest \\
    start-notebook.sh --NotebookApp.token='' --NotebookApp.password=''
ExecStop=/usr/bin/podman stop jupyterlab
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# 9. Enable and start JupyterLab service
echo "Enabling and starting JupyterLab service..."
systemctl daemon-reload
systemctl enable jupyterlab
systemctl start jupyterlab

echo "Waiting 10 seconds for JupyterLab to start..."
sleep 10

# 10. Verify JupyterLab is running
if systemctl is-active --quiet jupyterlab; then
    echo "✓ JupyterLab service started successfully"
    echo "✓ JupyterLab is accessible at http://localhost:8888"
    echo "✓ Workspace directory: /opt/jupyter/workspace"
    echo "✓ No token required for access"
else
    echo "✗ JupyterLab service failed to start"
    systemctl status jupyterlab
    exit 1
fi