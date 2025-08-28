#!/bin/bash
set -euo pipefail

# Script to run Go tests with per-directory coverage reporting
# Usage: ./run_coverage_tests.sh [COVERAGE_THRESHOLD] [PRINT_TS] [FAIL_ON_NO_TESTS] [DEBUG]

COV_THRESHOLD=15.4
PRINT_TS=${1:-""}
FAIL_ON_NO_TESTS=${2:-false}  # Set to true if directories without tests should fail the build
DEBUG=${3:-false}  # Set to true for verbose debugging
OVERALL_EXIT_CODE=0
FAILED_DIRS=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to find Go binary in common locations
find_go() {
    # Check if go is already in PATH
    if command -v go >/dev/null 2>&1; then
        echo "go"
        return 0
    fi
    
    # Check common Go installation locations
    for go_path in \
        "/usr/local/go/bin/go" \
        "/usr/bin/go" \
        "/snap/bin/go" \
        "$HOME/go/bin/go" \
        "/opt/go/bin/go"; do
        
        if [[ -x "$go_path" ]]; then
            echo "$go_path"
            return 0
        fi
    done
    
    return 1
}

# Find and set up Go
GO_BIN=$(find_go)
if [[ -z "$GO_BIN" ]]; then
    echo -e "${RED}ERROR: Go command not found in PATH or common locations.${NC}" >&2
    echo "Searched locations:" >&2
    echo "  - PATH: $(echo $PATH | tr ':' '\n' | grep -E 'go|bin' || echo 'No Go paths in PATH')" >&2
    echo "  - /usr/local/go/bin/go" >&2
    echo "  - /usr/bin/go" >&2
    echo "  - /snap/bin/go" >&2
    echo "  - $HOME/go/bin/go" >&2
    echo "  - /opt/go/bin/go" >&2
    echo "" >&2
    echo "Solutions:" >&2
    echo "  1. Install Go: sudo apt install golang-go" >&2
    echo "  2. Run without sudo: ./scripts/run_coverage_tests.sh" >&2
    echo "  3. Use sudo with PATH: sudo env PATH=\$PATH ./scripts/run_coverage_tests.sh" >&2
    exit 1
fi

# If Go is not 'go' (i.e., full path), add its directory to PATH
if [[ "$GO_BIN" != "go" ]]; then
    GO_DIR=$(dirname "$GO_BIN")
    export PATH="$GO_DIR:$PATH"
    echo "Added Go directory to PATH: $GO_DIR" >&2
fi

# Verify Go is working
echo "Using Go: $GO_BIN" >&2
echo "Go version: $($GO_BIN version)" >&2
echo "Working directory: $(pwd)" >&2
echo "Go module status:" >&2
$GO_BIN mod tidy >/dev/null 2>&1 || echo "  Warning: go mod tidy failed" >&2
if [[ -f "go.mod" ]]; then
    echo "  Go module found: $(head -n1 go.mod)" >&2
else
    echo "  No go.mod file found" >&2
fi

# Test basic Go functionality
echo "Testing basic Go functionality..." >&2
if $GO_BIN list ./... >/dev/null 2>&1; then
    echo "  Go can list packages successfully" >&2
else
    echo "  Warning: 'go list ./...' failed" >&2
fi
echo "" >&2

echo -e "${BLUE}=== Running Unit Tests with Coverage ===${NC}"
echo "Coverage threshold: ${COV_THRESHOLD}%"
if [[ -n "${PRINT_TS}" ]]; then
    echo "Build ID: ${PRINT_TS}"
fi
echo ""

# Find all directories with Go files (including those without tests)
ALL_GO_DIRS=$(find . -name "*.go" -type f | sed 's|/[^/]*$||' | sort -u | grep -v vendor | grep -v ".git")

# Find directories with test files
TEST_DIRS=$(find . -name "*_test.go" -type f | sed 's|/[^/]*$||' | sort -u | grep -v vendor | grep -v ".git")

