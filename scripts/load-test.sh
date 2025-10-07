#!/bin/bash

# Portask Load Testing Script
# Comprehensive performance testing

set -e

echo "🚀 Portask Load Testing Suite"
echo "==============================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_PID_FILE="/tmp/portask_loadtest.pid"
LOG_FILE="/tmp/portask_loadtest.log"
RESULTS_DIR="./load-test-results"

# Create results directory
mkdir -p $RESULTS_DIR

# Function to cleanup
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up...${NC}"
    if [ -f "$SERVER_PID_FILE" ]; then
        PID=$(cat $SERVER_PID_FILE)
        kill $PID 2>/dev/null || true
        rm -f $SERVER_PID_FILE
    fi
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# Trap cleanup on exit
trap cleanup EXIT

# Start server
start_server() {
    echo -e "${BLUE}🚀 Starting Portask server...${NC}"
    ./build/portask > $LOG_FILE 2>&1 &
    echo $! > $SERVER_PID_FILE
    sleep 3
    
    # Check if server is running
    if kill -0 $(cat $SERVER_PID_FILE) 2>/dev/null; then
        echo -e "${GREEN}✅ Server started (PID: $(cat $SERVER_PID_FILE))${NC}"
    else
        echo -e "${RED}❌ Failed to start server${NC}"
        exit 1
    fi
}

# Wait for server
wait_for_server() {
    echo -e "${BLUE}⏳ Waiting for server to be ready...${NC}"
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Server is ready${NC}"
            return 0
        fi
        sleep 1
    done
    echo -e "${RED}❌ Server not ready after 30 seconds${NC}"
    exit 1
}

# Test 1: HTTP API Benchmark
test_http_api() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}📊 Test 1: HTTP API Performance${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    # Health endpoint
    echo -e "\n${YELLOW}Testing /health endpoint...${NC}"
    ab -n 10000 -c 100 -q http://localhost:8080/health > $RESULTS_DIR/health_benchmark.txt 2>&1
    grep "Requests per second" $RESULTS_DIR/health_benchmark.txt
    grep "Time per request" $RESULTS_DIR/health_benchmark.txt | head -1
    
    # Status endpoint
    echo -e "\n${YELLOW}Testing /status endpoint...${NC}"
    ab -n 10000 -c 100 -q http://localhost:8080/status > $RESULTS_DIR/status_benchmark.txt 2>&1
    grep "Requests per second" $RESULTS_DIR/status_benchmark.txt
}

# Test 2: Message Publishing Load
test_message_publishing() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}📤 Test 2: Message Publishing Load${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    # Create test payload
    cat > /tmp/publish_payload.json <<EOF
{
  "topic": "load-test",
  "payload": "$(head -c 1024 /dev/urandom | base64 | tr -d '\n')",
  "priority": "normal"
}
EOF
    
    echo -e "\n${YELLOW}Publishing 1000 messages (100 concurrent)...${NC}"
    ab -n 1000 -c 100 -p /tmp/publish_payload.json -T application/json -q \
        http://localhost:8080/api/v1/messages/publish > $RESULTS_DIR/publish_benchmark.txt 2>&1 || true
    
    if [ -f $RESULTS_DIR/publish_benchmark.txt ]; then
        echo -e "${GREEN}Results saved to: $RESULTS_DIR/publish_benchmark.txt${NC}"
    fi
}

# Test 3: Ultra Benchmark
test_ultra_benchmark() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}🔥 Test 3: Ultra Performance Benchmark${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    echo -e "\n${YELLOW}Running ultra-benchmark (15 seconds)...${NC}"
    timeout 20s go run cmd/ultra-benchmark/main.go 2>&1 | tee $RESULTS_DIR/ultra_benchmark.txt || true
    
    # Extract key metrics
    if [ -f $RESULTS_DIR/ultra_benchmark.txt ]; then
        echo -e "\n${GREEN}📊 Ultra Benchmark Summary:${NC}"
        grep -E "(Messages Published|Messages Processed|Publish Rate|Process Rate|Throughput)" \
            $RESULTS_DIR/ultra_benchmark.txt | tail -6
    fi
}

