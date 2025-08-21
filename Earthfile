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
    ARG COV_THRESHOLD=0
    ARG PRINT_TS=0
    # Need bash to reliably capture pipeline status with PIPESTATUS
    RUN apk add --no-cache bash gawk

    # 1) Run tests, capture exit code, and build coverage artifacts
    #    - tee => console + test_raw.log
    #    - bash -o pipefail => pipeline returns failing command's status
    RUN --mount=type=cache,target=/root/.cache/go-build bash -o pipefail -c '
      set -u

      # Discover all packages (exclude cmd/)
      ALL_PKGS="$(go list ./... | grep -Ev "/cmd($|/)")"
      printf "%s\n" "$ALL_PKGS" > pkgs_all.txt
      COVERPKG="$(printf "%s\n" "$ALL_PKGS" | paste -sd, -)"

      # Stream test output to console AND log; record go test exit code
      go test -v -timeout=3m \
         -covermode=atomic \
         -coverpkg="$COVERPKG" \
         -coverprofile=coverage.out \
         -skip='"'"'^(TestCopyFile(/(Basic_Copy|Copy_with_preserve_flag|Create_missing_destination_directory))?|TestCopyDir(/(Basic_Directory_Copy|Copy_with_preserve_flag|Create_missing_destination_directory))?|TestCopyFilePermissions|TestCopyFileConcurrent|TestExecCmd$|TestExecCmdWithStream|TestExecCmdWithInput|TestGetFullCmdStr)$'"'"' \
         $ALL_PKGS 2>&1 | tee test_raw.log
      TEST_RC=${PIPESTATUS[0]}

      # Ensure coverage file exists so downstream steps always work
      [ -s coverage.out ] || echo "mode: atomic" > coverage.out

      # Build per-package coverage table (lowest -> highest)
      /usr/bin/gawk '"'"'BEGIN{OFS="";FS="[ \t]+"}
        FNR==NR {want[$0]=1; next}                 # pkgs_all.txt
        $0 ~ /^mode:/ {next}
        $1 !~ /:[0-9]+\.[0-9]+,/ {next}
        {
          split($1,a,":"); path=a[1]
          n=split(path,parts,"/"); if(n<2) next
          pkg=parts[1]; for(i=2;i<=n-1;i++) pkg=pkg"/"parts[i]
          stmts=$2+0; hits=$3+0
          total[pkg]+=stmts; if(hits>0) covered[pkg]+=stmts
        }
        END{
          for(p in want){
            t=total[p]+0; c=covered[p]+0; pct=(t>0)?(c/t*100.0):0.0
            printf "%.6f\t%s\n", pct, p
          }
        }'"'"' pkgs_all.txt coverage.out \
      | sort -t "$(printf "\t")" -k1,1n > pkgs_pct_sorted.tsv

      # Compute total; safe fallback if cover tool errors
      if ! go tool cover -func=coverage.out > coverage_total.txt 2>/dev/null; then
        echo "total: (statements) 0.0%" > coverage_total.txt
      fi
      TOTAL="$(/usr/bin/gawk '"'"'/^total:/ {print substr($3,1,length($3)-1)}'"'"' coverage_total.txt)"
      [ -n "$TOTAL" ] || TOTAL=0

      # Render a clean text table file
      PKG_W="$(/usr/bin/gawk '"'"'BEGIN{w=7} { if (length($0)>w) w=length($0) } END{print w}'"'"' pkgs_all.txt)"
      /usr/bin/gawk -F "\t" -v W="$PKG_W" -v TOT="$TOTAL" '"'"'
        BEGIN{
          covW=7
          print ""
          printf "%-*s  %*s\n", W, "Package", covW, "Coverage"
          for(i=0;i<W;i++) printf "-"; printf "  "; for(i=0;i<covW;i++) printf "-"; print ""
        }
        { printf "%-*s  %6.2f%%\n", W, $2, $1+0 }
        END{
          for(i=0;i<W;i++) printf "-"; printf "  "; for(i=0;i<7;i++) printf "-"; print ""
          printf "%-*s  %6.2f%%\n", W, "total", TOT+0
          print ""
        }'"'"' pkgs_pct_sorted.tsv > coverage_packages.txt

      # Threshold check -> set COV_RC=1 if total < threshold
      COV_RC=0
      /usr/bin/gawk -v total="$TOTAL" -v thr="$COV_THRESHOLD" '"'"'BEGIN{ exit( (total+0 < thr+0) ? 1 : 0 ) }'"'"' || COV_RC=1

      echo "$TEST_RC" > .test_rc
      echo "$COV_RC"  > .cov_rc
    '

    # 2) Save artifacts (always)
    SAVE ARTIFACT coverage.out AS LOCAL ./coverage.out
    SAVE ARTIFACT coverage_total.txt AS LOCAL ./coverage_total.txt
    SAVE ARTIFACT coverage_packages.txt AS LOCAL ./coverage_packages.txt
    SAVE ARTIFACT test_raw.log AS LOCAL ./test_raw.log

    # 3) Always print table + verdict, then fail if tests failed or under threshold
    RUN cat coverage_packages.txt && \
        /usr/bin/gawk "/^total:/ {print; found=1} END{ if(!found) print \"total: (statements) N/A\" }" coverage_total.txt && \
        TOTAL2="$(/usr/bin/gawk "/^total:/ {print substr(\$3,1,length(\$3)-1)}" coverage_total.txt)" && \
        /usr/bin/gawk -v total="$TOTAL2" -v thr="$COV_THRESHOLD" \
            "BEGIN{ if (total+0 < thr+0) { printf(\"❌ Coverage %.2f%% is below threshold %.2f%%\\n\", total, thr) } else { printf(\"✅ Coverage %.2f%% meets threshold %.2f%%\\n\", total, thr) } }" && \
        TEST_RC="$(cat .test_rc 2>/dev/null || echo 0)" && \
        COV_RC="$(cat .cov_rc 2>/dev/null || echo 0)" && \
        if [ "$TEST_RC" -ne 0 ]; then echo "❌ Unit tests failed (exit $TEST_RC)"; exit 1; fi && \
        if [ "$COV_RC" -ne 0 ]; then exit 1; fi