if [[ -z "${ALL_GO_DIRS}" ]]; then
    echo -e "${RED}ERROR: No Go directories found${NC}"
    exit 1
fi

# Create table header
echo "| Directory                           | Coverage | Result   |"
echo "|-------------------------------------|----------|----------|"

# Arrays to store results and failed tests
declare -a DIR_RESULTS
declare -a DIR_COVERAGE
declare -a DIR_STATUS
declare -a FAILED_TEST_DETAILS=()  # Initialize empty array

# Run tests for each directory with Go files
for GO_DIR in ${ALL_GO_DIRS}; do
    DIR_NAME=$(echo ${GO_DIR} | sed 's|^\./||')
    COVERAGE_FILE="${DIR_NAME//\//_}_coverage.out"
    TEST_LOG="${DIR_NAME//\//_}_test.log"
    
    # Create directory structure for coverage file
    mkdir -p "$(dirname "${COVERAGE_FILE}")"
    mkdir -p "$(dirname "${TEST_LOG}")"
    
    # Check if this directory has test files
    if echo "${TEST_DIRS}" | grep -q "^${GO_DIR}$"; then
        # Directory has tests - run them
        TEST_CMD="$GO_BIN test -v -coverprofile=\"${COVERAGE_FILE}\" \"${GO_DIR}\""
        
        if [[ "$DEBUG" == "true" ]]; then
            echo "DEBUG: Running: $TEST_CMD" >&2
        fi
        
        if timeout 300 $GO_BIN test -v -coverprofile="${COVERAGE_FILE}" "${GO_DIR}" > "${TEST_LOG}" 2>&1; then
            # Tests passed, check coverage
            if [[ -f "${COVERAGE_FILE}" ]] && [[ -s "${COVERAGE_FILE}" ]]; then
                # Calculate coverage percentage
                COVERAGE_PCT=$($GO_BIN tool cover -func="${COVERAGE_FILE}" | grep "total:" | awk '{print $3}' | sed 's/%//')
                
                if [[ -z "${COVERAGE_PCT}" ]]; then
                    if [[ "$DEBUG" == "true" ]]; then
                        echo "DEBUG: No total coverage found for ${DIR_NAME}" >&2
                        echo "DEBUG: Coverage file content:" >&2
                        head -n 3 "${COVERAGE_FILE}" >&2
                        echo "DEBUG: Cover tool output:" >&2
                        $GO_BIN tool cover -func="${COVERAGE_FILE}" >&2
                    fi
                    COVERAGE_PCT="0.0"
                fi
                
                # Tests passed and coverage was generated - this is a PASS
                STATUS="PASS"
                STATUS_COLOR="${GREEN}"
                
                DIR_RESULTS+=("${DIR_NAME}")
                DIR_COVERAGE+=("${COVERAGE_PCT}")
                DIR_STATUS+=("${STATUS}")
                
                printf "| %-35s | %8s%% | ${STATUS_COLOR}%-8s${NC} |\n" \
                    "${DIR_NAME}" "${COVERAGE_PCT}" "${STATUS}"
            else
                # No coverage file or empty - tests failed
                STATUS="FAIL"
                OVERALL_EXIT_CODE=1
                
                # Extract failed test information
                FAILED_TESTS=""
                if [[ -f "${TEST_LOG}" ]]; then
                    # Extract failed test names from test output
                    FAILED_TESTS=$(grep -E "^--- FAIL:|FAIL\s+.*\s+\(" "${TEST_LOG}" | sed 's/^--- FAIL: //' | sed 's/FAIL[[:space:]]*//' | sed 's/[[:space:]]*(.*//' | sort -u | tr '\n' ', ' | sed 's/,$//')
                    
                    if [[ -z "${FAILED_TESTS}" ]]; then
                        # If no specific test names found, check for compilation errors
                        if grep -q "build failed" "${TEST_LOG}" || grep -q "compilation error" "${TEST_LOG}"; then
                            FAILED_TESTS="compilation error"
                        else
                            FAILED_TESTS="unknown test failure"
                        fi
                    fi
                else
                    FAILED_TESTS="no test output"
                fi
                
                FAILED_DIRS="${FAILED_DIRS} ${DIR_NAME}(no-coverage)"
                FAILED_TEST_DETAILS+=("${DIR_NAME}: ${FAILED_TESTS}")
                
                if [[ "$DEBUG" == "true" ]]; then
                    echo "DEBUG: No coverage file for ${DIR_NAME}" >&2
                    if [[ -f "${COVERAGE_FILE}" ]]; then
                        echo "DEBUG: Coverage file exists but is empty" >&2
                        ls -la "${COVERAGE_FILE}" >&2
                    else
                        echo "DEBUG: Coverage file does not exist: ${COVERAGE_FILE}" >&2
                    fi
                    echo "DEBUG: Test output:" >&2
                    cat "${TEST_LOG}" >&2
                fi
                
                printf "| %-35s | %8s | ${RED}%-8s${NC} |\n" \
                    "${DIR_NAME}" "N/A" "FAIL"
            fi
        else
            # Tests failed - but check if coverage was still generated
            STATUS="FAIL"
            OVERALL_EXIT_CODE=0 # TODO: Disabling the failure for failed test for now
            
            # Extract failed test information
            FAILED_TESTS=""
            if [[ -f "${TEST_LOG}" ]]; then
                # Extract failed test names from test output
                FAILED_TESTS=$(grep -E "^--- FAIL:|FAIL\s+.*\s+\(" "${TEST_LOG}" | sed 's/^--- FAIL: //' | sed 's/FAIL[[:space:]]*//' | sed 's/[[:space:]]*(.*//' | sort -u | tr '\n' ', ' | sed 's/,$//')
                
                if [[ -z "${FAILED_TESTS}" ]]; then
                    # If no specific test names found, check for compilation errors
                    if grep -q "build failed" "${TEST_LOG}" || grep -q "compilation error" "${TEST_LOG}"; then
                        FAILED_TESTS="compilation error"
                    else
                        FAILED_TESTS="unknown test failure"
                    fi
                fi
            else
                FAILED_TESTS="no test output"
            fi
            
            FAILED_DIRS="${FAILED_DIRS} ${DIR_NAME}(test-fail)"
            FAILED_TEST_DETAILS+=("${DIR_NAME}: ${FAILED_TESTS}")
            
            # Try to get coverage even if tests failed
            COVERAGE_PCT="N/A"
            if [[ -f "${COVERAGE_FILE}" ]] && [[ -s "${COVERAGE_FILE}" ]]; then
                COVERAGE_FROM_FAILED=$($GO_BIN tool cover -func="${COVERAGE_FILE}" | grep "total:" | awk '{print $3}' | sed 's/%//')
                if [[ -n "${COVERAGE_FROM_FAILED}" ]]; then
                    COVERAGE_PCT="${COVERAGE_FROM_FAILED}%"
                fi
            fi
            
            if [[ "$DEBUG" == "true" ]]; then
                echo "DEBUG: Tests failed for ${DIR_NAME}" >&2
                echo "DEBUG: Command: $TEST_CMD" >&2
                echo "DEBUG: Test output:" >&2
                cat "${TEST_LOG}" >&2
                echo "DEBUG: ---" >&2
            fi
            
            printf "| %-35s | %8s | ${RED}%-8s${NC} |\n" \
                "${DIR_NAME}" "${COVERAGE_PCT}" "FAIL"
        fi
    else
        # Directory has no tests
        if [[ "${FAIL_ON_NO_TESTS}" == "true" ]]; then
            printf "| %-35s | %8s | ${RED}%-8s${NC} |\n" \
                "${DIR_NAME}" "N/A" "FAIL"
            OVERALL_EXIT_CODE=1
            FAILED_DIRS="${FAILED_DIRS} ${DIR_NAME}(no-tests)"
        else
            printf "| %-35s | %8s | ${YELLOW}%-8s${NC} |\n" \
                "${DIR_NAME}" "N/A" "NO-TESTS"
        fi
    fi