# Test 4: Concurrent Connections
test_concurrent_connections() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}🔗 Test 4: Concurrent Connections${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    echo -e "\n${YELLOW}Testing with 500 concurrent connections...${NC}"
    ab -n 5000 -c 500 -q http://localhost:8080/health > $RESULTS_DIR/concurrent_500.txt 2>&1
    grep "Requests per second" $RESULTS_DIR/concurrent_500.txt
    grep "Failed requests" $RESULTS_DIR/concurrent_500.txt
}

# Test 5: Go Benchmarks
test_go_benchmarks() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}⚡ Test 5: Go Micro Benchmarks${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    echo -e "\n${YELLOW}Running queue benchmarks...${NC}"
    go test -bench=BenchmarkQueue -benchmem -benchtime=3s ./pkg/queue/ 2>&1 | \
        tee $RESULTS_DIR/queue_benchmark.txt | grep -E "^Benchmark|ns/op|allocs/op"
    
    echo -e "\n${YELLOW}Running compression benchmarks...${NC}"
    go test -bench=. -benchmem -benchtime=2s ./pkg/compression/ 2>&1 | \
        tee $RESULTS_DIR/compression_benchmark.txt | grep -E "^Benchmark|ns/op|MB/s"
}

# Generate Report
generate_report() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}📄 Generating Load Test Report${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    
    REPORT_FILE="$RESULTS_DIR/LOAD_TEST_REPORT.md"
    
    cat > $REPORT_FILE <<EOF
# 🚀 Portask Load Test Report

**Test Date:** $(date)
**Platform:** $(uname -s) $(uname -m)
**Go Version:** $(go version)

---

## Test Summary

### 1. HTTP API Performance

#### Health Endpoint
\`\`\`
$(grep -A 5 "Requests per second" $RESULTS_DIR/health_benchmark.txt 2>/dev/null || echo "No data")
\`\`\`

#### Status Endpoint
\`\`\`
$(grep -A 5 "Requests per second" $RESULTS_DIR/status_benchmark.txt 2>/dev/null || echo "No data")
\`\`\`

### 2. Ultra Benchmark Results

\`\`\`
$(grep -E "(Messages Published|Messages Processed|Publish Rate|Process Rate|Throughput|ACHIEVEMENT)" \
    $RESULTS_DIR/ultra_benchmark.txt 2>/dev/null | tail -10 || echo "No data")
\`\`\`

### 3. Concurrent Connections Test

\`\`\`
$(grep -E "(Requests per second|Failed requests|Time per request)" \
    $RESULTS_DIR/concurrent_500.txt 2>/dev/null || echo "No data")
\`\`\`

### 4. Queue Micro Benchmarks

\`\`\`
$(cat $RESULTS_DIR/queue_benchmark.txt 2>/dev/null || echo "No data")
\`\`\`

### 5. Compression Benchmarks

\`\`\`
$(cat $RESULTS_DIR/compression_benchmark.txt 2>/dev/null || echo "No data")
\`\`\`

---

## Server Logs (Last 50 lines)

\`\`\`
$(tail -50 $LOG_FILE)
\`\`\`

---

**Report Generated:** $(date)
EOF
    
    echo -e "${GREEN}✅ Report saved to: $REPORT_FILE${NC}"
}

# Main execution
main() {
    echo -e "${BLUE}Starting load test suite...${NC}"
    echo ""
    
    # Check if Apache Bench is available
    if ! command -v ab &> /dev/null; then
        echo -e "${YELLOW}⚠️  Apache Bench (ab) not found. Some tests will be skipped.${NC}"
        echo -e "${YELLOW}   Install with: brew install apache2-utils (macOS) or apt install apache2-utils (Linux)${NC}"
    fi
    
    # Build if needed
    if [ ! -f "./build/portask" ]; then
        echo -e "${YELLOW}Building Portask...${NC}"
        make build
    fi
    
    # Start server and run tests
    start_server
    wait_for_server
    
    if command -v ab &> /dev/null; then
        test_http_api
        test_message_publishing
        test_concurrent_connections
    fi
    
    test_ultra_benchmark
    test_go_benchmarks
    
    # Generate report
    generate_report
    
    echo ""
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo -e "${GREEN}✅ Load Testing Complete!${NC}"
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo ""
    echo -e "📊 Results directory: ${BLUE}$RESULTS_DIR${NC}"
    echo -e "📄 Full report: ${BLUE}$REPORT_FILE${NC}"
    echo -e "📋 Server logs: ${BLUE}$LOG_FILE${NC}"
    echo ""
}

# Run main function
main

