#!/bin/bash

# SpiderMonkey Dumper Ollama Test Suite
# Tests that require network connectivity and Ollama service

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

# Check if Ollama is available
check_ollama() {
    if ! curl -s --connect-timeout 2 http://localhost:11434/api/tags >/dev/null 2>&1; then
        echo -e "${YELLOW}Warning: Ollama service not available at localhost:11434${NC}"
        echo -e "${YELLOW}Skipping Ollama network tests. Start Ollama to run these tests.${NC}"
        exit 0
    fi
}

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
        if timeout 30 eval "$cmd" >/dev/null 2>&1; then
            echo -e "${GREEN}✓ PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}✗ FAIL (expected success but got failure)${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        if timeout 30 eval "$cmd" >/dev/null 2>&1; then
            echo -e "${RED}✗ FAIL (expected failure but got success)${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        else
            echo -e "${GREEN}✓ PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        fi
    fi
    echo
}

print_section() {
    echo -e "${YELLOW}=== $1 ===${NC}"
}

print_summary() {
    echo -e "${YELLOW}=== OLLAMA TEST SUMMARY ===${NC}"
    echo "Tests run: $TESTS_RUN"
    echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All Ollama tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "${RED}Some Ollama tests failed! ❌${NC}"
        exit 1
    fi
}

# Start testing
echo -e "${YELLOW}SpiderMonkey Dumper Ollama Test Suite${NC}"
echo "Dumper: $DUMPER"
echo "Test sample: $TEST_SAMPLE"
echo

# Check Ollama availability
check_ollama

# === OLLAMA CONNECTIVITY TESTS ===
print_section "Ollama Connectivity"

run_test "Custom Ollama host (valid)" "pass" \
    "$DUMPER --ollama-host 'http://localhost:11434' --no-inner '$TEST_SAMPLE'"

run_test "Custom Ollama model" "pass" \
    "$DUMPER --ollama-model 'llama2' --no-inner '$TEST_SAMPLE'"

run_test "Custom timeout with network" "pass" \
    "$DUMPER --ollama-timeout 10 --no-inner '$TEST_SAMPLE'"

run_test "Custom retries with network" "pass" \
    "$DUMPER --ollama-retries 1 --no-inner '$TEST_SAMPLE'"

# === OLLAMA ERROR HANDLING ===
print_section "Ollama Error Handling"

run_test "Invalid Ollama host" "fail" \
    "$DUMPER --ollama-host 'http://invalid:9999' --ollama-timeout 5 --ollama-retries 1 --no-inner '$TEST_SAMPLE'"

run_test "Invalid Ollama model" "fail" \
    "$DUMPER --ollama-model 'nonexistent-model-12345' --ollama-timeout 5 --ollama-retries 1 --no-inner '$TEST_SAMPLE'"

# === OLLAMA INTEGRATION TESTS ===
print_section "Ollama Integration"

# Test actual decompilation if user has a model available
run_test "Decompilation with short timeout" "pass" \
    "$DUMPER --decompile --ollama-timeout 30 --ollama-retries 1 --no-inner '$TEST_SAMPLE'"

print_summary