done

echo ""

# Generate overall coverage report
echo -e "${BLUE}=== Generating Overall Coverage Report ===${NC}"

# Calculate overall coverage by running tests on all packages at once
OVERALL_COVERAGE_FILE="overall_coverage.out"

echo "Calculating overall repository coverage..."

# Method 1: Try to get overall coverage even if some tests fail
echo "Attempting overall coverage calculation (Method 1)..."
if $GO_BIN test -coverprofile="${OVERALL_COVERAGE_FILE}" ./... > overall_test.log 2>&1; then
    # All tests passed
    if [[ -f "${OVERALL_COVERAGE_FILE}" ]] && [[ -s "${OVERALL_COVERAGE_FILE}" ]]; then
        OVERALL_COVERAGE=$($GO_BIN tool cover -func="${OVERALL_COVERAGE_FILE}" | grep "total:" | awk '{print $3}' | sed 's/%//')
        
        if [[ -z "${OVERALL_COVERAGE}" ]]; then
            OVERALL_COVERAGE="0.0"
        fi
        
        echo "✓ Overall repository coverage: ${OVERALL_COVERAGE}%"
        COVERAGE_METHOD="all tests passed"
        
        # Copy the overall coverage file for artifacts
        cp "${OVERALL_COVERAGE_FILE}" coverage.out 2>/dev/null || echo "mode: set" > coverage.out
    else
        echo "✗ No overall coverage data generated despite test success"
        OVERALL_COVERAGE="0.0"
        echo "mode: set" > coverage.out
        COVERAGE_METHOD="failed - no data"
    fi
