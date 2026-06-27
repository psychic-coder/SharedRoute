# ShardRoute

A distributed rate-limiting and request-routing service written in Go. ShardRoute uses a hybrid token-bucket and sliding-window algorithm, utilizing atomic Redis Lua scripting and a highly performant local in-memory caching layer. 

**Reference Documentation:** [ShardRoute Design & Build Specifications](https://docs.google.com/document/d/1djncOWx3JMOk1C6ATY3TXyP_NMoeWIE_nPDH7pgWviQ/edit?usp=sharing)

---

## 🎯 Purpose & The Problems It Solves

As microservices and APIs scale, enforcing rate limits efficiently becomes a massive bottleneck. Traditional architectures typically query a central database (like Redis) on every single request. While Redis is fast, doing a full network round-trip for every API call introduces unacceptable latency and places a massive load on the Redis cluster, often causing Redis itself to become the point of failure.

**ShardRoute resolves these core issues by providing:**
1. **Zero-Latency Fast Path (Local Caching):** ShardRoute maintains an optimistic local representation of token buckets in-memory. 99% of requests are evaluated entirely in-memory without ever touching the network, reducing latency to nanoseconds.
2. **Authoritative Eventual Consistency:** A background worker asynchronously reconciles the local cache with the authoritative token states in Redis, ensuring limits are strongly enforced across all distributed nodes.
3. **Resilience (Fail-Open / Fail-Closed):** If Redis goes down, traditional rate limiters crash your entire API. ShardRoute features a robust `FailureHandler`. If the backend becomes unreachable, you can configure it to **Fail Open** (allow all traffic to keep your API alive) or **Fail Closed** (block traffic to protect underlying databases), complete with background ping recovery to automatically heal itself when Redis returns.
4. **Memory Safety (Eviction):** Idle keys are automatically evicted from the local cache via LRU/Time-based policies to prevent memory leaks during massive traffic spikes.

---

## 🏗️ Architecture

```text
[ Clients ] ---> [ Load Balancer / API Gateway ]
                        |
            +-----------+-----------+
            |                       |
    [ ShardRoute Node 1 ]   [ ShardRoute Node 2 ]
    (Local Memory Cache)    (Local Memory Cache)
            |                       |
            +-----------+-----------+
                        |
                 [ Redis Server ]
             (Authoritative Token State)
```

---

## 🚀 How to Use ShardRoute in a Real-Life Project

ShardRoute is designed to be deployed as a **standalone sidecar service** or a **centralized microservice** that sits right behind your API Gateway or Load Balancer. 

### Integration Pattern

1. **Deploy ShardRoute:** Spin up ShardRoute containers in your infrastructure (Kubernetes, AWS ECS, Fly.io) pointing to your shared Redis cluster. 
2. **API Interception:** When a user hits your main API (e.g., `api.yourcompany.com/v1/data`), your API Gateway or application middleware will first make an ultra-fast gRPC or HTTP call to the nearest ShardRoute node.
3. **Decision & Forwarding:**
   - **Allowed (`200 OK`)**: If ShardRoute allows the request, your API processes the user's request.
   - **Rejected (`429 Too Many Requests`)**: If ShardRoute rejects it, your API immediately drops the request and forwards the `429` status back to the user without touching your expensive backend logic.

### 1. Local Testing / Simulation

Spin up a full local cluster (3 ShardRoute nodes + 1 Redis instance) using Docker Compose to see how it balances traffic.

```bash
cd deploy
docker compose up -d --build
```

Send a test payload to a node (available on ports 8081, 8082, 8083):
```bash
curl -X POST http://localhost:8081/v1/check \
  -H "Content-Type: application/json" \
  -d '{"key": "user_123_ip_address", "cost": 1, "limit_name": "global_api"}'
```

### 2. Production Deployment (Kubernetes)

Build and push the highly-optimized scratch Docker container:
```bash
docker build -t your-registry/shardroute:latest -f deploy/Dockerfile .
docker push your-registry/shardroute:latest
```
Deploy it as a `Deployment` and expose it via a ClusterIP service. Ensure you pass your production Redis address via environment variables:
```yaml
env:
  - name: SHARDBROUTE_REDIS_ADDRS
    value: "redis://your-production-redis:6379"
  - name: SHARDBROUTE_FAILURE_MODE
    value: "fail_open"
```

### 3. Production Deployment (Fly.io)

A `fly.toml` is provided for immediate global edge deployment:
```bash
fly launch
fly secrets set SHARDBROUTE_REDIS_ADDRS="redis://your-production-redis"
fly deploy
```

---

## 📊 Observability & Load Testing

ShardRoute is built for massive scale and comes fully instrumented.

- **Metrics:** A `/metrics` endpoint is exposed for Prometheus scraping.
- **Grafana:** Import the pre-built `deploy/grafana/dashboard.json` into Grafana to monitor Request Rates, Latency Profiles (P50/P99), Local Cache Hit Ratios, and Node Health.
- **Benchmarking:** Use the included CLI tool to load-test your deployment:
  ```bash
  make build
  ./bin/shardroute-bench -c 100 -d 30s -url http://localhost:8081/v1/check
  ```

---

## 🛠️ Local Development

**Requirements:**
- Go 1.25+
- Redis (or Docker)
- Protobuf Compiler (`protoc`)

```bash
# Build the servers and CLI tool
make build

# Run the comprehensive race-detector test suite
make test
```
