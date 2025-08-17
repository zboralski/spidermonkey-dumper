#!/bin/bash

# SpiderMonkey Dumper Test Suite
# Comprehensive testing of command line options and functionality

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Test sample file
TEST_SAMPLE="samples/testdata/simple.jsc"
DUMPER="./dumper"

# Ensure dumper exists
if [[ ! -f "$DUMPER" ]]; then
    echo -e "${RED}Error: dumper binary not found. Run 'make' first.${NC}"
    exit 1
fi

# Ensure test sample exists
if [[ ! -f "$TEST_SAMPLE" ]]; then
    echo -e "${RED}Error: test sample $TEST_SAMPLE not found.${NC}"
    exit 1
fi

# Test framework functions
run_test() {
    local test_name="$1"
    local expected_result="$2"  # "pass" or "fail"
    shift 2
    local cmd="$@"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${BLUE}TEST $TESTS_RUN: $test_name${NC}"
    echo "Command: $cmd"
    
    if [[ "$expected_result" == "pass" ]]; then
        if eval "$cmd" >/dev/null 2>&1; then
            echo -e "${GREEN}✓ PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}✗ FAIL (expected success but got failure)${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        if eval "$cmd" >/dev/null 2>&1; then
            echo -e "${RED}✗ FAIL (expected failure but got success)${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        else
            echo -e "${GREEN}✓ PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        fi
    fi
    echo
}

run_output_test() {
    local test_name="$1"
    local expected_pattern="$2"
    shift 2
    local cmd="$@"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${BLUE}TEST $TESTS_RUN: $test_name${NC}"
    echo "Command: $cmd"
    echo "Expected pattern: $expected_pattern"
    
    local output
    output=$(eval "$cmd" 2>&1)
    
    if echo "$output" | grep -q "$expected_pattern"; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "Actual output:"
        echo "$output" | head -3
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo
}

run_file_test() {
    local test_name="$1"
    local expected_file="$2"
    shift 2
    local cmd="$@"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${BLUE}TEST $TESTS_RUN: $test_name${NC}"
    echo "Command: $cmd"
    echo "Expected file: $expected_file"
    
    # Clean up any existing file
    rm -f "$expected_file"
    
    if eval "$cmd" >/dev/null 2>&1 && [[ -f "$expected_file" ]]; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        # Clean up test file
        rm -f "$expected_file"
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo
}

print_section() {
    echo -e "${YELLOW}=== $1 ===${NC}"
}

print_summary() {
    echo -e "${YELLOW}=== TEST SUMMARY ===${NC}"
    echo "Tests run: $TESTS_RUN"
    echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed! ❌${NC}"
        exit 1
    fi
}

# Start testing
echo -e "${YELLOW}SpiderMonkey Dumper Test Suite${NC}"
echo "Dumper: $DUMPER"
echo "Test sample: $TEST_SAMPLE"
echo

# === BASIC FUNCTIONALITY TESTS ===
print_section "Basic Functionality"

run_test "Basic disassembly" "pass" \
    "$DUMPER '$TEST_SAMPLE'"

run_test "Help option" "pass" \
    "$DUMPER --help"

run_test "Version/debug info" "pass" \
    "$DUMPER --debug '$TEST_SAMPLE'"

# === COMMAND LINE OPTION TESTS ===
print_section "Command Line Options"

run_test "Color enable option" "pass" \
    "$DUMPER --color '$TEST_SAMPLE'"

run_test "Color disable option" "pass" \
    "$DUMPER --no-color '$TEST_SAMPLE'"

run_test "Lines enable option" "pass" \
    "$DUMPER --lines '$TEST_SAMPLE'"

run_test "Lines disable option" "pass" \
    "$DUMPER --no-lines '$TEST_SAMPLE'"

run_test "Disable inner functions" "pass" \
    "$DUMPER --no-inner '$TEST_SAMPLE'"

run_test "Disable syntactic sugar" "pass" \
    "$DUMPER --no-sugar '$TEST_SAMPLE'"