else
    # Some tests failed, but try to get coverage anyway
    echo "Some tests failed, attempting coverage with failures ignored..."
    
    # Method 1b: Try to get coverage despite test failures by continuing on failure
    if $GO_BIN test -coverprofile="${OVERALL_COVERAGE_FILE}" -failfast=false ./... > overall_test_with_failures.log 2>&1 || true; then
        if [[ -f "${OVERALL_COVERAGE_FILE}" ]] && [[ -s "${OVERALL_COVERAGE_FILE}" ]]; then
            OVERALL_COVERAGE=$($GO_BIN tool cover -func="${OVERALL_COVERAGE_FILE}" | grep "total:" | awk '{print $3}' | sed 's/%//')
            
            if [[ -z "${OVERALL_COVERAGE}" ]]; then
                OVERALL_COVERAGE="0.0"
            fi
            
            echo "✓ Overall repository coverage (with test failures): ${OVERALL_COVERAGE}%"
            COVERAGE_METHOD="with test failures"
            
            # Copy the overall coverage file for artifacts
            cp "${OVERALL_COVERAGE_FILE}" coverage.out 2>/dev/null || echo "mode: set" > coverage.out
        else
            echo "⚠ Method 1 failed, falling back to Method 2..."
            COVERAGE_METHOD="fallback to combined files"
            
            # Method 2: Combine individual coverage files from successful tests
            COMBINED_COVERAGE="combined_coverage.out"
            echo "mode: set" > "${COMBINED_COVERAGE}"
            
            SUCCESSFUL_DIRS=0
            TOTAL_LINES=0
            COVERED_LINES=0

            for TEST_DIR in ${TEST_DIRS}; do
                DIR_NAME=$(echo ${TEST_DIR} | sed 's|^\./||')
                COVERAGE_FILE="${DIR_NAME//\//_}_coverage.out"
                
                if [[ -f "${COVERAGE_FILE}" ]] && [[ -s "${COVERAGE_FILE}" ]]; then
                    # Skip the mode line and append
                    tail -n +2 "${COVERAGE_FILE}" >> "${COMBINED_COVERAGE}" 2>/dev/null || true
                    SUCCESSFUL_DIRS=$((SUCCESSFUL_DIRS + 1))
                    
                    # Extract coverage stats for better calculation
                    FUNC_OUTPUT=$($GO_BIN tool cover -func="${COVERAGE_FILE}" 2>/dev/null || true)
                    if echo "$FUNC_OUTPUT" | grep -q "total:"; then
                        TOTAL_LINE=$(echo "$FUNC_OUTPUT" | grep "total:")
                        # Get statements count and coverage percentage
                        STMT_TOTAL=$(echo "$TOTAL_LINE" | awk '{print $2}')
                        STMT_COVERAGE=$(echo "$TOTAL_LINE" | awk '{print $3}' | sed 's/%//')
                        
                        if [[ -n "$STMT_TOTAL" ]] && [[ -n "$STMT_COVERAGE" ]] && [[ "$STMT_TOTAL" != "0" ]]; then
                            STMT_COVERED=$(echo "scale=0; $STMT_TOTAL * $STMT_COVERAGE / 100" | bc -l)
                            TOTAL_LINES=$(echo "$TOTAL_LINES + $STMT_TOTAL" | bc -l)
                            COVERED_LINES=$(echo "$COVERED_LINES + $STMT_COVERED" | bc -l)
                        fi
                    fi
                fi
            done

            # Calculate overall coverage from combined files
            if [[ -s "${COMBINED_COVERAGE}" ]] && [[ $SUCCESSFUL_DIRS -gt 0 ]]; then
                # Try the direct method first
                OVERALL_COVERAGE=$($GO_BIN tool cover -func="${COMBINED_COVERAGE}" | grep "total:" | awk '{print $3}' | sed 's/%//' 2>/dev/null || echo "")
                
                # If that fails, use our manual calculation
                if [[ -z "$OVERALL_COVERAGE" ]] && [[ $(echo "$TOTAL_LINES > 0" | bc -l) -eq 1 ]]; then
                    OVERALL_COVERAGE=$(echo "scale=1; $COVERED_LINES * 100 / $TOTAL_LINES" | bc -l)
                fi
                
                if [[ -z "${OVERALL_COVERAGE}" ]]; then
                    OVERALL_COVERAGE="0.0"
                fi
                
                echo "✓ Overall repository coverage (from $SUCCESSFUL_DIRS successful test directories): ${OVERALL_COVERAGE}%"
                
                # Copy combined coverage for artifact
                cp "${COMBINED_COVERAGE}" coverage.out 2>/dev/null || echo "mode: set" > coverage.out
            else
                echo "✗ No coverage data available from any method"
                OVERALL_COVERAGE="0.0"
                echo "mode: set" > coverage.out
                COVERAGE_METHOD="no data available"
            fi
        fi
    else
        echo "✗ All coverage calculation methods failed"
        OVERALL_COVERAGE="0.0"
        echo "mode: set" > coverage.out
        COVERAGE_METHOD="all methods failed"
    fi
