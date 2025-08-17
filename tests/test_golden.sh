#!/bin/bash

# Golden/Snapshot Tests for SpiderMonkey Dumper
# Tests .jsc → .dis output for deterministic results

set -e  # Exit on any error

# Set locale for deterministic output
export LC_ALL=C

# Colors for output  
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration - detect if running from main directory or tests directory
if [[ -d "tests/golden" ]]; then
  # Running from main directory
  GOLDEN_DIR="tests/golden"
  ACTUAL_DIR="tests/actual"
  SAMPLES_DIR="samples/testdata"
  DUMPER="./tests/bin/dumper"
else
  # Running from tests directory  
  GOLDEN_DIR="golden"
  ACTUAL_DIR="actual"
  SAMPLES_DIR="../samples/testdata"
  DUMPER="./tests/bin/dumper"
fi

# Test samples (without .jsc extension)
SAMPLES=(
    "simple"
    "functions"
    "minimal"
    "nested"
    "constants"
)

# Fixed flags for deterministic output
DUMPER_FLAGS="--no-color --no-lines --no-sugar"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  --generate    Generate/update snapshots"
    echo "  --diff        Show detailed differences (default verify mode)"
    echo "  --help        Show this help message"
    exit 1
}

ensure_dumper() {
    if [[ ! -f "$DUMPER" ]]; then
        echo -e "${RED}Error: $DUMPER not found. Run 'make test-golden' first.${NC}"
        exit 1
    fi
}

ensure_dirs() {
    mkdir -p "$GOLDEN_DIR" "$ACTUAL_DIR"
}

generate_golden() {
    echo -e "${YELLOW}=== Generating Golden Snapshots ===${NC}"
    
    for sample in "${SAMPLES[@]}"; do
        local input="$SAMPLES_DIR/$sample.jsc"
        local golden="$GOLDEN_DIR/$sample.dis.golden"
        
        if [[ ! -f "$input" ]]; then
            echo -e "${RED}Warning: Sample $input not found, skipping${NC}"
            continue
        fi
        
        echo -e "${BLUE}Generating: $sample.dis.golden${NC}"
        
        # Generate golden snapshot with deterministic flags
        if "$DUMPER" $DUMPER_FLAGS "$input" > "$golden" 2>/dev/null; then
            echo -e "${GREEN}✓ Generated $golden${NC}"
        else
            echo -e "${RED}✗ Failed to generate $golden${NC}"
            exit 1
        fi
    done
    
    echo -e "${GREEN}Golden snapshots generated successfully!${NC}"
}

run_golden_test() {
    local sample="$1"
    local input="$SAMPLES_DIR/$sample.jsc"
    local golden="$GOLDEN_DIR/$sample.dis.golden"
    local actual="$ACTUAL_DIR/$sample.dis"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    
    # Check if golden snapshot exists
    if [[ ! -f "$golden" ]]; then
        echo -e "${RED}✗ $sample: Golden snapshot missing ($golden)${NC}"
        echo -e "${YELLOW}  Run with --generate to create snapshots${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    
    # Check if input sample exists
    if [[ ! -f "$input" ]]; then
        echo -e "${RED}✗ $sample: Input sample missing ($input)${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    
    # Generate actual output
    if ! "$DUMPER" $DUMPER_FLAGS "$input" > "$actual" 2>/dev/null; then
        echo -e "${RED}✗ $sample: Failed to generate output${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    
    # Compare with golden snapshot
    if diff -q "$golden" "$actual" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ $sample${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        rm -f "$actual"  # Clean up on success
        return 0
    else
        echo -e "${RED}✗ $sample: Output differs from snapshot${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

show_diff() {
    local sample="$1"
    local golden="$GOLDEN_DIR/$sample.dis.golden"
    local actual="$ACTUAL_DIR/$sample.dis"
    
    if [[ -f "$golden" && -f "$actual" ]]; then
        echo -e "${YELLOW}=== Diff for $sample ===${NC}"
        diff -u "$golden" "$actual" || true
        echo
    fi
}

verify_golden() {
    echo -e "${YELLOW}=== Verifying Golden Snapshots ===${NC}"
    
    local failed_samples=()
    
    for sample in "${SAMPLES[@]}"; do
        if ! run_golden_test "$sample"; then
            failed_samples+=("$sample")
        fi
    done
    
    # Show diffs for failed tests if requested
    if [[ "$SHOW_DIFF" == "true" && ${#failed_samples[@]} -gt 0 ]]; then
        echo
        echo -e "${YELLOW}=== Detailed Differences ===${NC}"
        for sample in "${failed_samples[@]}"; do
            show_diff "$sample"
        done
    fi
    
    print_summary
}

print_summary() {
    echo
    echo -e "${YELLOW}=== Golden Test Summary ===${NC}"
    echo "Tests run: $TESTS_RUN"
    echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All golden tests passed! 🎉${NC}"
        exit 0
    else
        echo -e "${RED}Some golden tests failed! ❌${NC}"
        echo -e "${YELLOW}To update snapshots: make test-golden-update${NC}"
        echo -e "${YELLOW}To see differences: ./tests/test_golden.sh --diff${NC}"
        exit 1
    fi
}

# Parse command line options
GENERATE_MODE=false
SHOW_DIFF=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --generate)
            GENERATE_MODE=true
            shift
            ;;
        --diff)
            SHOW_DIFF=true
            shift
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Main execution
ensure_dumper
ensure_dirs

if [[ "$GENERATE_MODE" == "true" ]]; then
    generate_golden
else
    verify_golden
fi