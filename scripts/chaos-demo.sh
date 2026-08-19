#!/usr/bin/env bash
# Chaos / failure-injection demo.
#
# Runs a sustained load test against the gateway while killing backend1
# mid-test and bringing it back up, then queries Prometheus for the
# resulting circuit-breaker / health-check state timeline. Demonstrates
# that failures on one backend get retried onto a healthy one, the circuit
# breaker opens and later recovers, and the health checker independently
# pulls the dead backend out of rotation and restores it.
#
# Requires: docker compose, k6, go (all already used elsewhere in this repo).
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

KILL_AT=10       # seconds into the hold stage to kill backend1
RESTART_AT=50    # seconds into the hold stage to restart backend1
REPORT_OUT="docs/chaos-demo-report.md"

echo "==> Starting docker-compose stack (gateway, 3 backends, prometheus)..."
docker-compose up --build -d

echo "==> Waiting for gateway to become healthy..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/metrics >/dev/null 2>&1; then
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "gateway did not become healthy in time" >&2
        exit 1
    fi
    sleep 1
done

echo "==> Waiting for Prometheus to become ready..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:9090/-/ready >/dev/null 2>&1; then
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "prometheus did not become ready in time" >&2
        exit 1
    fi
    sleep 1
done

# Give the health checker a couple of cycles to mark everything alive.
sleep 3

mkdir -p docs
DEMO_START=$(date +%s)

echo "==> Launching k6 load test in the background (tests/chaos_load_test.js)..."
k6 run tests/chaos_load_test.js &
K6_PID=$!

echo "==> Waiting ${KILL_AT}s into the hold stage before killing backend1..."
sleep $((10 + KILL_AT))   # +10s for the ramp-up stage
echo "==> Killing backend1 to simulate a mid-traffic crash..."
docker stop backend1

echo "==> Waiting $((RESTART_AT - KILL_AT))s before restarting backend1..."
sleep $((RESTART_AT - KILL_AT))
echo "==> Restarting backend1..."
docker start backend1

echo "==> Waiting for k6 to finish..."
wait "$K6_PID"

DEMO_END=$(date +%s)
ELAPSED_MIN=$(awk -v s="$DEMO_START" -v e="$DEMO_END" 'BEGIN { printf "%.2f", (e - s) / 60 + 0.5 }')

echo "==> Querying Prometheus and rendering the report..."
go run ./tools/chaos-report -minutes "$ELAPSED_MIN" -out "$REPORT_OUT"

echo
echo "Done. Report: $REPORT_OUT"
echo "Prometheus UI (for manual graph inspection): http://localhost:9090"
echo "Bring the stack down with: docker-compose down -v"
