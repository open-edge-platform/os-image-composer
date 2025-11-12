VERSION 0.8

LOCALLY
ARG http_proxy=$(echo $http_proxy)
ARG https_proxy=$(echo $https_proxy)
ARG no_proxy=$(echo $no_proxy)
ARG HTTP_PROXY=$(echo $HTTP_PROXY)
ARG HTTPS_PROXY=$(echo $HTTPS_PROXY)
ARG NO_PROXY=$(echo $NO_PROXY)
ARG REGISTRY

# Use pre-built Go image that already has most tools
FROM ${REGISTRY}golang:1.24.1-bullseye

ENV http_proxy=$http_proxy
ENV https_proxy=$https_proxy
ENV no_proxy=$no_proxy
ENV HTTP_PROXY=$HTTP_PROXY
ENV HTTPS_PROXY=$HTTPS_PROXY
ENV NO_PROXY=$NO_PROXY

# Set Go environment variables (already set in golang image, but ensure they're correct)
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV GOBIN="/go/bin"
ENV PATH="${GOBIN}:${PATH}"

# The golang image already includes:
# - wget, curl, git, build-essential
# - Most basic tools
# - Go 1.24.1

# Only install absolutely essential packages that might be missing
# Use --no-install-recommends and || true to continue even if some fail
RUN apt-get update && apt-get install -y --no-install-recommends \
    bc bash rpm mmdebstrap dosfstools sbsigntool xorriso grub-common cryptsetup \
    || echo "Some packages failed to install, continuing..."

RUN ln -s /bin/uname /usr/bin/uname

golang-base:
    # Create Go workspace
    RUN mkdir -p /go/src /go/bin /go/pkg && chmod -R 777 /go
    
    # Install golangci-lint
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.7
    
    WORKDIR /work
    COPY go.mod .
    COPY go.sum .
    RUN go mod download # for caching
    COPY cmd/ ./cmd
    COPY internal/ ./internal
    COPY image-templates/ ./image-templates

all:
    BUILD +build

build:
    FROM +golang-base
    
    # Copy .git directory to enable git commands
    COPY .git .git
    
    # Detect version from git tags
    RUN VERSION=$(git tag --sort=-creatordate | head -n1 2>/dev/null || echo "dev") && \
        echo "$VERSION" > /tmp/version
    
    # Get git commit SHA
    RUN COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
        echo "$COMMIT_SHA" > /tmp/commit_sha
    
    # Get build date in UTC
    RUN BUILD_DATE=$(date -u '+%Y-%m-%d') && \
        echo "$BUILD_DATE" > /tmp/build_date

    # Build with variables instead of cat substitution
    RUN VERSION=$(cat /tmp/version) && \
        COMMIT_SHA=$(cat /tmp/commit_sha) && \
        BUILD_DATE=$(cat /tmp/build_date) && \
        CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
        go build -trimpath -buildmode=pie -o build/live-installer \
            -ldflags "-s -w \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Version=$VERSION' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Toolname=Image-Composer' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Organization=Open Edge Platform' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.BuildDate=$BUILD_DATE' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.CommitSHA=$COMMIT_SHA'" \
            ./cmd/live-installer

    SAVE ARTIFACT build/live-installer AS LOCAL ./build/live-installer
    
    # Build with variables instead of cat substitution
    RUN VERSION=$(cat /tmp/version) && \
        COMMIT_SHA=$(cat /tmp/commit_sha) && \
        BUILD_DATE=$(cat /tmp/build_date) && \
        CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
        go build -trimpath -buildmode=pie -o build/os-image-composer \
            -ldflags "-s -w \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Version=$VERSION' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Toolname=Image-Composer' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Organization=Open Edge Platform' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.BuildDate=$BUILD_DATE' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.CommitSHA=$COMMIT_SHA'" \
            ./cmd/os-image-composer
            
    SAVE ARTIFACT build/os-image-composer AS LOCAL ./build/os-image-composer

lint:
    FROM +golang-base
    WORKDIR /work
    COPY . /work
    RUN --mount=type=cache,target=/root/.cache \
        golangci-lint run ./...

test:
    FROM +golang-base
    ARG PRINT_TS=""
    ARG FAIL_ON_NO_TESTS=false
    
    # Copy the entire project (including scripts directory)
    COPY . /work
    
    # Make the coverage script executable
    RUN chmod +x /work/scripts/run_coverage_tests.sh
    
    # Run the comprehensive coverage tests using our script
    RUN cd /work && ./scripts/run_coverage_tests.sh "${PRINT_TS}" "${FAIL_ON_NO_TESTS}"
    
    # Save all generated artifacts locally
    SAVE ARTIFACT coverage.out AS LOCAL ./coverage.out
    SAVE ARTIFACT coverage_total.txt AS LOCAL ./coverage_total.txt
    SAVE ARTIFACT coverage_packages.txt AS LOCAL ./coverage_packages.txt
    SAVE ARTIFACT test_raw.log AS LOCAL ./test_raw.log

