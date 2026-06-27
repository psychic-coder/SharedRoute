# ShardRoute

### A Self-Healing Distributed Rate Limiter & Request Router

> **Reference Documentation:** [ShardRoute Design Document — v1.0](https://docs.google.com/document/d/1djncOWx3JMOk1C6ATY3TXyP_NMoeWIE_nPDH7pgWviQ/edit?usp=sharing)

---

## The Problem

Every system that exposes an API to multiple tenants — Stripe, Twilio, Razorpay, or any internal microservice mesh — needs to enforce limits on how many requests a given customer, API key, or IP can make per unit time. This sounds trivial until it has to work correctly across many servers handling tens of thousands of requests per second.

### Why This Is Hard, Not Trivial

The naive solution is: keep a counter in Redis, run `INCR` on every request, check it against a threshold, and `EXPIRE` it after the window ends. This works on a whiteboard and falls apart under real concurrency and scale for three concrete reasons:

**1. Race conditions on the read-then-write path**

If a server reads the counter, decides the request is allowed, and writes the increment as two separate Redis calls, two concurrent requests can both read the same pre-increment value and both get allowed — even when only one should be. At low traffic this is invisible. At thousands of requests per second from the same key, it silently lets through far more traffic than the configured limit.

**2. Network round-trip cost at the edge**

If every single incoming request has to make a network call to Redis before the application can decide whether to proceed, that round-trip (typically 1–5ms even within the same region) is now a tax paid on every request, multiplied across every router node. At high concurrency this becomes the dominant component of P99 latency — not the actual business logic.

**3. Single point of failure**

A single Redis instance handling rate-limit checks means that if that instance becomes slow or unreachable, every dependent service has to make a decision: block all traffic (fail closed, which takes down the product) or let all traffic through unchecked (fail open, which removes the protection at the exact moment load is highest). Most naive implementations don't even make this decision explicitly — they just crash or hang.

### Why Standard Algorithms Aren't Enough Either

| Algorithm | How it works | Where it breaks |
|-----------|-------------|-----------------|
| Fixed window | Counter resets every N seconds | Allows up to 2x the limit in a burst spanning the window boundary |
| Sliding window log | Stores a timestamp per request | Memory grows linearly with request volume per key; expensive at high cardinality |
| Pure token bucket | Tokens refill at a fixed rate | Refill bookkeeping per key still needs an atomic read-modify-write somewhere |

None of these problems are about which formula to use — they're about **where the state lives, how it's updated atomically, and what happens when that state becomes unreachable**. That's the actual systems problem ShardRoute is solving.

---

## What ShardRoute Does

ShardRoute is a distributed rate-limiting and request-routing layer designed to be **correct under concurrency**, **fast under load**, and to **degrade predictably** — not catastrophically — when its backing store is unavailable.

### Sliding Window + Token Bucket Hybrid Algorithm

Instead of choosing one algorithm and accepting its weakness, ShardRoute uses a **token bucket for the steady-state rate** (cheap, O(1) state per key) combined with a **short sliding window log** capped at a small fixed size (catches boundary bursts that a pure fixed window would miss, without the unbounded memory growth of a full sliding log). The combination is implemented as a single atomic operation rather than two separate checks.

### Atomicity via Lua Scripts

The check-and-decrement is implemented as a single Redis Lua script (`EVALSHA`), so the read, the decision, and the write happen as one atomic operation from Redis's perspective. This directly removes the race condition — there is no window between "read the counter" and "write the new value" for two concurrent requests to land in.

```lua
-- Shape of the atomic Lua script
local tokens = redis.call('HGET', key, 'tokens')
local refill  = compute_refill(now, last_refill_time)
if tokens + refill >= 1 then
  redis.call('HSET', key, 'tokens', tokens + refill - 1)
  return 1  -- allowed
else
  return 0  -- rejected
end
```

### Local Caching Layer: Keeping Redis Off the Hot Path

Each router node keeps an **in-memory approximate counter** per key, synced to Redis on a short interval (tens of milliseconds) rather than on every request. Most requests are checked against the local approximation; only the periodic sync touches the network.

> **Explicit Tradeoff:** A key could briefly exceed its limit by an amount bounded by the sync interval and request rate. That tradeoff is the right one for the API rate-limiting use case, where being off by a handful of requests for a few milliseconds is harmless, but adding 2–3ms to every single request fleet-wide is not.

### Explicit Failure Handling — Not Accidental

When Redis becomes unreachable, ShardRoute follows one of two explicitly configured modes:

- **Fail Open** — requests are allowed through using only the local approximation, prioritizing availability. Suitable for public free-tier APIs or internal service meshes where a Redis blip should not cascade into a full outage.
- **Fail Closed** — requests are rejected once local state can no longer be trusted, prioritizing protection. Suitable for premium APIs where unauthorized traffic during an outage is worse than downtime.

### Self-Healing

A background goroutine continuously PINGs Redis. Once it detects the backend has recovered, it automatically heals the node back to healthy state — no manual intervention required.

---

## Architecture

```text
[ Clients ] ---> [ Load Balancer / API Gateway ]
                            |
              +-------------+-------------+
              |                           |
   [ ShardRoute Node 1 ]       [ ShardRoute Node 2 ]
   (Local Memory Cache)        (Local Memory Cache)
              |                           |
              +-------------+-------------+
                            |
                     [ Redis Cluster ]
                 (Authoritative Token State)
```

| Component | Role |
|-----------|------|
| Router node | Holds the local approximate counter; serves the hot-path decision without a network call |
| Redis Cluster | Source of truth for the true distributed count; executes the atomic Lua check-and-decrement |
| Sync interval | Periodic reconciliation between local approximation and Redis state |
| Failure mode flag | Per-route config deciding fail-open vs. fail-closed when Redis is unreachable |
| k6 + Grafana | Load generation and observability for real, defensible latency and throughput numbers |

---

## Using ShardRoute in Your Project

ShardRoute is deployed as a **standalone sidecar** or **centralized microservice** that sits between your API Gateway and your application. Your application never needs to import a library — it simply makes a lightweight HTTP or gRPC call to ShardRoute before processing any request.

### Integration Pattern

```text
User Request
     │
     ▼
[ Your API Gateway ]
     │
     ├──► POST /v1/check ──► [ ShardRoute ]
     │         │                   │
     │    allowed=true         allowed=false
     │         │                   │
     ▼         ▼                   ▼
[ Your App ]  Process         Return 429 immediately
              Request         (never touch your backend)
```

**Step 1 — Make a rate-limit check before processing any request:**

```bash
curl -X POST http://your-shardroute:8080/v1/check \
  -H "Content-Type: application/json" \
  -d '{
    "key":        "user_id_or_api_key_or_ip",
    "cost":       1,
    "limit_name": "my_api_endpoint"
  }'
```

**Step 2 — Interpret the response:**

```json
// Allowed — proceed with request
{ "allowed": true, "tokens_remaining": 42 }

// Rejected — return 429 to the user immediately
{ "allowed": false, "tokens_remaining": 0, "retry_after_ms": 3200 }
```

**Step 3 — Wire it into your application middleware:**

```go
// Example: Go middleware
func RateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        resp, err := checkShardRoute(r.Context(), userID, cost)
        if err != nil || !resp.Allowed {
            w.Header().Set("Retry-After", strconv.FormatInt(resp.RetryAfterMs/1000, 10))
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

```python
# Example: FastAPI/Python middleware
async def rate_limit_middleware(request: Request, call_next):
    resp = requests.post("http://shardroute:8080/v1/check", json={
        "key": request.headers.get("X-API-Key"),
        "cost": 1,
        "limit_name": "global"
    }).json()

    if not resp["allowed"]:
        return JSONResponse({"error": "rate limit exceeded"}, status_code=429)

    return await call_next(request)
```

### Configuration

Copy `config.example.yaml` and adjust for your environment:

```yaml
server:
  http_port: 8080
  grpc_port: 9090

redis:
  mode: "single"             # "single" or "cluster"
  addrs: ["redis:6379"]
  dial_timeout_ms: 200
  command_timeout_ms: 100

limiter:
  default_capacity: 100          # max burst tokens
  default_refill_per_second: 10  # sustained rate
  window_size_ms: 1000
  max_requests_per_window: 100
  sync_interval_ms: 100          # local cache reconciliation interval

failure_mode: "fail_open"        # "fail_open" or "fail_closed"

health:
  unhealthy_threshold: 3         # failures before degraded mode
  check_interval_ms: 2000        # Redis health ping interval

observability:
  log_level: "info"
  metrics_enabled: true
```

---

## Running Locally

**Requirements:** Go 1.25+, Docker

```bash
# Clone the repo
git clone https://github.com/psychic-coder/shardroute
cd shardroute

# Start a 3-node cluster + Redis locally
cd deploy && docker compose up -d --build

# Test the rate limiter
curl -X POST http://localhost:8081/v1/check \
  -H "Content-Type: application/json" \
  -d '{"key": "user_123", "cost": 1, "limit_name": "api"}'
# → {"allowed":true,"tokens_remaining":99}

# Hammer the same key to trigger the limit
for i in $(seq 1 110); do
  curl -s -X POST http://localhost:8081/v1/check \
    -H "Content-Type: application/json" \
    -d '{"key": "user_123", "cost": 1, "limit_name": "api"}' | jq .allowed
done
```

---

## Production Deployment

### Kubernetes

```bash
docker build -t your-registry/shardroute:latest -f deploy/Dockerfile .
docker push your-registry/shardroute:latest
```

```yaml
# k8s/deployment.yaml (example)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shardroute
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: shardroute
          image: your-registry/shardroute:latest
          env:
            - name: SHARDBROUTE_REDIS_ADDRS
              value: "redis://your-production-redis:6379"
            - name: SHARDBROUTE_FAILURE_MODE
              value: "fail_open"
          ports:
            - containerPort: 8080   # HTTP
            - containerPort: 9090   # gRPC
```

### Fly.io

```bash
brew install flyctl
fly auth login
fly redis create          # answer "No" to ProdPack — rate-limit state is transient
fly secrets set SHARDBROUTE_REDIS_ADDRS="redis://your-upstash-url"
fly deploy
```

---

## Observability

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Liveness probe — always returns 200 |
| `GET /readyz` | Readiness probe — returns 503 if Redis degraded |
| `GET /metrics` | Prometheus metrics scrape endpoint |

Import `deploy/grafana/dashboard.json` into Grafana to get real-time panels for:
- **Request Rate** — Allowed vs. Rejected per second
- **Latency** — P50 and P99 across all nodes
- **Cache Hit Ratio** — Local hits vs. Redis round-trips
- **Node Health** — Degraded/Healthy status per instance

---

## Load Testing & Chaos Engineering

```bash
# Benchmark your deployment directly
make build
./bin/shardroute-bench -c 100 -d 30s -url http://localhost:8081/v1/check

# k6 baseline — 10,000 concurrent clients
k6 run loadtest/k6_baseline.js

# Chaos test — kills Redis mid-load-test to validate fail-open/closed behavior
bash loadtest/chaos_redis_kill.sh
```

---

## Local Development

```bash
make build    # build shardroute + shardroute-bench
make test     # run full test suite with race detector
make lint     # run golangci-lint
```
