#!/bin/bash

# Comprehensive fuzz test runner for all fuzz tests in the image-composer project
set -e

echo "Running All Fuzz Tests"
echo "======================"

# Test duration (can be overridden with environment variable)
FUZZ_TIME=${FUZZ_TIME:-30s}

echo "Fuzz test duration: ${FUZZ_TIME}"
echo ""

# Dynamically find the project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}" && git rev-parse --show-toplevel 2>/dev/null || echo "${SCRIPT_DIR}")"

echo "Running from: ${PROJECT_ROOT}"
cd "${PROJECT_ROOT}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to run a single fuzz test (for packages)
run_package_fuzz_test() {
    local package=$1
    local test_name=$2
    echo -e "${YELLOW}Testing ${test_name} in ${package}...${NC}"
    
    if go test -run='^$' -fuzz="${test_name}" -fuzztime="${FUZZ_TIME}" "./${package}"; then
        echo -e "${GREEN}✅ ${test_name} PASSED!${NC}"
        return 0
    else
        echo -e "${RED}❌ ${test_name} FAILED!${NC}"
        return 1
    fi
}

# Function to run a single fuzz test (for main.go)
run_main_fuzz_test() {
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

# Track test results
TOTAL_TESTS=0
FAILED_TESTS=0

echo -e "${BLUE}=== MAIN.GO FUZZ TESTS ===${NC}"

# Test 1: createRootCommand function
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_main_fuzz_test "FuzzCreateRootCommand"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 2: Command line argument handling
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_main_fuzz_test "FuzzCommandLineArgs"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""
echo -e "${BLUE}=== CONFIG PACKAGE FUZZ TESTS ===${NC}"

# Test 3: Config template loading
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config" "FuzzLoadTemplate"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 4: YAML parsing
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config" "FuzzParseYAMLTemplate"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""
echo -e "${BLUE}=== VALIDATION PACKAGE FUZZ TESTS ===${NC}"

# Test 5: Schema validation
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/validate" "FuzzValidateAgainstSchema"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 6: Image template validation
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/validate" "FuzzValidateImageTemplateJSON"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 7: User template validation
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/validate" "FuzzValidateUserTemplateJSON"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 8: Config validation
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/validate" "FuzzValidateConfigJSON"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""
echo -e "${BLUE}=== MANIFEST PACKAGE FUZZ TESTS ===${NC}"

# Test 9: Manifest writing
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/manifest" "FuzzWriteManifestToFile"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 10: SPDX writing
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/manifest" "FuzzWriteSPDXToFile"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""

# Test 11: Document namespace generation
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if ! run_package_fuzz_test "internal/config/manifest" "FuzzGenerateDocumentNamespace"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

echo ""
echo "============================================"
echo "COMPREHENSIVE FUZZ TEST SUMMARY"
echo "============================================"
echo "Total tests: $TOTAL_TESTS"
echo "Passed: $((TOTAL_TESTS - FAILED_TESTS))"
echo "Failed: $FAILED_TESTS"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}All fuzz tests passed!${NC}"
    echo ""
    echo "The image-composer application handles various input combinations without crashing."
    echo "This helps ensure robust operation against malformed inputs and edge cases."
    echo ""
    echo "Key areas tested:"
    echo "- Main command creation and argument parsing"
    echo "- Configuration loading and parsing (YAML/JSON)"
    echo "- Schema validation and template processing"
    echo "- Manifest and SPDX document generation"
    exit 0
else
    echo -e "${RED}$FAILED_TESTS fuzz test(s) failed.${NC}"
    echo ""
    echo "Check the output above for details about what input caused the failure."
    echo "Failing inputs are saved in testdata/fuzz/FuzzFunctionName/"
    echo ""
    echo "To debug a specific failure:"
    echo "1. Check the testdata/fuzz/ directory for saved failing inputs"
    echo "2. Run the specific test with -v flag for verbose output"
    echo "3. Use go test -run=FuzzFunctionName to reproduce the failure"
    echo ""
    echo "Examples:"
    echo "  go test -run=FuzzCreateRootCommand -v ./cmd/image-composer"
    echo "  go test -run=FuzzValidateAgainstSchema -v ./internal/config/validate"
    exit 1
fi