test-debug:
    FROM +golang-base
    ARG PRINT_TS=""
    ARG FAIL_ON_NO_TESTS=false
    
    # Copy the entire project (including scripts directory)
    COPY . /work
    
    # Make the coverage script executable
    RUN chmod +x /work/scripts/run_coverage_tests.sh
    
    # Run the coverage tests with debug output
    RUN cd /work && ./scripts/run_coverage_tests.sh "${PRINT_TS}" "${FAIL_ON_NO_TESTS}" "true"
    
    # Save all generated artifacts locally
    SAVE ARTIFACT coverage.out AS LOCAL ./coverage.out
    SAVE ARTIFACT coverage_total.txt AS LOCAL ./coverage_total.txt
    SAVE ARTIFACT coverage_packages.txt AS LOCAL ./coverage_packages.txt
    SAVE ARTIFACT test_raw.log AS LOCAL ./test_raw.log

test-quick:
    FROM +golang-base
    RUN go test ./...

deb:
    FROM debian:bookworm-slim
    ARG VERSION=1.0.0
    ARG ARCH=amd64
    
    WORKDIR /pkg
    
    # Create directory structure following FHS (Filesystem Hierarchy Standard)
    RUN mkdir -p usr/local/bin \
                 etc/os-image-composer/config \
                 usr/share/os-image-composer/examples \
                 usr/share/doc/os-image-composer \
                 var/cache/os-image-composer \
                 DEBIAN
    
    # Copy the built binary from the build target
    COPY +build/os-image-composer usr/local/bin/os-image-composer
    
    # Make the binary executable
    RUN chmod +x usr/local/bin/os-image-composer
    
    # Create default global configuration with system paths (user-editable)
    # Note: Must be named config.yml to match the default search paths in the code
    RUN echo "# OS Image Composer - Global Configuration" > etc/os-image-composer/config.yml && \
        echo "# This file contains tool-level settings that apply across all image builds." >> etc/os-image-composer/config.yml && \
        echo "# Image-specific parameters should be defined in the image specification." >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "# Core tool settings" >> etc/os-image-composer/config.yml && \
        echo "workers: 8" >> etc/os-image-composer/config.yml && \
        echo "# Number of concurrent download workers (1-100, default: 8)" >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "config_dir: \"/etc/os-image-composer/config\"" >> etc/os-image-composer/config.yml && \
        echo "# Directory containing configuration files for different target OSs" >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "cache_dir: \"/var/cache/os-image-composer\"" >> etc/os-image-composer/config.yml && \
        echo "# Package cache directory where downloaded RPMs/DEBs are stored" >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "work_dir: \"/tmp/os-image-composer\"" >> etc/os-image-composer/config.yml && \
        echo "# Working directory for build operations and image assembly" >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "temp_dir: \"/tmp\"" >> etc/os-image-composer/config.yml && \
        echo "# Temporary directory for short-lived files" >> etc/os-image-composer/config.yml && \
        echo "" >> etc/os-image-composer/config.yml && \
        echo "# Logging configuration" >> etc/os-image-composer/config.yml && \
        echo "logging:" >> etc/os-image-composer/config.yml && \
        echo "  level: \"info\"" >> etc/os-image-composer/config.yml && \
        echo "  # Log verbosity level: debug, info, warn, error" >> etc/os-image-composer/config.yml
    
    # Copy OS variant configuration files (user-editable)
    COPY config etc/os-image-composer/config
    
    # Copy image templates as examples (read-only, for reference)
    COPY image-templates usr/share/os-image-composer/examples
    
    # Copy documentation
    COPY README.md usr/share/doc/os-image-composer/
    COPY LICENSE usr/share/doc/os-image-composer/
    COPY docs/architecture/os-image-composer-cli-specification.md usr/share/doc/os-image-composer/
    
    # Create the DEBIAN control file with proper metadata
    RUN echo "Package: os-image-composer" > DEBIAN/control && \
        echo "Version: ${VERSION}" >> DEBIAN/control && \
        echo "Section: utils" >> DEBIAN/control && \
        echo "Priority: optional" >> DEBIAN/control && \
        echo "Architecture: ${ARCH}" >> DEBIAN/control && \
        echo "Maintainer: Intel Edge Software Team <edge.platform@intel.com>" >> DEBIAN/control && \
        echo "Depends: bash, coreutils, unzip, dosfstools, xorriso, grub-common" >> DEBIAN/control && \
        echo "Recommends: mmdebstrap, debootstrap" >> DEBIAN/control && \
        echo "License: MIT" >> DEBIAN/control && \
        echo "Description: OS Image Composer (OIC)" >> DEBIAN/control && \
        echo " OIC enables users to compose custom bootable OS images based on a" >> DEBIAN/control && \
        echo " user-provided template that specifies package lists, configurations," >> DEBIAN/control && \
        echo " and output formats for supported distributions." >> DEBIAN/control
    
    # Build the debian package
    RUN dpkg-deb --build . os-image-composer_${VERSION}_${ARCH}.deb
    
    # Save the debian package artifact to dist/ directory
    SAVE ARTIFACT os-image-composer_${VERSION}_${ARCH}.deb AS LOCAL dist/os-image-composer_${VERSION}_${ARCH}.deb
