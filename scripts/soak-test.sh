#!/usr/bin/env bash
# ==============================================================================
# 🐺 Watchdog Soak & Stability Test Runner (Linux / macOS)
# ==============================================================================
set -euo pipefail

DURATION=${1:-"60"} # Default: 60 seconds
PORT=${2:-"19100"}
HOST="127.0.0.1"
TOKEN="soak-test-token-$$"
LOG_FILE="/tmp/watchdog-soak.log"

echo "🐺 Starting Watchdog Soak Test for ${DURATION} seconds on ${HOST}:${PORT}..."

# Build binary
echo "🔨 Compiling Watchdog binary..."
go build -o /tmp/watchdog-soak-bin ./main.go

# Start background server
echo "🚀 Launching Watchdog server daemon..."
/tmp/watchdog-soak-bin server --port "${PORT}" --host "${HOST}" --token "${TOKEN}" > "${LOG_FILE}" 2>&1 &
SERVER_PID=$!

cleanup() {
    echo "🛑 Terminating server PID ${SERVER_PID}..."
    kill -TERM "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
    rm -f /tmp/watchdog-soak-bin
    echo "✅ Cleanup complete."
}
trap cleanup EXIT

# Wait for server ready
sleep 1
READY=0
for i in $(seq 1 10); do
    if curl -s -f "http://${HOST}:${PORT}/health" >/dev/null 2>&1; then
        READY=1
        break
    fi
    sleep 0.5
done

if [ "$READY" -ne 1 ]; then
    echo "❌ Failed to start Watchdog server within 5 seconds. Log dump:"
    cat "${LOG_FILE}"
    exit 1
fi

echo "✅ Server started successfully. Running stress and soak queries..."

START_TIME=$(date +%s)
END_TIME=$((START_TIME + DURATION))
REQUESTS_TOTAL=0
REQUESTS_FAILED=0

while [ $(date +%s) -lt "$END_TIME" ]; do
    # 1. Health check
    if curl -s -f "http://${HOST}:${PORT}/health" >/dev/null; then
        REQUESTS_TOTAL=$((REQUESTS_TOTAL + 1))
    else
        REQUESTS_FAILED=$((REQUESTS_FAILED + 1))
    fi

    # 2. Prometheus metrics
    if curl -s -f "http://${HOST}:${PORT}/metrics" >/dev/null; then
        REQUESTS_TOTAL=$((REQUESTS_TOTAL + 1))
    else
        REQUESTS_FAILED=$((REQUESTS_FAILED + 1))
    fi

    # 3. Authenticated Snapshot API
    if curl -s -f -H "Authorization: Bearer ${TOKEN}" "http://${HOST}:${PORT}/api/v1/snapshot" >/dev/null; then
        REQUESTS_TOTAL=$((REQUESTS_TOTAL + 1))
    else
        REQUESTS_FAILED=$((REQUESTS_FAILED + 1))
    fi

    # 4. Authenticated Diagnostics API
    if curl -s -f -H "Authorization: Bearer ${TOKEN}" "http://${HOST}:${PORT}/api/v1/diagnostics" >/dev/null; then
        REQUESTS_TOTAL=$((REQUESTS_TOTAL + 1))
    else
        REQUESTS_FAILED=$((REQUESTS_FAILED + 1))
    fi

    sleep 0.2
done

ELAPSED=$(( $(date +%s) - START_TIME ))

echo ""
echo "=============================================================================="
echo "📊 Soak Test Completed Successfully!"
echo "  Duration        : ${ELAPSED}s"
echo "  Total Requests  : ${REQUESTS_TOTAL}"
echo "  Failed Requests : ${REQUESTS_FAILED}"
echo "=============================================================================="

if [ "$REQUESTS_FAILED" -gt 0 ]; then
    echo "❌ Soak test detected request failures!"
    exit 1
fi

echo "🎉 Zero failures detected. System stability verified."