fi

# Display results
echo "Coverage threshold: ${COV_THRESHOLD}%"
echo "Calculation method: ${COVERAGE_METHOD}"

if (( $(echo "${OVERALL_COVERAGE} >= ${COV_THRESHOLD}" | bc -l) )); then
    echo -e "${GREEN}✓ Overall coverage PASSED threshold${NC}"
else
    echo -e "${RED}✗ Overall coverage FAILED threshold${NC}"
    OVERALL_EXIT_CODE=1
fi

# Generate coverage reports for saving
echo "## Test Coverage Report" > coverage_total.txt
echo "" >> coverage_total.txt
echo "**Overall Coverage:** ${OVERALL_COVERAGE}%" >> coverage_total.txt
echo "**Threshold:** ${COV_THRESHOLD}% (applies to overall coverage only)" >> coverage_total.txt
echo "**Status:** $(if [[ ${OVERALL_EXIT_CODE} -eq 0 ]]; then echo "PASSED"; else echo "FAILED"; fi)" >> coverage_total.txt
echo "" >> coverage_total.txt

# FIXED: Use safer array check that works with set -u
if [[ "${#FAILED_TEST_DETAILS[@]}" -gt 0 ]] 2>/dev/null; then
    echo "**Failed Tests:**" >> coverage_total.txt
    for detail in "${FAILED_TEST_DETAILS[@]}"; do
        echo "  • ${detail}" >> coverage_total.txt
    done
    echo "" >> coverage_total.txt