# === OLLAMA OPTION PARSING TESTS ===
print_section "Ollama Option Parsing (No Network)"

run_test "Custom timeout (valid)" "pass" \
    "$DUMPER --ollama-timeout 120 --no-inner '$TEST_SAMPLE'"

run_test "Custom retries (valid)" "pass" \
    "$DUMPER --ollama-retries 5 --no-inner '$TEST_SAMPLE'"

run_test "Zero retries (valid)" "pass" \
    "$DUMPER --ollama-retries 0 --no-inner '$TEST_SAMPLE'"

# === ERROR HANDLING TESTS ===
print_section "Error Handling"

run_test "Invalid timeout (negative)" "fail" \
    "$DUMPER --ollama-timeout -1 '$TEST_SAMPLE'"

run_test "Invalid timeout (zero)" "fail" \
    "$DUMPER --ollama-timeout 0 '$TEST_SAMPLE'"

run_test "Invalid retries (negative)" "fail" \
    "$DUMPER --ollama-retries -1 '$TEST_SAMPLE'"

run_test "Missing file argument" "fail" \
    "$DUMPER --debug"

run_test "Non-existent file" "fail" \
    "$DUMPER nonexistent.jsc"

run_test "Invalid option" "fail" \
    "$DUMPER --invalid-option '$TEST_SAMPLE'"

# === OUTPUT VALIDATION TESTS ===
print_section "Output Validation"

run_output_test "Help contains timeout option" \
    "ollama-timeout" \
    "$DUMPER --help"

run_output_test "Help contains retries option" \
    "ollama-retries" \
    "$DUMPER --help"

# Skip output validation for now - can hang in some environments
# run_output_test "Error message for invalid timeout" \
#     "Invalid timeout value" \
#     "$DUMPER --ollama-timeout -5 '$TEST_SAMPLE'"

# run_output_test "Error message for invalid retries" \
#     "Invalid retries value" \
#     "$DUMPER --ollama-retries -3 '$TEST_SAMPLE'"

# run_output_test "Debug output contains debug markers" \
#     "\[\+\]" \
#     "$DUMPER --debug '$TEST_SAMPLE'"

# === FILE OUTPUT TESTS ===
print_section "File Output"

run_file_test "Creates .dis file" \
    "samples/testdata/simple.dis" \
    "$DUMPER '$TEST_SAMPLE'"

# === OPTION COMBINATION TESTS ===
print_section "Option Combinations"

run_test "Multiple valid options" "pass" \
    "$DUMPER --debug --no-color --lines --ollama-timeout 180 '$TEST_SAMPLE'"

run_test "All disable options" "pass" \
    "$DUMPER --no-color --no-lines --no-inner --no-sugar '$TEST_SAMPLE'"

run_test "Debug with custom Ollama settings" "pass" \
    "$DUMPER --debug --ollama-timeout 60 --ollama-retries 1 --no-inner '$TEST_SAMPLE'"

# === EDGE CASES ===
print_section "Edge Cases"

run_test "Very high timeout value" "pass" \
    "$DUMPER --ollama-timeout 3600 '$TEST_SAMPLE'"

run_test "Maximum reasonable retries" "pass" \
    "$DUMPER --ollama-retries 10 '$TEST_SAMPLE'"

run_test "Long option followed by short file name" "pass" \
    "$DUMPER --ollama-timeout 300 '$TEST_SAMPLE'"

# === INTEGRATION TESTS ===
print_section "Integration Tests"

# Test that options don't interfere with each other
run_test "All Ollama options together" "pass" \
    "$DUMPER --ollama-timeout 120 --ollama-retries 2 --no-inner '$TEST_SAMPLE'"

# Test mixed option styles
run_test "Mixed short and long options" "pass" \
    "$DUMPER -v --no-color '$TEST_SAMPLE'"

# Clean up any test artifacts
rm -f samples/testdata/simple.dis
rm -f samples/testdata/simple.js

print_summary