# Load Test Benchmark Results

The API Gateway was put through a rigorous load testing scenario using [k6](https://k6.io/). The objective was to measure throughput (RPS), latency, and to verify that the token-bucket Rate Limiter drops traffic exactly as configured.

## Scenario
- **Duration**: 25 seconds (5s ramp-up to 50 VUs, 15s hold, 5s ramp-down)
- **Target Endpoint**: `/api/orders` (Load balanced via Weighted Round-Robin across 2 backend servers)
- **Configuration**:
  - `rate_limit.requests_per_second: 100`
  - `rate_limit.burst: 200`

## Results Summary

| Metric | Result |
|--------|--------|
| **Total Requests** | `87,030` |
| **Throughput (RPS)** | `3,481.01 req/s` |
| **Max Concurrent Users** | `50` |
| **Average Latency (All)** | `1.05 ms` |
| **p(99) Latency** | `4.81 ms` |
| **Successful Routing (200 OK)** | `2,516` (Matches the ~100 RPS limit * 25 seconds) |
| **Rate Limited (429 Too Many)** | `84,514` (97.10% of traffic correctly blocked) |

## Key Findings

1. **Blazing Fast**: The gateway handled **~3,481 requests per second** effortlessly on extremely constrained hardware (Docker containers capped at ~5MB RAM each).
2. **Rate Limiting Works**: The gateway correctly allowed exactly ~100 requests per second through to the backends (totaling 2,516 successful hits) and instantly blocked the remaining 84,514 requests with a `429 Too Many Requests` status.
3. **Ultra-Low Latency**: The p(99) latency was under **5 milliseconds**, meaning 99% of all requests were evaluated, authenticated, rate-limited, and responded to in under 5ms. The average response time was a blistering **1.05ms**. 

