VERSION 0.8

LOCALLY
ARG http_proxy=$(echo $http_proxy)
ARG https_proxy=$(echo $https_proxy)
ARG no_proxy=$(echo $no_proxy)
ARG HTTP_PROXY=$(echo $HTTP_PROXY)
ARG HTTPS_PROXY=$(echo $HTTPS_PROXY)
ARG NO_PROXY=$(echo $NO_PROXY)
ARG REGISTRY

FROM ${REGISTRY}golang:1.24.1-alpine3.21
ENV http_proxy=$http_proxy
ENV https_proxy=$https_proxy
ENV no_proxy=$no_proxy
ENV HTTP_PROXY=$HTTP_PROXY
ENV HTTPS_PROXY=$HTTPS_PROXY
ENV NO_PROXY=$NO_PROXY

golang-base:
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.7
    WORKDIR /work
    COPY go.mod .
    COPY go.sum .
    RUN go mod download # for caching
    COPY cmd/ ./cmd
    COPY internal/ ./internal
    COPY image-templates/ ./image-templates
    COPY schema/ ./schema
    COPY testdata/ ./testdata

all:
    BUILD +build

fetch-golang:
    RUN apk add curl && curl -fsSLO https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
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
                     -X main.Version=$version \
                     -X main.BuildDate=$(cat /tmp/build_date) \
                     -X main.CommitSHA=$(cat /tmp/commit_sha)" \
            ./cmd/image-composer
    SAVE ARTIFACT build/image-composer AS LOCAL ./build/image-composer

test:
    FROM +golang-base
    RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
        go test -v ./...

lint:
    FROM +golang-base
    WORKDIR /work
    COPY . /work
    RUN --mount=type=cache,target=/root/.cache \
        golangci-lint run ./...
