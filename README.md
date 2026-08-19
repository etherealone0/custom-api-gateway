# Custom API Gateway & L7 Load Balancer

## Project Overview

A high-performance and lightweight API Gateway written in Go. The gateway handles routing, load balancing, health checking, and critical edge features like rate limiting and JWT authentication.

## Architecture

The gateway processes incoming requests through a strict middleware pipeline before routing them to healthy backend servers.

```mermaid
graph TD
    Client([Client]) --> Gateway
    
    subgraph Gateway [API Gateway Core]
        direction TB
        L[Logger & Metrics] --> C[CORS]
        C --> J[JWT Auth]
        J --> RL[Rate Limiter]
        RL --> RT[Router]
        
        RT --> |Longest-Prefix Match| P[Reverse Proxy]
        P --> |Retry Loop| CB[Circuit Breaker]
        CB --> B[Load Balancer]
    end
    
    B -->|NextServer| Backend1[(Backend 1)]
    B -->|NextServer| Backend2[(Backend 2)]
    B -->|NextServer| Backend3[(Backend 3)]
    
    HC[Background Health Checker] -.->|Poll interval| Backend1
    HC -.->|Poll interval| Backend2
    HC -.->|Poll interval| Backend3
    
    W[Config Watcher] -.->|fsnotify| Config[(config.yaml)]
    W -.->|Hot Reload| RT
```

## Quickstart

### Prerequisites
- [Go 1.26+](https://golang.org/doc/install)
- [Docker](https://docs.docker.com/get-docker/)

### Running Locally with Docker

The easiest way to test the gateway is using the included `docker-compose.yml`, which spins up the gateway and three mock backends.

1. **Start the cluster in the background:**
   ```bash
   make docker-up
   ```
2. **Test the gateway:**
   The gateway runs on `localhost:8080`. (Note: You will need a valid JWT token to pass the authentication middleware, or you can disable JWT in `config.yaml`).
3. **Tear down the cluster:**
   ```bash
   make docker-down
   ```

### Running Natively
```bash
make build
JWT_SECRET=mysecret ./bin/gateway
```

## Configuration Reference

The gateway is configured via a `config.yaml` file. Changes to this file are automatically detected via `fsnotify` and reloaded with zero downtime.

```yaml
server:
  port: 8080                 
  read_timeout: 5s           
  write_timeout: 10s         
  tls:
    enabled: false           # Set to true and provide cert/key files for HTTPS

routes:
  - path: /api/orders        # Matched via longest-prefix
    strip_prefix: false      # Removes /api/orders before forwarding
    strategy: round_robin    # Options: round_robin, weighted_round_robin, least_conn
    backends:
      - url: http://backend1:9000
        weight: 1            # Used by weighted_round_robin
      - url: http://backend2:9000
        weight: 2

health_check:
  interval: 10s              # Ping frequency
  timeout: 2s                # Ping timeout
  path: /healthz             # Health endpoint

rate_limit:
  requests_per_second: 100   # Max sustained RPS per IP
  burst: 200                 # Max instantaneous burst allowed

auth:
  jwt_secret: ${JWT_SECRET}  

cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]

circuit_breaker:
  failure_threshold: 3       # Consecutive failures before circuit opens (fast-fails)
  success_threshold: 2       # Consecutive successes to close circuit again
  timeout: 5s                # Time to wait in Open state before testing again

retry:
  max_attempts: 3            # retry attempts at 5xx errors
  base_delay: 100ms          # Starting backoff delay
  max_delay: 2s              # Maximum backoff delay
```

## Benchmark Results

The gateway was benchmarked using [k6](https://k6.io/) simulating 50 concurrent virtual users over 25 seconds against a 2-backend route.

| Metric | Result |
|--------|--------|
| **Throughput (RPS)** | `3,481 req/sec` |
| **Average Latency** | `1.05 ms` |
| **p(99) Latency** | `4.81 ms` |
| **Total Requests** | `87,030` |

*Note: The rate limiter accurately dropped 97% of this traffic with HTTP 429, allowing exactly 100 RPS through as configured. The gateway remained fully responsive.*

## Chaos / Failure-Injection Demo

The benchmark above proves the gateway is fast on the happy path. This demo proves the resilience machinery — retries, the per-backend circuit breaker, and the health checker — actually works under a real failure, not just in unit tests.

`scripts/chaos-demo.sh` runs a sustained [k6](https://k6.io/) load against `/api/orders` (backed by `backend1` + `backend2`) and, partway through, kills `backend1` with `docker stop` and later brings it back with `docker start`. While that happens:

- **Retries** mask individual failed attempts by re-picking a backend via the load balancer.
- The **circuit breaker** for `backend1` trips open after `circuit_breaker.failure_threshold` consecutive failures, fast-fails for `circuit_breaker.timeout`, then probes half-open and closes again once `success_threshold` successes land.
- The **health checker** independently notices `backend1` is down (and later back up) on its own `health_check.interval` poll and pulls it out of / back into rotation.

A `prometheus` service (added to `docker-compose.yml`, scraping the gateway's `/metrics` every 2s) records `gateway_circuit_breaker_state`, `gateway_backend_health`, and `gateway_requests_total` for the whole run. After the load test finishes, a small Go tool (`tools/chaos-report`) queries Prometheus and renders the state transitions and request outcomes into `docs/chaos-demo-report.md`.

Run it with:

```bash
make chaos-demo
```

Then inspect `docs/chaos-demo-report.md` for the generated timeline, or open http://localhost:9090 to graph `gateway_circuit_breaker_state{backend="http://backend1:9000"}` and `gateway_backend_health{backend="http://backend1:9000"}` directly. Tear the stack down afterward with `make docker-down`.

## Extending the Gateway

### Adding a New Balancer Strategy

1. **Implement the `Balancer` interface:**
   Create a new file in `internal/balancer/` and implement the `NextServer` method.
   ```go
   type Balancer interface {
       NextServer() (*Backend, error)
   }
   ```

2. **Register it in the factory:**
   Open `internal/balancer/balancer.go` and add your algorithm to the switch statement in `NewBalancer()`:
   ```go
   func NewBalancer(strategy string, pool *ServerPool) (Balancer, error) {
       switch strategy {
       case "round_robin":
           return NewRoundRobin(pool), nil
       case "ip_hash":
           return NewIPHash(pool), nil // <-- Your custom strategy
       // ...
       }
   }
   ```

3. **Update config:**
   Set `strategy: ip_hash` in your `config.yaml`.