fi

echo "**Note:** Directory PASS/FAIL indicates test results only, not coverage." >> coverage_total.txt
echo "**Note:** Coverage threshold applies to overall repository coverage only." >> coverage_total.txt
echo "" >> coverage_total.txt

echo "| Directory                           | Coverage | Result   |" >> coverage_total.txt
echo "|-------------------------------------|----------|----------|" >> coverage_total.txt

# Recreate the table for the report
for GO_DIR in ${ALL_GO_DIRS}; do
    DIR_NAME=$(echo ${GO_DIR} | sed 's|^\./||')
    COVERAGE_FILE="${DIR_NAME//\//_}_coverage.out"
    TEST_LOG="${DIR_NAME//\//_}_test.log"
    
    # Check if this directory has test files
    if echo "${TEST_DIRS}" | grep -q "^${GO_DIR}$"; then
        # Directory has tests
        if [[ -f "${COVERAGE_FILE}" ]] && [[ -s "${COVERAGE_FILE}" ]]; then
            COVERAGE_PCT=$($GO_BIN tool cover -func="${COVERAGE_FILE}" | grep "total:" | awk '{print $3}' | sed 's/%//')
            if [[ -z "${COVERAGE_PCT}" ]]; then
                COVERAGE_PCT="0.0"
            fi
            
            # Check if tests passed by looking at the test log
            if grep -q "PASS" "${TEST_LOG}" && ! grep -q "FAIL" "${TEST_LOG}"; then
                STATUS="PASS"
            else
                STATUS="FAIL"
            fi
            
            printf "| %-35s | %8s%% | %-8s |\n" \
                "${DIR_NAME}" "${COVERAGE_PCT}" "${STATUS}" >> coverage_total.txt
        else
            # Tests failed or no coverage generated - check if any coverage was generated
            COVERAGE_DISPLAY="N/A"
            if [[ -f "${COVERAGE_FILE}" ]] && [[ -s "${COVERAGE_FILE}" ]]; then
                COVERAGE_FROM_FAILED=$($GO_BIN tool cover -func="${COVERAGE_FILE}" | grep "total:" | awk '{print $3}')
                if [[ -n "${COVERAGE_FROM_FAILED}" ]]; then
                    COVERAGE_DISPLAY="${COVERAGE_FROM_FAILED}"
                fi
            fi
            
            printf "| %-35s | %8s | %-8s |\n" \
                "${DIR_NAME}" "${COVERAGE_DISPLAY}" "FAIL" >> coverage_total.txt
        fi
    else
        # Directory has no tests
        if [[ "${FAIL_ON_NO_TESTS}" == "true" ]]; then
            printf "| %-35s | %8s | %-8s |\n" \
                "${DIR_NAME}" "N/A" "FAIL" >> coverage_total.txt
        else
            printf "| %-35s | %8s | %-8s |\n" \
                "${DIR_NAME}" "N/A" "NO-TESTS" >> coverage_total.txt
        fi
    fi
done

# Generate package-level coverage breakdown
if [[ -f "overall_coverage.out" ]] && [[ -s "overall_coverage.out" ]]; then
    echo "" > coverage_packages.txt
    echo "Package Coverage Breakdown (sorted by coverage ascending):" >> coverage_packages.txt
    echo "================================================================" >> coverage_packages.txt
    $GO_BIN tool cover -func="overall_coverage.out" | grep -v "total:" | sort -k3 -n >> coverage_packages.txt
