#!/bin/bash

# Fuzz test runner
set -e

echo "Running fuzz tests"
echo "=================================="

# Test duration (can be overridden with environment variable)
FUZZ_TIME=${FUZZ_TIME:-30s}

echo "Fuzz test duration: ${FUZZ_TIME}"
echo ""

# Dynamically find the ICT directory (Go module root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ICT_DIR="$(cd "${SCRIPT_DIR}" && git rev-parse --show-toplevel 2>/dev/null || echo "${SCRIPT_DIR}")"

echo "Running from: ${ICT_DIR}"
cd "${ICT_DIR}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to run a single fuzz test
run_fuzz_test() {
    local test_name=$1
    echo -e "${YELLOW}Testing ${test_name} function...${NC}"
    
    if go test -run='^$' -fuzz="${test_name}" -fuzztime="${FUZZ_TIME}" ./cmd/image-composer; then
        echo -e "${GREEN}✅ ${test_name} PASSED!${NC}"
        return 0
    else
        echo -e "${RED}❌ ${test_name} FAILED!${NC}"
        return 1
    fi
}

# Run both fuzz tests
TOTAL_TESTS=0
FAILED_TESTS=0

# Test 1: createRootCommand function
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_fuzz_test "FuzzCreateRootCommand"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 2: Command line argument handling
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_fuzz_test "FuzzCommandLineArgs"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""
echo "=================================="
echo "FUZZ TEST SUMMARY"
echo "=================================="
echo "Total tests: $TOTAL_TESTS"
echo "Passed: $((TOTAL_TESTS - FAILED_TESTS))"
echo "Failed: $FAILED_TESTS"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}All fuzz tests passed!${NC}"
    echo ""
    echo "The functions handle various input combinations without crashing."
    echo "This helps ensure the CLI is robust against different flag values and arguments."
    exit 0
else
    echo -e "${RED}$FAILED_TESTS fuzz test(s) failed.${NC}"
    echo ""
    echo "Check the output above for details about what input caused the failure."
    echo "Failing inputs are saved in testdata/fuzz/FuzzFunctionName/"
    exit 1
fi
