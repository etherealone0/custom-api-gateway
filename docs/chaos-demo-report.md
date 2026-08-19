# Chaos Demo Report

Generated 2026-08-20T02:13:41+05:30, covering the window 2026-08-20T02:11:22+05:30 to 2026-08-20T02:13:41+05:30 (2 minutes).

## State Transition Timeline

| t+ (s) | Component | Backend | From | To |
|--------|-----------|---------|------|----|
| 48 | circuit breaker | http://backend1:9000 | closed | open |
| 54 | health check | http://backend1:9000 | healthy | unhealthy |
| 92 | circuit breaker | http://backend1:9000 | open | closed |
| 92 | health check | http://backend1:9000 | unhealthy | healthy |

## Request Outcomes on /api/orders

| Status | Count |
|--------|-------|
| 200 | 9574 |
| 429 | 13997 |
| 503 | 39 |
