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
        go build -trimpath -o build/os-image-composer \
            -ldflags "-s -w -extldflags '-static' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Version=$version' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Toolname=Image-Composer' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.Organization=Open Edge Platform' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.BuildDate=$(cat /tmp/build_date)' \
                     -X 'github.com/open-edge-platform/os-image-composer/internal/config/version.CommitSHA=$(cat /tmp/commit_sha)'" \
            ./cmd/image-composer
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