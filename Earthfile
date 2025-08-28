VERSION 0.8

LOCALLY
ARG http_proxy=$(echo $http_proxy)
ARG https_proxy=$(echo $https_proxy)
ARG no_proxy=$(echo $no_proxy)
ARG HTTP_PROXY=$(echo $HTTP_PROXY)
ARG HTTPS_PROXY=$(echo $HTTPS_PROXY)
ARG NO_PROXY=$(echo $NO_PROXY)
ARG REGISTRY

FROM ${REGISTRY}ubuntu:latest
ENV http_proxy=$http_proxy
ENV https_proxy=$https_proxy
ENV no_proxy=$no_proxy
ENV HTTP_PROXY=$HTTP_PROXY
ENV HTTPS_PROXY=$HTTPS_PROXY
ENV NO_PROXY=$NO_PROXY

# Install Go and other required package manually
ENV DEBIAN_FRONTEND=noninteractive
ENV DEBCONF_NONINTERACTIVE_SEEN=true

# Pre-configure debconf to avoid prompts
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# Install basic tools 
RUN apt-get update && apt-get install -y \
    wget curl git build-essential \
    && rm -rf /var/lib/apt/lists/*
    
# Install system tools
RUN apt-get update && apt-get install -y \
    util-linux e2fsprogs dosfstools \
    && rm -rf /var/lib/apt/lists/*
    
# Install compression tools
RUN apt-get update && apt-get install -y \
    bzip2 xz-utils zstd \
    && rm -rf /var/lib/apt/lists/*
   
# Install disk tools
RUN apt-get update && apt-get install -y \
    parted gdisk cryptsetup lvm2 psmisc \
    && rm -rf /var/lib/apt/lists/*
    
# Install package management tools
RUN apt-get update && apt-get install -y \
    dpkg-dev rpm lsb-release createrepo-c mmdebstrap \
    && rm -rf /var/lib/apt/lists/*

# Install boot and GRUB tools
RUN apt-get update && apt-get install -y \
    systemd-boot grub2-common grub-common grub-efi-amd64-bin dracut \
    && rm -rf /var/lib/apt/lists/*

# Install security and signing tools
RUN apt-get update && apt-get install -y \
    sbsigntool gnupg2 systemd-ukify \
    && rm -rf /var/lib/apt/lists/*
    
# Install virtualization and ISO tools
RUN apt-get update && apt-get install -y \
    xorriso qemu-utils qemu-system-x86 \
    && rm -rf /var/lib/apt/lists/*

# Download and install Go 1.24.1
RUN wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz \
    && rm go1.24.1.linux-amd64.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV GOBIN="/go/bin"
ENV PATH="${GOBIN}:${PATH}"

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

fetch-golang:
    RUN apt-get update && apt-get install -y curl && curl -fsSLO https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
    SAVE ARTIFACT go1.24.1.linux-amd64.tar.gz

build:
    FROM +golang-base
    ARG version='0.0.0-unknown'
    # Get build date in UTC
    RUN date -u '+%Y-%m-%d' > /tmp/build_date
    # Get git commit SHA if in a git repo, otherwise use "unknown"
    RUN if [ -d .git ]; then \
            git rev-parse --short HEAD > /tmp/commit_sha; \
        else \
            echo "unknown" > /tmp/commit_sha; \
        fi
    RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
        go build -trimpath -o build/image-composer \
            -ldflags "-s -w -extldflags '-static' \
                     -X 'github.com/open-edge-platform/image-composer/internal/config/version.Version=$version' \
                     -X 'github.com/open-edge-platform/image-composer/internal/config/version.Toolname=Image-Composer' \
                     -X 'github.com/open-edge-platform/image-composer/internal/config/version.Organization=Open Edge Platform' \
                     -X 'github.com/open-edge-platform/image-composer/internal/config/version.BuildDate=$(cat /tmp/build_date)' \
                     -X 'github.com/open-edge-platform/image-composer/internal/config/version.CommitSHA=$(cat /tmp/commit_sha)'" \
            ./cmd/image-composer
    SAVE ARTIFACT build/image-composer AS LOCAL ./build/image-composer

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
    
    # Install dependencies required by the coverage script
    RUN apt-get update && apt-get install -y bc bash
    
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

# Additional test targets for convenience
test-debug:
    FROM +golang-base
    ARG PRINT_TS=""
    ARG FAIL_ON_NO_TESTS=false
    
    # Install dependencies required by the coverage script
    RUN apt-get update && apt-get install -y bc bash
    
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