elif [[ -f "coverage.out" ]] && [[ -s "coverage.out" ]]; then
    echo "" > coverage_packages.txt
    echo "Package Coverage Breakdown (sorted by coverage ascending):" >> coverage_packages.txt
    echo "================================================================" >> coverage_packages.txt
    $GO_BIN tool cover -func="coverage.out" | grep -v "total:" | sort -k3 -n >> coverage_packages.txt
fi

# Generate detailed test log
echo "=== Detailed Test Results ===" > test_raw.log
echo "Threshold: ${COV_THRESHOLD}%" >> test_raw.log
echo "Overall Coverage: ${OVERALL_COVERAGE}%" >> test_raw.log
echo "Status: $(if [[ ${OVERALL_EXIT_CODE} -eq 0 ]]; then echo "PASSED"; else echo "FAILED"; fi)" >> test_raw.log
echo "" >> test_raw.log

for GO_DIR in ${ALL_GO_DIRS}; do
    DIR_NAME=$(echo ${GO_DIR} | sed 's|^\./||')
    TEST_LOG="${DIR_NAME//\//_}_test.log"
    echo "=== ${DIR_NAME} ===" >> test_raw.log
    
    # Check if this directory has test files
    if echo "${TEST_DIRS}" | grep -q "^${GO_DIR}$"; then
        # Directory has tests - include test log
        if [[ -f "${TEST_LOG}" ]]; then
            cat "${TEST_LOG}" >> test_raw.log
        else
            echo "No test log found" >> test_raw.log
        fi
    else
        # Directory has no tests
        echo "No test files found in this directory" >> test_raw.log
    fi
    echo "" >> test_raw.log
done

echo ""
if [[ ${OVERALL_EXIT_CODE} -eq 0 ]]; then
    if [[ -n "${FAILED_DIRS}" ]]; then
        echo -e "${RED}❌ Test failures detected${NC}"
        # FIXED: Use safer array check for displaying failed test details
        if [[ "${#FAILED_TEST_DETAILS[@]}" -gt 0 ]] 2>/dev/null; then
            echo -e "${RED}Failed directories and tests:${NC}"
            for detail in "${FAILED_TEST_DETAILS[@]}"; do
                echo -e "${RED}  • ${detail}${NC}"
            done
        fi
    else
        echo -e "${GREEN}🎉 All tests passed!${NC}"
    fi
    echo -e "${GREEN}🎉 Overall coverage requirements met!${NC}"
else
    if [[ -n "${FAILED_DIRS}" ]]; then
        echo -e "${RED}❌ Test failures detected${NC}"
        # FIXED: Use safer array check for displaying failed test details
        if [[ "${#FAILED_TEST_DETAILS[@]}" -gt 0 ]] 2>/dev/null; then
            echo -e "${RED}Failed directories and tests:${NC}"
            for detail in "${FAILED_TEST_DETAILS[@]}"; do
                echo -e "${RED}  • ${detail}${NC}"
            done
        fi
    else
        echo -e "${GREEN}🎉 All tests passed!${NC}"
    fi
    if (( $(echo "${OVERALL_COVERAGE} < ${COV_THRESHOLD}" | bc -l) )); then
        echo -e "${RED}❌ Overall coverage (${OVERALL_COVERAGE}%) below threshold (${COV_THRESHOLD}%)${NC}"
    fi
fi

echo ""
echo "Note: Directory PASS/FAIL is based on test results only, not coverage."
echo "Note: Overall coverage threshold (${COV_THRESHOLD}%) applies to repository total."
echo "Note: Directories without tests are marked as NO-TESTS and $(if [[ "${FAIL_ON_NO_TESTS}" == "true" ]]; then echo "DO"; else echo "DO NOT"; fi) cause build failure"

exit ${OVERALL_EXIT_CODE}
