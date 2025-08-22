#!/bin/bash
# Script to upgrade Go to 1.23+ to support iter package

set -e

echo "=== Upgrading Go to 1.23+ ==="

# Check current Go version
CURRENT_GO=$(go version 2>/dev/null || echo "Go not found")
echo "Current Go: $CURRENT_GO"

# Download Go 1.23.4
GO_VERSION="1.23.4"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"

echo "Downloading Go $GO_VERSION..."
wget -q --show-progress "$GO_URL"

# Remove old Go installation
echo "Removing old Go installation..."
sudo rm -rf /usr/local/go

# Install new Go
echo "Installing Go $GO_VERSION..."
sudo tar -C /usr/local -xzf "$GO_TARBALL"

# Clean up download
rm "$GO_TARBALL"

# Verify installation
echo "Verifying installation..."
NEW_GO_VERSION=$(/usr/local/go/bin/go version)
echo "New Go: $NEW_GO_VERSION"

# Update PATH if needed
if ! echo "$PATH" | grep -q "/usr/local/go/bin"; then
    echo "Adding Go to PATH..."
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
fi

# Update go.mod
echo "Updating go.mod..."
if [[ -f "go.mod" ]]; then
    # Backup go.mod
    cp go.mod go.mod.backup
    
    # Update go version in go.mod
    sed -i 's/go [0-9]\+\.[0-9]\+\.[0-9]\+/go 1.23/' go.mod
    sed -i 's/go [0-9]\+\.[0-9]\+/go 1.23/' go.mod
    
    echo "✓ Updated go.mod to require Go 1.23"
    echo "✓ Backed up original go.mod to go.mod.backup"
else
    echo "⚠ No go.mod found"
fi

# Test the installation
echo ""
echo "Testing Go installation..."

if go version >/dev/null 2>&1; then
    echo "✅ go version: $(go version)"
else
    echo "❌ Go command not working"
    exit 1
fi

if go mod tidy >/dev/null 2>&1; then
    echo "✅ go mod tidy successful"
else
    echo "❌ go mod tidy failed:"
    go mod tidy
    exit 1
fi

if go list ./... >/dev/null 2>&1; then
    echo "✅ go list ./... successful"
else
    echo "❌ go list ./... failed:"
    go list ./...
    exit 1
fi

if go build ./... >/dev/null 2>&1; then
    echo "✅ go build ./... successful"
else
    echo "❌ go build ./... failed. First few errors:"
    go build ./... 2>&1 | head -n 5
    exit 1
fi

echo ""
echo "🎉 Go 1.23+ upgrade completed successfully!"
echo ""
echo "Next steps:"
echo "1. Open a new terminal or run: source ~/.bashrc"
echo "2. Verify: go version"
echo "3. Test coverage: ./scripts/run_coverage_tests.sh"
echo ""
echo "If you need to revert go.mod: mv go.mod.backup go.mod"
