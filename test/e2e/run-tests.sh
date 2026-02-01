#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASSED=0
FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Functional test: verify I/O throttling at a specific rate
# Args: $1=test_name $2=limit_bytes_per_sec $3=write_size_mb $4=expected_min_secs $5=expected_max_secs
run_io_throttle_test() {
    local test_name="$1"
    local limit_bps="$2"
    local write_mb="$3"
    local min_secs="$4"
    local max_secs="$5"
    
    # Convert limit to human readable
    local limit_human
    if (( limit_bps >= 1048576 )); then
        limit_human="$((limit_bps / 1048576))MB/s"
    else
        limit_human="$((limit_bps / 1024))KB/s"
    fi
    
    echo ""
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}Test: ${test_name}${NC}"
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "  Write limit: ${YELLOW}${limit_human}${NC}"
    echo -e "  Write size:  ${YELLOW}${write_mb}MB${NC}"
    echo -e "  Expected:    ${YELLOW}${min_secs}-${max_secs} seconds${NC}"
    echo ""
    
    # Create the test manifest dynamically
    LIMIT_BPS=${limit_bps} envsubst < $SCRIPT_DIR/assets/io-throttle-test-template.yaml > /tmp/io-throttle-test.yaml

    echo -e "${CYAN}► Creating pod with io.max wbps=${limit_bps}...${NC}"
    
    # Clean up any previous test resources first (wait for actual deletion)
    kubectl delete pod io-throttle-test-pod --ignore-not-found --wait=true 2>/dev/null || true
    kubectl delete resourceclaimtemplate io-throttle-claim --ignore-not-found --wait=true 2>/dev/null || true
    sleep 1
    
    if ! kubectl create -f /tmp/io-throttle-test.yaml 2>&1 | grep -v "^Warning:"; then
        echo -e "${RED}FAIL${NC}: Failed to create manifest"
        ((FAILED++))
        return 1
    fi
    
    local pod="io-throttle-test-pod"
    
    if ! kubectl wait --for=condition=Ready "pod/${pod}" --timeout=120s 2>/dev/null; then
        echo -e "${RED}FAIL${NC}: Pod did not become ready"
        kubectl describe pod "${pod}" 2>/dev/null | grep -A 10 "Events:" || true
        kubectl delete -f /tmp/io-throttle-test.yaml --ignore-not-found 2>/dev/null || true
        ((FAILED++))
        return 1
    fi
    
    # Show cgroup setting
    local io_max
    io_max=$(kubectl exec "${pod}" -- cat /sys/fs/cgroup/io.max 2>/dev/null)
    echo -e "${CYAN}► Cgroup io.max:${NC} ${GREEN}${io_max}${NC}"
    
    # Run dd test
    echo -e "${CYAN}► Writing ${write_mb}MB with direct I/O...${NC}"
    
    local dd_output
    dd_output=$(kubectl exec "${pod}" -- sh -c "
        time dd if=/dev/zero of=/tmp/testfile bs=1M count=${write_mb} oflag=direct 2>&1
    " 2>&1)
    
    # Extract elapsed time
    local elapsed_time
    elapsed_time=$(echo "$dd_output" | grep -oE '[0-9]+\.[0-9]+ seconds' | grep -oE '[0-9.]+' | head -1 || echo "")
    
    # Extract throughput
    local throughput
    throughput=$(echo "$dd_output" | grep -oE '[0-9.]+[KMG]?B/s' | tail -1 || echo "unknown")
    
    echo ""
    echo -e "  ${BOLD}Result:${NC} ${write_mb}MB in ${GREEN}${elapsed_time}s${NC} @ ${GREEN}${throughput}${NC}"
    
    # Check result
    local passed=false
    if [[ -n "$elapsed_time" ]]; then
        if (( $(echo "$elapsed_time >= $min_secs && $elapsed_time <= $max_secs" | bc -l 2>/dev/null || echo "0") )); then
            passed=true
        fi
    fi
    
    if $passed; then
        echo -e "  ${GREEN}${BOLD}✓ PASS${NC}"
        ((PASSED++))
    else
        echo -e "  ${RED}${BOLD}✗ FAIL${NC} - Expected ${min_secs}-${max_secs}s, got ${elapsed_time}s"
        ((FAILED++))
    fi
    
    # Clean up (wait for deletion to complete to avoid conflicts with next test)
    kubectl delete -f /tmp/io-throttle-test.yaml --ignore-not-found --wait=true 2>/dev/null || true
    rm -f /tmp/io-throttle-test.yaml
    sleep 1
}

echo ""
echo -e "${BOLD}╔═════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║  io.max Functional Tests                        ║${NC}"
echo -e "${BOLD}║                                                 ║${NC}"
echo -e "${BOLD}║  Verifying block I/O bandwidth throttling       ║${NC}"
echo -e "${BOLD}╚═════════════════════════════════════════════════╝${NC}"

# All tests write 8MB, but with different limits -> different expected times
# This demonstrates the throttling is actually controlling the rate

# Test 1: 1MB/s limit, write 8MB -> expect ~8 seconds
run_io_throttle_test "io.max @ 1MB/s (8MB write)" \
    1048576 \
    8 \
    7.0 \
    10.0

# Test 2: 2MB/s limit, write 8MB -> expect ~4 seconds  
run_io_throttle_test "io.max @ 2MB/s (8MB write)" \
    2097152 \
    8 \
    3.0 \
    6.0

# Test 3: 4MB/s limit, write 8MB -> expect ~2 seconds
run_io_throttle_test "io.max @ 4MB/s (8MB write)" \
    4194304 \
    8 \
    1.5 \
    3.5

# Test 4: 8MB/s limit, write 8MB -> expect ~1 second
run_io_throttle_test "io.max @ 8MB/s (8MB write)" \
    8388608 \
    8 \
    0.5 \
    2.0

# Summary
echo ""
echo -e "${BOLD}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}Results: ${GREEN}${PASSED} passed${NC}, ${RED}${FAILED} failed${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════════════════${NC}"

if [[ ${FAILED} -gt 0 ]]; then
    exit 1
fi
